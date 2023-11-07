package subcmd

import (
	"fmt"
	"os"
	"path/filepath"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var getCmd = &cobra.Command{
	Use:     "get [local dir]",
	Short:   "Download MD-Repo data to local dir",
	Aliases: []string{"download", "down"},
	Long:    `This downloads MD-Repo data to the given local dir.`,
	RunE:    processGetCommand,
	Args:    cobra.ExactArgs(1),
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(getCmd)

	flag.SetTokenFlags(getCmd)
	flag.SetForceFlags(getCmd, false)
	flag.SetParallelTransferFlags(getCmd)
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

	tokenFlagValues := flag.GetTokenFlagValues()
	forceFlagValues := flag.GetForceFlagValues()
	parallelTransferFlagValues := flag.GetParallelTransferFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()

	maxConnectionNum := parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	config := commons.GetConfig()

	// handle token
	if len(tokenFlagValues.TicketString) > 0 {
		config.TicketString = tokenFlagValues.TicketString
	}

	if len(tokenFlagValues.Token) > 0 {
		config.Token = tokenFlagValues.Token
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	if len(config.Token) > 0 && len(config.TicketString) == 0 {
		config.TicketString, err = commons.GetMDRepoTicketStringFromToken(tokenFlagValues.ServiceURL, config.Token)
		if err != nil {
			return xerrors.Errorf("failed to read ticket from token: %w", err)
		}
	}

	if len(config.TicketString) == 0 {
		return commons.TokenNotProvidedError
	}

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	targetPath := args[0]

	mdRepoTickets, err := commons.GetMDRepoTicketsFromString(config.TicketString)
	if err != nil {
		return xerrors.Errorf("failed to retrieve tickets: %w", err)
	}

	// we may further optimize this by run it parallel
	for _, mdRepoTicket := range mdRepoTickets {
		sourcePath := commons.MakeIRODSReleasePath(mdRepoTicket.IRODSDataPath)
		targetPath = commons.MakeLocalPath(targetPath)

		// display
		logger.Debugf("download iRODS ticket: %s", mdRepoTicket.IRODSTicket)
		logger.Debugf("download %s => %s", sourcePath, targetPath)

		// Create a file system
		account, err := commons.GetAccount(&mdRepoTicket)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS Account: %w", err)
		}

		filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
		}

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

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		targetDirPath := commons.GetDir(targetFilePath)
		_, err := os.Stat(targetDirPath)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(targetDirPath)
			}

			return xerrors.Errorf("failed to stat dir %s: %w", targetDirPath, err)
		}

		fileExist := false
		targetEntry, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			fileExist = true
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

		if fileExist {
			if !force {
				if targetEntry.Size() == sourceEntry.Size {
					if len(sourceEntry.CheckSum) > 0 {
						// compare hash
						hash, err := commons.HashLocalFile(targetFilePath, sourceEntry.CheckSumAlgorithm)
						if err != nil {
							return xerrors.Errorf("failed to get hash of %s: %w", targetFilePath, err)
						}

						if sourceEntry.CheckSum == hash {
							fmt.Printf("skip downloading file %s. The file with the same hash already exists!\n", targetFilePath)
							return nil
						}
					}
				}
			}
		}

		threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
		parallelJobManager.Schedule(sourcePath, getTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled file download %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		_, err := os.Stat(targetPath)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(targetPath)
			}

			return xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
		}

		logger.Debugf("downloading dir %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
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
