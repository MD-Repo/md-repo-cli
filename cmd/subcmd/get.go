package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var getCmd = &cobra.Command{
	Use:     "get [mdrepo_ticket|mdrepo_ticket_file_path] [local dir]",
	Short:   "Download MD-Repo data to local dir",
	Aliases: []string{"download", "down"},
	Long:    `This downloads MD-Repo data to the given local dir.`,
	RunE:    processGetCommand,
	Args:    cobra.ExactArgs(2),
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(getCmd)

	flag.SetForceFlags(getCmd, false)
	flag.SetParallelTransferFlags(getCmd, false)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processGetCommand",
	})

	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	forceFlagValues := flag.GetForceFlagValues()
	parallelTransferFlagValues := flag.GetParallelTransferFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()

	maxConnectionNum := parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	if retryFlagValues.RetryNumber > 1 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	ticketString := args[0]
	targetPath := args[1]

	mdRepoTickets := []commons.MDRepoTicket{}

	_, err = os.Stat(ticketString)
	if err != nil {
		if !os.IsNotExist(err) {
			return xerrors.Errorf("failed to read MD-Repo ticket file %s: %w", ticketString, err)
		}
		// not exist --> maybe ticket string?

		tickets, err := commons.GetConfig().GetMDRepoTickets(ticketString)
		if err != nil {
			return xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
		}

		mdRepoTickets = append(mdRepoTickets, tickets...)
	} else {
		// file exist
		ticketDataBytes, err := os.ReadFile(ticketString)
		if err != nil {
			return xerrors.Errorf("failed to read MD-Repo ticket file %s: %w", ticketString, err)
		}

		ticketLines := strings.Split(string(ticketDataBytes), "\n")
		for _, ticketLine := range ticketLines {
			ticketLine = strings.TrimSpace(ticketLine)
			if len(ticketLine) > 0 {
				tickets, err := commons.GetConfig().GetMDRepoTickets(ticketLine)
				if err != nil {
					return xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
				}

				mdRepoTickets = append(mdRepoTickets, tickets...)
			}
		}
	}

	if len(mdRepoTickets) == 0 {
		return xerrors.Errorf("failed to parse MD-Repo Ticket. No ticket is provided")
	}

	// we may further optimize this by run it parallel
	for _, mdRepoTicket := range mdRepoTickets {
		// Create a file system
		account, err := commons.GetAccount(&mdRepoTicket)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS Account: %w", err)
		}

		filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
		}

		sourcePath := commons.MakeIRODSReleasePath(mdRepoTicket.IRODSDataPath)
		targetPath = commons.MakeLocalPath(targetPath)

		// display
		logger.Debugf("download iRODS ticket: %s", mdRepoTicket.IRODSTicket)
		logger.Debugf("download path: %s", sourcePath)

		parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, !progressFlagValues.NoProgress)
		parallelJobManager.Start()

		err = getOne(parallelJobManager, sourcePath, targetPath, forceFlagValues.Force)
		if err != nil {
			filesystem.Release()
			return xerrors.Errorf("failed to perform get %s to %s: %w", sourcePath, targetPath, err)
		}

		parallelJobManager.DoneScheduling()
		err = parallelJobManager.Wait()
		if err != nil {
			filesystem.Release()
			return xerrors.Errorf("failed to perform parallel jobs: %w", err)
		}

		filesystem.Release()
	}
	return nil
}

func getOne(parallelJobManager *commons.ParallelJobManager, sourcePath string, targetPath string, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getOne",
	})

	filesystem := parallelJobManager.GetFilesystem()

	sourceEntry, err := commons.StatIRODSPath(filesystem, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)

		exist := false
		targetEntry, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			exist = true
		}

		getTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackGet := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceEntry.Size, false)

			logger.Debugf("downloading file %s to %s", sourcePath, targetFilePath)
			err := fs.DownloadFileParallel(sourcePath, "", targetFilePath, 0, callbackGet)
			if err != nil {
				job.Progress(-1, sourceEntry.Size, true)
				return xerrors.Errorf("failed to download %s to %s: %w", sourcePath, targetFilePath, err)
			}

			logger.Debugf("downloaded file %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceEntry.Size, sourceEntry.Size, false)
			return nil
		}

		if exist {
			if force {
				logger.Debugf("deleting existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else {
				if targetEntry.Size() == sourceEntry.Size {
					if len(sourceEntry.CheckSum) > 0 {
						// compare hash
						md5hash, err := commons.HashLocalFileMD5(targetFilePath)
						if err != nil {
							return xerrors.Errorf("failed to get hash of %s: %w", targetFilePath, err)
						}

						if sourceEntry.CheckSum == md5hash {
							fmt.Printf("skip downloading file %s. The file with the same hash already exists!\n", targetFilePath)
							return nil
						}
					}
				}

				logger.Debugf("deleting existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			}
		}

		threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
		parallelJobManager.Schedule(sourcePath, getTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled file download %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("downloading dir %s to %s", sourcePath, targetPath)

		entries, err := commons.ListIRODSDir(filesystem, sourceEntry.Path)
		if err != nil {
			return xerrors.Errorf("failed to list dir %s: %w", sourceEntry.Path, err)
		}

		// make target dir
		targetDir := filepath.Join(targetPath, sourceEntry.Name)
		err = os.MkdirAll(targetDir, 0766)
		if err != nil {
			return xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
		}

		for idx := range entries {
			path := entries[idx].Path

			err = getOne(parallelJobManager, path, targetDir, force)
			if err != nil {
				return xerrors.Errorf("failed to get %s to %s: %w", path, targetDir, err)
			}
		}
	}
	return nil
}
