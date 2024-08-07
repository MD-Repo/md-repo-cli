package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
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
	"golang.org/x/exp/slices"
	"golang.org/x/xerrors"
)

var submitCmd = &cobra.Command{
	Use:     "submit [data dirs] ...",
	Short:   "Submit local data to MD-Repo",
	Aliases: []string{"upload", "up", "put", "contribute"},
	RunE:    processSubmitCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddSubmitCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitCmd)

	flag.SetTokenFlags(submitCmd)
	flag.SetForceFlags(submitCmd, true)
	flag.SetParallelTransferFlags(submitCmd)
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
	if !commons.HasSubmitMetadataInDir(sourcePath) {
		// metadata path not exist?
		return xerrors.Errorf("source %s must have submit metadata", sourcePath)
	}

	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to readdir source %s: %w", sourcePath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// found dir
			return xerrors.Errorf("source %s has sub directory %s", sourcePath, entry.Name())
		}
	}

	return nil
}

// scanSourcePaths scans source paths and return valid sources only
func scanSourcePaths(sourcePaths []string, orcID string) ([]string, []string, string, error) {
	validSourcePaths := []string{}
	invalidSourcePaths := []string{}

	for _, sourcePath := range sourcePaths {
		sourcePath = commons.MakeLocalPath(sourcePath)

		st, stErr := os.Stat(sourcePath)
		if stErr != nil {
			if os.IsNotExist(stErr) {
				return nil, nil, "", irodsclient_types.NewFileNotFoundError(sourcePath)
			}

			return nil, nil, "", stErr
		}

		if !st.IsDir() {
			return nil, nil, "", xerrors.Errorf("source %s is file", sourcePath)
		}

		err := checkValidSourcePath(sourcePath)
		if err == nil {
			// valid
			validSourcePaths = append(validSourcePaths, sourcePath)
			continue
		}

		// may have sub dirs?
		dirEntries, readErr := os.ReadDir(sourcePath)
		if readErr != nil {
			return nil, nil, "", xerrors.Errorf("failed to list source %s: %w", sourcePath, readErr)
		}

		hasSubDirs := false
		for _, dirEntry := range dirEntries {
			if dirEntry.IsDir() {
				hasSubDirs = true

				entryPath := filepath.Join(sourcePath, dirEntry.Name())
				chkErr := checkValidSourcePath(entryPath)
				if chkErr == nil {
					// valid
					validSourcePaths = append(validSourcePaths, entryPath)
				} else {
					// invalid
					invalidSourcePaths = append(invalidSourcePaths, entryPath)
				}
			}
		}

		if !hasSubDirs {
			// invalid
			invalidSourcePaths = append(invalidSourcePaths, sourcePath)
		}
	}

	// sort source paths by name to match to tickets always in the same order
	slices.Sort(validSourcePaths)
	slices.Sort(invalidSourcePaths)

	// if orcID is given, override the orcID
	if len(orcID) > 0 {
		return validSourcePaths, invalidSourcePaths, orcID, nil
	}

	orcIDFound := ""
	for _, validSourcePath := range validSourcePaths {
		myOrcID, err := commons.ReadOrcIDFromSubmitMetadataFileInDir(validSourcePath)
		if err != nil {
			return nil, nil, "", xerrors.Errorf("failed to read ORC-ID from metadata for %s: %w", validSourcePath, err)
		}

		if len(myOrcID) == 0 {
			return nil, nil, "", xerrors.Errorf("failed to read ORC-ID from metadata for %s: %w", validSourcePath, commons.InvalidOrcIDError)
		}

		if len(orcIDFound) == 0 {
			orcIDFound = myOrcID
		}

		if orcIDFound != myOrcID {
			return nil, nil, "", xerrors.Errorf("Lead Contributor's ORC-ID mismatch for %s, expected %s, but got %s: %w", validSourcePath, orcIDFound, myOrcID, commons.InvalidOrcIDError)
		}
	}

	return validSourcePaths, invalidSourcePaths, orcIDFound, nil
}

func processSubmitCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
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

	sourcePaths := args[0:]
	validSourcePaths, invalidSourcePaths, orcID, err := scanSourcePaths(sourcePaths, submissionFlagValues.OrcID)
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

		if expectedSimulationNo != len(validSourcePaths) {
			logger.Debugf("we found %d simulations, but expected %d simulations", len(validSourcePaths), expectedSimulationNo)

			logger.Debugf("the simulations found:")
			for sourceIdx, sourcePath := range validSourcePaths {
				logger.Debugf("[%d] %s", sourceIdx+1, sourcePath)
			}

			logger.Debugf("the directories ignored due to lack of metadata file:")
			for sourceIdx, sourcePath := range invalidSourcePaths {
				logger.Debugf("[%d] %s", sourceIdx+1, sourcePath)
			}

			return commons.NewSimulationNoNotMatchingError(validSourcePaths, invalidSourcePaths, expectedSimulationNo)
		}

		if len(orcID) == 0 {
			orcID = commons.InputOrcID()
		}
	}

	if len(config.Token) > 0 && len(config.TicketString) == 0 {
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

		config.TicketString, err = commons.GetMDRepoTicketStringFromToken(tokenFlagValues.ServiceURL, newToken)
		if err != nil {
			return xerrors.Errorf("failed to read ticket from token: %w", err)
		}
	}

	if len(config.TicketString) == 0 {
		return commons.TokenNotProvidedError
	}

	// verify metadatas
	invalidErr := commons.VerifySubmitMetadata(validSourcePaths, tokenFlagValues.ServiceURL, config.Token)
	if invalidErr != nil {
		return invalidErr
	} else {
		logger.Debugf("all submit metadata are valid")
	}

	if retryFlagValues.RetryNumber > 0 && !retryFlagValues.RetryChild {
		err = commons.RunWithRetry(retryFlagValues.RetryNumber, retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	mdRepoTickets, err := commons.GetMDRepoTicketsFromString(config.TicketString)
	if err != nil {
		return xerrors.Errorf("failed to retrieve tickets: %w", err)
	}

	if len(mdRepoTickets) != len(validSourcePaths) {
		logger.Debugf("we found %d simulations, but we got %d tokens", len(mdRepoTickets), len(validSourcePaths))
	}

	for ticketIdx, mdRepoTicket := range mdRepoTickets {
		sourcePath := validSourcePaths[ticketIdx]
		targetPath := commons.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

		// display
		logger.Debugf("submission iRODS ticket: %s", mdRepoTicket.IRODSTicket)
		logger.Debugf("submission %s => %s", sourcePath, targetPath)

		// Create a file system
		account, err := commons.GetAccount(&mdRepoTicket)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS Account: %w", err)
		}

		filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, parallelTransferFlagValues.TCPBufferSize)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
		}

		submitStatusFile := commons.NewSubmitStatusFile()
		submitStatusFile.Token = config.Token

		parallelJobManager := commons.NewParallelJobManager(filesystem, parallelTransferFlagValues.ThreadNumber, !progressFlagValues.NoProgress)
		parallelJobManager.Start()

		err = submitOne(parallelJobManager, submitStatusFile, sourcePath, targetPath, forceFlagValues, parallelTransferFlagValues)
		if err != nil {
			submitStatusFile.SetErrored()
			submitStatusFile.CreateStatusFile(filesystem, targetPath)
			filesystem.Release()

			return xerrors.Errorf("failed to submit %s to %s: %w", sourcePath, targetPath, err)
		}

		parallelJobManager.DoneScheduling()

		// status file
		submitStatusFile.SetInProgress()
		err = submitStatusFile.CreateStatusFile(filesystem, targetPath)
		if err != nil {
			filesystem.Release()

			return xerrors.Errorf("failed to create status file on %s: %w", targetPath, err)
		}

		err = parallelJobManager.Wait()
		if err != nil {
			submitStatusFile.SetErrored()
			submitStatusFile.CreateStatusFile(filesystem, targetPath)
			filesystem.Release()

			return xerrors.Errorf("failed to perform parallel jobs: %w", err)
		}

		// status file
		submitStatusFile.SetCompleted()
		err = submitStatusFile.CreateStatusFile(filesystem, targetPath)
		if err != nil {
			filesystem.Release()

			return xerrors.Errorf("failed to create status file on %s: %w", targetPath, err)
		}

		filesystem.Release()
	}

	return nil
}

func submitOne(parallelJobManager *commons.ParallelJobManager, submitStatusFile *commons.SubmitStatusFile, sourcePath string, targetPath string, forceFlagValues *flag.ForceFlagValues, parallelTransferFlagValues *flag.ParallelTransferFlagValues) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
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

		fileExist := false
		targetEntry, err := filesystem.StatFile(targetFilePath)
		if err != nil {
			if !irodsclient_types.IsFileNotFoundError(err) {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}
		} else {
			fileExist = true
		}

		putTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackPut := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceStat.Size(), false)

			logger.Debugf("uploading file %s to %s", sourcePath, targetFilePath)

			var uploadErr error
			if parallelTransferFlagValues.ThreadNumber == 1 {
				_, uploadErr = fs.UploadFile(sourcePath, targetFilePath, "", false, true, true, callbackPut)
			} else if parallelTransferFlagValues.RedirectToResource {
				_, uploadErr = fs.UploadFileParallelRedirectToResource(sourcePath, targetFilePath, "", 0, false, true, true, callbackPut)
			} else if parallelTransferFlagValues.Icat {
				_, uploadErr = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, true, true, callbackPut)
			} else {
				// auto
				if sourceStat.Size() >= commons.RedirectToResourceMinSize {
					// redirect-to-resource
					_, uploadErr = fs.UploadFileParallelRedirectToResource(sourcePath, targetFilePath, "", 0, false, true, true, callbackPut)
				} else {
					if filesystem.SupportParallelUpload() {
						_, uploadErr = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, true, true, callbackPut)
					} else {
						if sourceStat.Size() >= commons.ParallelUploadMinSize {
							// does not support parall upload via iCAT
							// redirect-to-resource
							_, uploadErr = fs.UploadFileParallelRedirectToResource(sourcePath, targetFilePath, "", 0, false, true, true, callbackPut)
						} else {
							_, uploadErr = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, false, true, true, callbackPut)
						}
					}
				}
			}

			if uploadErr != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to upload %s to %s: %w", sourcePath, targetFilePath, uploadErr)
			}

			logger.Debugf("uploaded file %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceStat.Size(), sourceStat.Size(), false)

			job.Done()
			return nil
		}

		hash, err := irodsclient_util.HashLocalFile(sourcePath, "md5")
		if err != nil {
			return xerrors.Errorf("failed to get hash for %s: %w", sourcePath, err)
		}

		targetFileRelPath := targetFilePath
		if strings.HasPrefix(targetFilePath, fmt.Sprintf("%s/", targetPath)) {
			targetFileRelPath = targetFilePath[len(targetPath)+1:]
		}

		submitStatusEntry := commons.SubmitStatusEntry{
			IRODSPath: targetFileRelPath, // store relative path
			Size:      sourceStat.Size(),
			MD5Hash:   hex.EncodeToString(hash),
		}
		submitStatusFile.AddFile(submitStatusEntry)

		if fileExist {
			if !forceFlagValues.Force {
				if targetEntry.Size == sourceStat.Size() {
					if len(targetEntry.CheckSum) > 0 {
						// compare hash
						if bytes.Equal(hash, targetEntry.CheckSum) {
							fmt.Printf("skip uploading file %s. The file with the same hash already exists!\n", targetFilePath)
							return nil
						}
					}
				}
			}
		}

		threadsRequired := computeThreadsRequiredForSubmit(filesystem, sourceStat.Size())
		parallelJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled local file upload %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		_, err := filesystem.Stat(targetPath)
		if err != nil {
			return xerrors.Errorf("failed to stat dir %s: %w", targetPath, err)
		}

		logger.Debugf("uploading local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to read dir %s: %w", sourcePath, err)
		}

		// make target dir
		for _, entryInDir := range entries {
			newSourcePath := filepath.Join(sourcePath, entryInDir.Name())
			err = submitOne(parallelJobManager, submitStatusFile, newSourcePath, targetPath, forceFlagValues, parallelTransferFlagValues)
			if err != nil {
				return xerrors.Errorf("failed to perform put %s to %s: %w", newSourcePath, targetPath, err)
			}
		}
	}
	return nil
}

func computeThreadsRequiredForSubmit(fs *fs.FileSystem, size int64) int {
	if fs.SupportParallelUpload() {
		return irodsclient_util.GetNumTasksForParallelTransfer(size)
	}

	return 1
}
