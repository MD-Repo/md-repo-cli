package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"

	"github.com/MD-Repo/md-repo-cli/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var getCmd = &cobra.Command{
	Use:   "get [mdrepo_ticket] [local dir]",
	Short: "Download MD-Repo data to local dir",
	Long:  `This downloads MD-Repo data to the given local dir.`,
	RunE:  processGetCommand,
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(getCmd)

	getCmd.Flags().BoolP("force", "f", false, "Get forcefully")
	getCmd.Flags().Int("download_thread_num", commons.MaxParallelJobThreadNumDefault, "Specify the number of download threads")
	getCmd.Flags().String("tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	getCmd.Flags().Bool("no_progress", false, "Do not display progress bar")

	getCmd.Flags().Int("retry", 1, "Retry if fails")
	getCmd.Flags().Int("retry_interval", 60, "Retry interval in seconds")

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processGetCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
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

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
		}
	}

	downloadThreadNum := commons.MaxParallelJobThreadNumDefault
	downloadThreadNumFlag := command.Flags().Lookup("download_thread_num")
	if downloadThreadNumFlag != nil {
		n, err := strconv.ParseInt(downloadThreadNumFlag.Value.String(), 10, 32)
		if err == nil {
			downloadThreadNum = int(n)
		}
	}

	maxConnectionNum := downloadThreadNum + 2 // 2 for metadata op

	tcpBufferSize := commons.TcpBufferSizeDefault
	tcpBufferSizeFlag := command.Flags().Lookup("tcp_buffer_size")
	if tcpBufferSizeFlag != nil {
		n, err := commons.ParseSize(tcpBufferSizeFlag.Value.String())
		if err == nil {
			tcpBufferSize = int(n)
		}
	}

	progress := true
	noProgressFlag := command.Flags().Lookup("no_progress")
	if noProgressFlag != nil {
		noProgress, err := strconv.ParseBool(noProgressFlag.Value.String())
		if err != nil {
			progress = true
		} else {
			progress = !noProgress
		}
	}

	retryChild := false
	retryChildFlag := command.Flags().Lookup("retry_child")
	if retryChildFlag != nil {
		retryChildValue, err := strconv.ParseBool(retryChildFlag.Value.String())
		if err != nil {
			retryChildValue = false
		}

		retryChild = retryChildValue
	}

	retry := int64(1)
	retryFlag := command.Flags().Lookup("retry")
	if retryFlag != nil {
		retry, err = strconv.ParseInt(retryFlag.Value.String(), 10, 32)
		if err != nil {
			retry = 1
		}
	}

	retryInterval := int64(60)
	retryIntervalFlag := command.Flags().Lookup("retry_interval")
	if retryIntervalFlag != nil {
		retryInterval, err = strconv.ParseInt(retryIntervalFlag.Value.String(), 10, 32)
		if err != nil {
			retryInterval = 60
		}
	}

	if retry > 1 && !retryChild {
		err = commons.RunWithRetry(int(retry), int(retryInterval))
		if err != nil {
			return err
		}
		return nil
	}

	if len(args) < 2 {
		return xerrors.Errorf("not enough input arguments")
	}

	ticket := args[0]
	targetPath := args[1]

	mdRepoTicket, err := commons.GetConfig().GetMDRepoTicket(ticket)
	if err != nil {
		return xerrors.Errorf("failed to get MD-Repo Ticket: %w", err)
	}

	// Create a file system
	account, err := commons.GetAccount(mdRepoTicket)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS Account: %w", err)
	}

	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, tcpBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	// display
	logger.Debugf("download iRODS ticket: %s", mdRepoTicket.IRODSTicket)
	logger.Debugf("download path: %s", mdRepoTicket.IRODSDataPath)

	parallelJobManager := commons.NewParallelJobManager(filesystem, downloadThreadNum, progress)
	parallelJobManager.Start()

	err = getOne(parallelJobManager, mdRepoTicket.IRODSDataPath, targetPath, force)
	if err != nil {
		return xerrors.Errorf("failed to perform get %s to %s: %w", mdRepoTicket.IRODSDataPath, targetPath, err)
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	return nil
}

func getOne(parallelJobManager *commons.ParallelJobManager, sourcePath string, targetPath string, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getOne",
	})

	sourcePath = commons.MakeIRODSReleasePath(sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

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
