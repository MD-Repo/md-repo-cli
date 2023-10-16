package subcmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var submitCmd = &cobra.Command{
	Use:     "submit [data dirs] ...",
	Short:   "Submit local data to MD-Repo",
	Aliases: []string{"upload", "up", "put"},
	RunE:    processSubmitCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddSubmitCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitCmd)

	flag.SetTokenFlags(submitCmd)
	flag.SetForceFlags(submitCmd, true)
	flag.SetParallelTransferFlags(submitCmd, true)
	flag.SetProgressFlags(submitCmd)
	flag.SetRetryFlags(submitCmd)

	rootCmd.AddCommand(submitCmd)
}

func checkValidSourcePath(sourcePath string) error {
	st, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat source %s: %w", sourcePath, err)
	}

	if !st.IsDir() {
		return xerrors.Errorf("source %s must be a directory", sourcePath)
	}

	// check if source path has metadata in it
	if commons.HasSubmitMetadataInDir(sourcePath) {
		// found
		return nil
	}

	// metadata path not exist?
	return xerrors.Errorf("invalid metadata dir %s", sourcePath)
}

// scanSourcePaths scans source paths and return valid sources only
func scanSourcePaths(sourcePaths []string) ([]string, string, error) {
	foundSourcePaths := []string{}

	for _, sourcePath := range sourcePaths {
		sourcePath = commons.MakeLocalPath(sourcePath)

		err := checkValidSourcePath(sourcePath)
		if err == nil {
			// valid
			foundSourcePaths = append(foundSourcePaths, sourcePath)
			continue
		}

		// may have sub dirs?
		st, stErr := os.Stat(sourcePath)
		if stErr != nil {
			if os.IsNotExist(err) {
				return nil, "", irodsclient_types.NewFileNotFoundError(sourcePath)
			}

			return nil, "", err
		}

		if !st.IsDir() {
			return nil, "", err
		}

		dirEntries, readErr := os.ReadDir(sourcePath)
		if readErr != nil {
			return nil, "", xerrors.Errorf("failed to list source %s: %w", sourcePath, readErr)
		}

		for _, dirEntry := range dirEntries {
			entryPath := filepath.Join(sourcePath, dirEntry.Name())
			chkErr := checkValidSourcePath(entryPath)
			if chkErr == nil {
				// valid
				foundSourcePaths = append(foundSourcePaths, entryPath)
			}
		}
	}

	orcIDFound := ""
	for _, foundSourcePath := range foundSourcePaths {
		orcID, err := commons.ReadOrcIDFromSubmitMetadataFileInDir(foundSourcePath)
		if err != nil {
			return nil, "", xerrors.Errorf("failed to read ORC-ID from metadata for %s: %w", foundSourcePath, err)
		}

		if len(orcID) == 0 {
			return nil, "", xerrors.Errorf("failed to read ORC-ID from metadata for %s: %w", foundSourcePath, commons.InvalidOrcIDError)
		}

		if len(orcIDFound) == 0 {
			orcIDFound = orcID
		}

		if orcIDFound != orcID {
			return nil, "", xerrors.Errorf("Lead Contributor's ORC-ID mismatch for %s, expected %s, but got %s: %w", foundSourcePath, orcIDFound, orcID, commons.InvalidOrcIDError)
		}
	}

	return foundSourcePaths, orcIDFound, nil
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

	tokenFlagValues := flag.GetTokenFlagValues()
	forceFlagValues := flag.GetForceFlagValues()
	parallelTransferFlagValues := flag.GetParallelTransferFlagValues()
	progressFlagValues := flag.GetProgressFlagValues()
	retryFlagValues := flag.GetRetryFlagValues()
	submissionFlagValues := flag.GetSubmissionFlagValues()
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

	sourcePaths := args[0:]
	sourcePaths, orcID, err := scanSourcePaths(sourcePaths)
	if err != nil {
		return xerrors.Errorf("failed to scan source paths: %w", err)
	}

	if !retryFlagValues.RetryChild {
		// only parent has input
		expectedSimulationNo := 0
		if submissionFlagValues.ExpectedSimulations > 0 {
			expectedSimulationNo = submissionFlagValues.ExpectedSimulations
		} else {
			expectedSimulationNo = commons.InputSimulationNo()
		}

		numSimulations := len(sourcePaths)
		if expectedSimulationNo != numSimulations {
			logger.Debugf("we found %d simulations, but expected %d simulations", numSimulations, expectedSimulationNo)

			logger.Debugf("the simulations we found are:")
			for sourceIdx, sourcePath := range sourcePaths {
				logger.Debugf("[%d] %s\n", sourceIdx+1, sourcePath)
			}

			return xerrors.Errorf("The number of simulations typed (%d) does not match the number of simulations we found (%d): %w", expectedSimulationNo, numSimulations, commons.SimulationNoNotMatchingError)
		}
	}

	if len(config.Token) > 0 && len(config.TicketString) == 0 {
		// orcID
		// override ORC-ID
		if len(submissionFlagValues.OrcID) > 0 {
			orcID = submissionFlagValues.OrcID
		}

		// encrypt
		tokenBytes, err := commons.Base64Decode(config.Token)
		if err != nil {
			return xerrors.Errorf("failed to decode token using BASE64: %w", err)
		}

		newToken, err := commons.HMACStringSHA224(tokenBytes, orcID)
		if err != nil {
			return xerrors.Errorf("failed to encrypt token using SHA3-224 HMAC: %w", err)
		}

		logger.Debugf("encrypted token: %s", newToken)

		config.TicketString, err = commons.GetMDRepoTicketStringFromToken(newToken)
		if err != nil {
			return xerrors.Errorf("failed to read ticket from token: %w", err)
		}
	}

	if len(config.TicketString) == 0 {
		return commons.TokenNotProvidedError
	}

	maxConnectionNum := parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	mdRepoTicket, err := commons.GetMDRepoTicketFromString(config.TicketString)
	if err != nil {
		return xerrors.Errorf("failed to retrieve ticket: %w", err)
	}

	// Create a file system
	account, err := commons.GetAccount(&mdRepoTicket)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS Account: %w", err)
	}

	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := commons.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

	// display
	logger.Debugf("submission iRODS ticket: %s", mdRepoTicket.IRODSTicket)
	logger.Debugf("submission path: %s", targetPath)

	submitStatusFile := commons.NewSubmitStatusFile()

	parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, !progressFlagValues.NoProgress)

	includeFirstDir := false
	if len(sourcePaths) > 1 {
		includeFirstDir = true
	}

	for _, sourcePath := range sourcePaths {
		logger.Debugf("submitting %s", sourcePath)

		err = submitOne(parallelJobManager, submitStatusFile, sourcePath, targetPath, targetPath, forceFlagValues.Force, parallelTransferFlagValues.SingleTread, includeFirstDir)
		if err != nil {
			return xerrors.Errorf("failed to submit %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()

	// status file
	submitStatusFile.SetInProgress()
	err = submitStatusFile.CreateStatusFile(filesystem, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create status file on %s: %w", targetPath, err)
	}

	parallelJobManager.Start()
	err = parallelJobManager.Wait()
	if err != nil {
		submitStatusFile.SetErrored()
		defer submitStatusFile.CreateStatusFile(filesystem, targetPath)
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	// status file
	submitStatusFile.SetCompleted()
	err = submitStatusFile.CreateStatusFile(filesystem, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to create status file on %s: %w", targetPath, err)
	}

	return nil
}

func submitOne(parallelJobManager *commons.ParallelJobManager, submitStatusFile *commons.SubmitStatusFile, sourcePath string, targetRootPath string, targetPath string, force bool, singleThreaded bool, includeFirstDir bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "submitOne",
	})

	filesystem := parallelJobManager.GetFilesystem()

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if !sourceStat.IsDir() {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		targetDirPath := commons.GetDir(targetFilePath)
		_, err := commons.StatIRODSPath(filesystem, targetDirPath)
		if err != nil {
			return xerrors.Errorf("failed to stat dir %s: %w", targetDirPath, err)
		}

		fileExist := commons.ExistsIRODSFile(filesystem, targetFilePath)

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

		hash, err := commons.HashLocalFile(sourcePath, "md5")
		if err != nil {
			return xerrors.Errorf("failed to get hash for %s: %w", sourcePath, err)
		}

		targetFileRelPath := targetFilePath
		if strings.HasPrefix(targetFilePath, fmt.Sprintf("%s/", targetRootPath)) {
			targetFileRelPath = targetFilePath[len(targetRootPath)+1:]
		}

		submitStatusEntry := commons.SubmitStatusEntry{
			IRODSPath: targetFileRelPath, // store relative path
			Size:      sourceStat.Size(),
			MD5Hash:   hash,
		}
		submitStatusFile.AddFile(submitStatusEntry)

		if fileExist {
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
						if hash == targetEntry.CheckSum {
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
		_, err := commons.StatIRODSPath(filesystem, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
		}

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
			err = submitOne(parallelJobManager, submitStatusFile, newSourcePath, targetRootPath, targetDir, force, singleThreaded, true)
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
