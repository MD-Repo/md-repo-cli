package subcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/cyverse/go-irodsclient/fs"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var submitCmd = &cobra.Command{
	Use:     "submit [mdrepo_ticket] [local data dir or file] ...",
	Short:   "Submit local data to MD-Repo",
	Aliases: []string{"upload", "up"},
	RunE:    processSubmitCommand,
}

func AddPutCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(submitCmd)

	submitCmd.Flags().BoolP("force", "f", false, "Submit forcefully")
	submitCmd.Flags().MarkHidden("force")

	submitCmd.Flags().Bool("single_threaded", false, "Transfer a file using a single thread")
	submitCmd.Flags().Int("upload_thread_num", commons.MaxParallelJobThreadNumDefault, "Specify the number of upload threads")
	submitCmd.Flags().String("tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	submitCmd.Flags().Bool("no_progress", false, "Do not display progress bar")

	submitCmd.Flags().Int("retry", 1, "Retry if fails")
	submitCmd.Flags().Int("retry_interval", 60, "Retry interval in seconds")

	rootCmd.AddCommand(submitCmd)
}

func processSubmitCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processSubmitCommand",
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

	singleThreaded := false
	singleThreadedFlag := command.Flags().Lookup("single_threaded")
	if singleThreadedFlag != nil {
		singleThreaded, err = strconv.ParseBool(singleThreadedFlag.Value.String())
		if err != nil {
			singleThreaded = false
		}
	}

	uploadThreadNum := commons.MaxParallelJobThreadNumDefault
	uploadThreadNumFlag := command.Flags().Lookup("upload_thread_num")
	if uploadThreadNumFlag != nil {
		n, err := strconv.ParseInt(uploadThreadNumFlag.Value.String(), 10, 32)
		if err == nil {
			uploadThreadNum = int(n)
		}
	}

	maxConnectionNum := uploadThreadNum + 2 // 2 for metadata op

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
			return xerrors.Errorf("failed to run with retry %d: %w", retry, err)
		}
		return nil
	}

	if len(args) < 2 {
		return xerrors.Errorf("not enough input arguments")
	}

	ticket := strings.TrimSpace(args[0])
	sourcePaths := args[1:]

	log.Debugf("ticket string: %s", ticket)

	mdRepoTicket, err := commons.GetConfig().GetMDRepoTicket(ticket)
	if err != nil {
		return xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
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
	logger.Debugf("submission iRODS ticket: %s", mdRepoTicket.IRODSTicket)
	logger.Debugf("submission path: %s", mdRepoTicket.IRODSDataPath)

	existingEntries, err := filesystem.List(mdRepoTicket.IRODSDataPath)
	if err != nil {
		return xerrors.Errorf("failed to list %s: %w", mdRepoTicket.IRODSDataPath, err)
	}

	logger.Debugf("found %d files in path %s", len(existingEntries), mdRepoTicket.IRODSDataPath)

	submitStatusFile := commons.NewSubmitStatusFile()

	parallelJobManager := commons.NewParallelJobManager(filesystem, uploadThreadNum, progress)

	for _, sourcePath := range sourcePaths {
		includeFirstDir := false
		if len(sourcePaths) > 1 {
			includeFirstDir = true
		}
		err = submitOne(parallelJobManager, submitStatusFile, sourcePath, mdRepoTicket.IRODSDataPath, force, singleThreaded, includeFirstDir)
		if err != nil {
			return xerrors.Errorf("failed to submit %s to %s: %w", sourcePath, mdRepoTicket.IRODSDataPath, err)
		}
	}

	parallelJobManager.DoneScheduling()

	// status file
	submitStatusFile.SetInProgress()
	err = createSubmitStatusFile(filesystem, submitStatusFile, mdRepoTicket.IRODSDataPath)
	if err != nil {
		return xerrors.Errorf("failed to create status file on %s: %w", mdRepoTicket.IRODSDataPath, err)
	}

	parallelJobManager.Start()
	err = parallelJobManager.Wait()
	if err != nil {
		submitStatusFile.SetErrored()
		defer createSubmitStatusFile(filesystem, submitStatusFile, mdRepoTicket.IRODSDataPath)
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	// status file
	submitStatusFile.SetCompleted()
	err = createSubmitStatusFile(filesystem, submitStatusFile, mdRepoTicket.IRODSDataPath)
	if err != nil {
		return xerrors.Errorf("failed to create status file on %s: %w", mdRepoTicket.IRODSDataPath, err)
	}

	return nil
}

func createSubmitStatusFile(filesystem *fs.FileSystem, submitStatusFile *commons.SubmitStatusFile, dataRootPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "createStatusFile",
	})

	dataRootPath = commons.MakeIRODSLandingPath(dataRootPath)
	statusFileName := commons.GetMDRepoStatusFilename()
	statusFilePath := commons.MakeTargetIRODSFilePath(filesystem, statusFileName, dataRootPath)

	localTempFile := filepath.Join(os.TempDir(), statusFileName)

	logger.Debugf("creating local status file to %s", localTempFile)
	jsonBytes, err := json.Marshal(submitStatusFile)
	if err != nil {
		return xerrors.Errorf("failed to marshal submit status file to json: %w", err)
	}

	err = os.WriteFile(localTempFile, jsonBytes, 0666)
	if err != nil {
		return xerrors.Errorf("failed to write submit status file to local: %w", err)
	}

	logger.Debugf("creating status file to %s", statusFilePath)

	err = filesystem.UploadFile(localTempFile, statusFilePath, "", false, nil)
	if err != nil {
		return xerrors.Errorf("failed to create submit status file to %s: %w", statusFilePath, err)
	}

	os.Remove(localTempFile)
	return nil
}

func submitOne(parallelJobManager *commons.ParallelJobManager, submitStatusFile *commons.SubmitStatusFile, sourcePath string, targetPath string, force bool, singleThreaded bool, includeFirstDir bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "submitOne",
	})

	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSLandingPath(targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if !sourceStat.IsDir() {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		exist := commons.ExistsIRODSFile(filesystem, targetFilePath)

		putTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackPut := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceStat.Size(), false)

			logger.Debugf("uploading file %s to %s", sourcePath, targetFilePath)
			if singleThreaded {
				err = fs.UploadFile(sourcePath, targetFilePath, "", false, callbackPut)
			} else {
				err = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, callbackPut)
			}

			if err != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to upload %s to %s: %w", sourcePath, targetFilePath, err)
			}

			logger.Debugf("uploaded file %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceStat.Size(), sourceStat.Size(), false)
			return nil
		}

		md5hash, err := commons.HashLocalFileMD5(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to get hash for %s: %w", sourcePath, err)
		}

		submitStatusEntry := commons.SubmitStatusEntry{
			IRODSPath: targetFilePath,
			Size:      sourceStat.Size(),
			MD5Hash:   md5hash,
		}
		submitStatusFile.AddFile(submitStatusEntry)

		if exist {
			targetEntry, err := commons.StatIRODSPath(filesystem, targetFilePath)
			if err != nil {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}

			if force {
				logger.Debugf("deleting existing file %s", targetFilePath)
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else {
				if targetEntry.Size == sourceStat.Size() {
					if len(targetEntry.CheckSum) > 0 {
						// compare hash
						if md5hash == targetEntry.CheckSum {
							fmt.Printf("skip uploading file %s. The file with the same hash already exists!\n", targetFilePath)
							return nil
						}
					}
				}

				logger.Debugf("deleting existing file %s", targetFilePath)
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			}
		}

		threadsRequired := computeThreadsRequiredForSubmit(filesystem, singleThreaded, sourceStat.Size())
		parallelJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled local file upload %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("uploading local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to read dir %s: %w", sourcePath, err)
		}

		// make target dir
		targetDir := targetPath
		if includeFirstDir {
			targetDir = path.Join(targetPath, filepath.Base(sourcePath))
			err = filesystem.MakeDir(targetDir, true)
			if err != nil {
				return xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
			}
		}

		for _, entryInDir := range entries {
			newSourcePath := filepath.Join(sourcePath, entryInDir.Name())
			err = submitOne(parallelJobManager, submitStatusFile, newSourcePath, targetDir, force, singleThreaded, true)
			if err != nil {
				return xerrors.Errorf("failed to perform put %s to %s: %w", newSourcePath, targetDir, err)
			}
		}
	}

	return nil
}

func computeThreadsRequiredForSubmit(fs *fs.FileSystem, singleThreaded bool, size int64) int {
	if singleThreaded {
		return 1
	}

	if fs.SupportParallelUpload() {
		return irodsclient_util.GetNumTasksForParallelTransfer(size)
	}

	return 1
}
