package subcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
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
	Aliases: []string{"upload", "up", "put"},
	RunE:    processSubmitCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddPutCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitCmd)

	flag.SetForceFlags(submitCmd, true)
	flag.SetParallelTransferFlags(submitCmd, true)
	flag.SetProgressFlags(submitCmd)
	flag.SetRetryFlags(submitCmd)

	rootCmd.AddCommand(submitCmd)
}

func processSubmitCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processSubmitCommand",
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

	ticketString := strings.TrimSpace(args[0])
	sourcePaths := args[1:]

	mdRepoTickets, err := commons.GetConfig().GetMDRepoTickets(ticketString)
	if err != nil {
		return xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
	}

	if len(mdRepoTickets) == 0 {
		return xerrors.Errorf("failed to parse MD-Repo Ticket. No ticket is provided")
	}

	// Create a file system
	account, err := commons.GetAccount(&mdRepoTickets[0])
	if err != nil {
		return xerrors.Errorf("failed to get iRODS Account: %w", err)
	}

	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := commons.MakeIRODSLandingPath(mdRepoTickets[0].IRODSDataPath)

	// display
	logger.Debugf("submission iRODS ticket: %s", mdRepoTickets[0].IRODSTicket)
	logger.Debugf("submission path: %s", targetPath)

	submitStatusFile := commons.NewSubmitStatusFile()

	parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, !progressFlagValues.NoProgress)

	for _, sourcePath := range sourcePaths {
		includeFirstDir := false
		if len(sourcePaths) > 1 {
			includeFirstDir = true
		}

		sourcePath = commons.MakeLocalPath(sourcePath)

		err = submitOne(parallelJobManager, submitStatusFile, sourcePath, targetPath, forceFlagValues.Force, parallelTransferFlagValues.SingleTread, includeFirstDir)
		if err != nil {
			return xerrors.Errorf("failed to submit %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()

	// status file
	submitStatusFile.SetInProgress()
	err = createSubmitStatusFile(filesystem, submitStatusFile, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create status file on %s: %w", targetPath, err)
	}

	parallelJobManager.Start()
	err = parallelJobManager.Wait()
	if err != nil {
		submitStatusFile.SetErrored()
		defer createSubmitStatusFile(filesystem, submitStatusFile, targetPath)
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	// status file
	submitStatusFile.SetCompleted()
	err = createSubmitStatusFile(filesystem, submitStatusFile, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create status file on %s: %w", targetPath, err)
	}

	return nil
}

func createSubmitStatusFile(filesystem *fs.FileSystem, submitStatusFile *commons.SubmitStatusFile, dataRootPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "createStatusFile",
	})

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

	if filesystem.ExistsFile(statusFilePath) {
		err = filesystem.RemoveFile(statusFilePath, true)
		if err != nil {
			return xerrors.Errorf("failed to delete stale submit status file %s: %w", statusFilePath, err)
		}
	}

	err = filesystem.UploadFile(localTempFile, statusFilePath, "", false, nil)
	if err != nil {
		return xerrors.Errorf("failed to create submit status file %s: %w", statusFilePath, err)
	}

	os.Remove(localTempFile)
	return nil
}

func submitOne(parallelJobManager *commons.ParallelJobManager, submitStatusFile *commons.SubmitStatusFile, sourcePath string, targetPath string, force bool, singleThreaded bool, includeFirstDir bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "submitOne",
	})

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
