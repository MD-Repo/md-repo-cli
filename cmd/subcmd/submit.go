package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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

	flag.SetSubmissionFlags(submitCmd)
	flag.SetTokenFlags(submitCmd)
	flag.SetForceFlags(submitCmd, true)
	flag.SetParallelTransferFlags(submitCmd, false, false, true)
	flag.SetProgressFlags(submitCmd)
	flag.SetRetryFlags(submitCmd)
	flag.SetTransferReportFlags(submitCmd)

	rootCmd.AddCommand(submitCmd)
}

func processSubmitCommand(command *cobra.Command, args []string) error {
	submit, err := NewSubmitCommand(command, args)
	if err != nil {
		return err
	}

	return submit.Process()
}

type SubmitCommand struct {
	command *cobra.Command

	commonFlagValues           *flag.CommonFlagValues
	submissionFlagValues       *flag.SubmissionFlagValues
	tokenFlagValues            *flag.TokenFlagValues
	forceFlagValues            *flag.ForceFlagValues
	parallelTransferFlagValues *flag.ParallelTransferFlagValues
	progressFlagValues         *flag.ProgressFlagValues
	retryFlagValues            *flag.RetryFlagValues
	transferReportFlagValues   *flag.TransferReportFlagValues

	maxConnectionNum int

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string

	parallelJobManager    *commons.ParallelJobManager
	transferReportManager *commons.TransferReportManager
	submitStatusFile      *commons.SubmitStatusFile
	updatedPathMap        map[string]bool
}

func NewSubmitCommand(command *cobra.Command, args []string) (*SubmitCommand, error) {
	submit := &SubmitCommand{
		command: command,

		commonFlagValues:           flag.GetCommonFlagValues(command),
		submissionFlagValues:       flag.GetSubmissionFlagValues(),
		tokenFlagValues:            flag.GetTokenFlagValues(),
		forceFlagValues:            flag.GetForceFlagValues(),
		parallelTransferFlagValues: flag.GetParallelTransferFlagValues(),
		progressFlagValues:         flag.GetProgressFlagValues(),
		retryFlagValues:            flag.GetRetryFlagValues(),
		transferReportFlagValues:   flag.GetTransferReportFlagValues(command),

		updatedPathMap: map[string]bool{},
	}

	submit.maxConnectionNum = submit.parallelTransferFlagValues.ThreadNumber

	// path
	submit.sourcePaths = args

	return submit, nil
}

func (submit *SubmitCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SubmitCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(submit.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	config := commons.GetConfig()

	// handle token
	if len(submit.tokenFlagValues.TicketString) > 0 {
		config.TicketString = submit.tokenFlagValues.TicketString
	}

	if len(submit.tokenFlagValues.Token) > 0 {
		config.Token = submit.tokenFlagValues.Token
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	validSourcePaths, invalidSourcePaths, orcID, err := submit.scanSourcePaths(submit.submissionFlagValues.OrcID)
	if err != nil {
		return xerrors.Errorf("failed to scan source paths: %w", err)
	}

	if !submit.retryFlagValues.RetryChild {
		// only parent has input
		expectedSimulationNo := 0
		if submit.submissionFlagValues.ExpectedSimulations > 0 {
			expectedSimulationNo = submit.submissionFlagValues.ExpectedSimulations
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

		config.TicketString, err = commons.GetMDRepoTicketStringFromToken(submit.tokenFlagValues.ServiceURL, newToken)
		if err != nil {
			return xerrors.Errorf("failed to read ticket from token: %w", err)
		}
	}

	if len(config.TicketString) == 0 {
		return commons.TokenNotProvidedError
	}

	// verify metadatas
	invalidErr := commons.VerifySubmitMetadata(validSourcePaths, submit.tokenFlagValues.ServiceURL, config.Token)
	if invalidErr != nil {
		return invalidErr
	} else {
		logger.Debugf("all submit metadata are valid")
	}

	// handle retry
	if submit.retryFlagValues.RetryNumber > 0 && !submit.retryFlagValues.RetryChild {
		err = commons.RunWithRetry(submit.retryFlagValues.RetryNumber, submit.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", submit.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// transfer report
	submit.transferReportManager, err = commons.NewTransferReportManager(submit.transferReportFlagValues.Report, submit.transferReportFlagValues.ReportPath, submit.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer submit.transferReportManager.Release()

	// get ticket
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

		// we create filesystem, job manager for every ticket as they require separate auth
		// Create a file system
		submit.account, err = commons.GetAccount(&mdRepoTicket)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS Account: %w", err)
		}

		submit.filesystem, err = commons.GetIRODSFSClientForLargeFileIO(submit.account, submit.maxConnectionNum, submit.parallelTransferFlagValues.TCPBufferSize)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
		}
		defer submit.filesystem.Release()

		// parallel job manager
		submit.parallelJobManager = commons.NewParallelJobManager(submit.filesystem, submit.parallelTransferFlagValues.ThreadNumber, !submit.progressFlagValues.NoProgress, submit.progressFlagValues.ShowFullPath)
		submit.parallelJobManager.Start()

		// submit status file
		submit.submitStatusFile = commons.NewSubmitStatusFile()
		submit.submitStatusFile.Token = config.Token

		// run
		err = submit.submitOne(mdRepoTicket, sourcePath, targetPath)
		if err != nil {
			submit.submitStatusFile.SetErrored()
			submit.submitStatusFile.CreateStatusFile(submit.filesystem, targetPath)
			return xerrors.Errorf("failed to submit %q to %q: %w", sourcePath, targetPath, err)
		}

		// create a status file
		submit.submitStatusFile.SetInProgress()
		err = submit.submitStatusFile.CreateStatusFile(submit.filesystem, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to create status file on %q: %w", targetPath, err)
		}

		// release parallel job manager
		submit.parallelJobManager.DoneScheduling()

		err = submit.parallelJobManager.Wait()
		if err != nil {
			submit.submitStatusFile.SetErrored()
			submit.submitStatusFile.CreateStatusFile(submit.filesystem, targetPath)
			return xerrors.Errorf("failed to perform parallel jobs: %w", err)
		}

		// status file
		submit.submitStatusFile.SetCompleted()
		err = submit.submitStatusFile.CreateStatusFile(submit.filesystem, targetPath)
		if err != nil {
			return xerrors.Errorf("failed to create status file on %q: %w", targetPath, err)
		}
	}

	return nil
}

func (submit *SubmitCommand) checkValidSourcePath(sourcePath string) error {
	st, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat source %q: %w", sourcePath, err)
	}

	if !st.IsDir() {
		return commons.NewNotDirError(sourcePath)
	}

	// check if source path has metadata in it
	if !commons.HasSubmitMetadataInDir(sourcePath) {
		// metadata path not exist?
		return xerrors.Errorf("source %q must have submit metadata", sourcePath)
	}

	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to readdir source %q: %w", sourcePath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// found dir
			return commons.NewNotFileError(filepath.Join(sourcePath, entry.Name()))
		}
	}

	return nil
}

// scanSourcePaths scans source paths and return valid sources only
func (submit *SubmitCommand) scanSourcePaths(orcID string) ([]string, []string, string, error) {
	validSourcePaths := []string{}
	invalidSourcePaths := []string{}

	for _, sourcePath := range submit.sourcePaths {
		sourcePath = commons.MakeLocalPath(sourcePath)

		st, stErr := os.Stat(sourcePath)
		if stErr != nil {
			if os.IsNotExist(stErr) {
				return nil, nil, "", irodsclient_types.NewFileNotFoundError(sourcePath)
			}

			return nil, nil, "", stErr
		}

		if !st.IsDir() {
			return nil, nil, "", commons.NewNotDirError(sourcePath)
		}

		err := submit.checkValidSourcePath(sourcePath)
		if err == nil {
			// valid
			validSourcePaths = append(validSourcePaths, sourcePath)
			continue
		}

		// may have sub dirs?
		dirEntries, readErr := os.ReadDir(sourcePath)
		if readErr != nil {
			return nil, nil, "", xerrors.Errorf("failed to list source %q: %w", sourcePath, readErr)
		}

		hasSubDirs := false
		for _, dirEntry := range dirEntries {
			if dirEntry.IsDir() {
				hasSubDirs = true

				entryPath := filepath.Join(sourcePath, dirEntry.Name())
				chkErr := submit.checkValidSourcePath(entryPath)
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
			return nil, nil, "", xerrors.Errorf("failed to read ORC-ID from metadata for %q: %w", validSourcePath, err)
		}

		if len(myOrcID) == 0 {
			return nil, nil, "", xerrors.Errorf("failed to read ORC-ID from metadata for %q: %w", validSourcePath, commons.InvalidOrcIDError)
		}

		if len(orcIDFound) == 0 {
			orcIDFound = myOrcID
		}

		if orcIDFound != myOrcID {
			return nil, nil, "", xerrors.Errorf("Lead Contributor's ORC-ID mismatch for %q, expected %s, but got %s: %w", validSourcePath, orcIDFound, myOrcID, commons.InvalidOrcIDError)
		}
	}

	return validSourcePaths, invalidSourcePaths, orcIDFound, nil
}

func (submit *SubmitCommand) submitOne(mdRepoTicket commons.MDRepoTicket, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SubmitCommand",
		"function": "submitOne",
	})

	logger.Debugf("submit %q to %q (ticket: %q)", sourcePath, targetPath, mdRepoTicket.IRODSTicket)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(sourcePath)
		}

		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	targetRootPath := targetPath

	if sourceStat.IsDir() {
		// dir
		targetPath = commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		return submit.submitDir(sourceStat, sourcePath, targetRootPath, targetPath)
	}

	// file
	targetPath = commons.MakeTargetIRODSFilePath(submit.filesystem, sourcePath, targetPath)
	return submit.submitFile(sourceStat, sourcePath, targetRootPath, targetPath)
}

func (submit *SubmitCommand) scheduleSubmit(sourceStat fs.FileInfo, sourcePath string, targetRootPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SubmitCommand",
		"function": "scheduleSubmit",
	})

	threadsRequired := submit.calculateThreadForTransferJob(sourceStat.Size())

	submitTask := func(job *commons.ParallelJob) error {
		manager := job.GetManager()
		fs := manager.GetFilesystem()

		callbackSubmit := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		job.Progress(0, sourceStat.Size(), false)

		logger.Debugf("uploading a file %q to %q", sourcePath, targetPath)

		var uploadErr error
		var uploadResult *irodsclient_fs.FileTransferResult
		notes := []string{}

		// determine how to upload
		transferMode := submit.determineTransferMode(sourceStat.Size())
		switch transferMode {
		case commons.TransferModeRedirect:
			uploadResult, uploadErr = fs.UploadFileRedirectToResource(sourcePath, targetPath, "", threadsRequired, false, true, true, false, callbackSubmit)
			notes = append(notes, "redirect-to-resource", fmt.Sprintf("%d threads", threadsRequired))
		case commons.TransferModeICAT:
			fallthrough
		default:
			uploadResult, uploadErr = fs.UploadFileParallel(sourcePath, targetPath, "", threadsRequired, false, true, true, false, callbackSubmit)
			notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))
		}

		if uploadErr != nil {
			job.Progress(-1, sourceStat.Size(), true)
			return xerrors.Errorf("failed to upload %q to %q: %w", sourcePath, targetPath, uploadErr)
		}

		err := submit.transferReportManager.AddTransfer(uploadResult, commons.TransferMethodPut, uploadErr, notes)
		if err != nil {
			job.Progress(-1, sourceStat.Size(), true)
			return xerrors.Errorf("failed to add transfer report: %w", err)
		}

		logger.Debugf("uploaded a file %q to %q", sourcePath, targetPath)
		job.Progress(sourceStat.Size(), sourceStat.Size(), false)

		job.Done()
		return nil
	}

	// submit status file
	hash, err := irodsclient_util.HashLocalFile(sourcePath, "md5")
	if err != nil {
		return xerrors.Errorf("failed to get hash for %q: %w", sourcePath, err)
	}

	targetRelPath := targetPath
	if strings.HasPrefix(targetPath, fmt.Sprintf("%s/", targetRootPath)) {
		targetRelPath = targetPath[len(targetRootPath)+1:]
	}

	submitStatusEntry := commons.SubmitStatusEntry{
		IRODSPath: targetRelPath,
		Size:      sourceStat.Size(),
		MD5Hash:   hex.EncodeToString(hash),
	}
	submit.submitStatusFile.AddFile(submitStatusEntry)

	// schedule
	err = submit.parallelJobManager.Schedule(sourcePath, submitTask, threadsRequired, progress.UnitsBytes)
	if err != nil {
		return xerrors.Errorf("failed to schedule upload %q to %q: %w", sourcePath, targetPath, err)
	}

	logger.Debugf("scheduled a file upload %q to %q", sourcePath, targetPath)

	return nil
}

func (submit *SubmitCommand) submitFile(sourceStat fs.FileInfo, sourcePath string, targetRootPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SubmitCommand",
		"function": "submitFile",
	})

	commons.MarkIRODSPathMap(submit.updatedPathMap, targetPath)

	targetEntry, err := submit.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			return submit.scheduleSubmit(sourceStat, sourcePath, targetRootPath, targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		return commons.NewNotFileError(targetPath)
	}

	if !submit.forceFlagValues.Force {
		if targetEntry.Size == sourceStat.Size() {
			// compare hash
			if len(targetEntry.CheckSum) > 0 {
				localChecksum, err := irodsclient_util.HashLocalFile(sourcePath, string(targetEntry.CheckSumAlgorithm))
				if err != nil {
					return xerrors.Errorf("failed to get hash for %q: %w", sourcePath, err)
				}

				if bytes.Equal(localChecksum, targetEntry.CheckSum) {
					// skip
					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:                  commons.TransferMethodPut,
						StartAt:                 now,
						EndAt:                   now,
						SourcePath:              sourcePath,
						SourceSize:              sourceStat.Size(),
						SourceChecksumAlgorithm: string(targetEntry.CheckSumAlgorithm),
						SourceChecksum:          hex.EncodeToString(localChecksum),
						DestPath:                targetEntry.Path,
						DestSize:                targetEntry.Size,
						DestChecksum:            hex.EncodeToString(targetEntry.CheckSum),
						DestChecksumAlgorithm:   string(targetEntry.CheckSumAlgorithm),
						Notes:                   []string{"differential", "same checksum", "skip"},
					}

					submit.transferReportManager.AddFile(reportFile)

					commons.Printf("skip uploading a file %q to %q. The file with the same hash already exists!\n", sourcePath, targetPath)
					logger.Debugf("skip uploading a file %q to %q. The file with the same hash already exists!", sourcePath, targetPath)

					// add skipped status entry
					hash, err := irodsclient_util.HashLocalFile(sourcePath, "md5")
					if err != nil {
						return xerrors.Errorf("failed to get hash for %q: %w", sourcePath, err)
					}

					targetRelPath := targetPath
					if strings.HasPrefix(targetPath, fmt.Sprintf("%s/", targetRootPath)) {
						targetRelPath = targetPath[len(targetRootPath)+1:]
					}

					submitStatusEntry := commons.SubmitStatusEntry{
						IRODSPath: targetRelPath,
						Size:      targetEntry.Size,
						MD5Hash:   hex.EncodeToString(hash),
					}
					submit.submitStatusFile.AddFile(submitStatusEntry)
					return nil
				}
			}
		}
	}

	// schedule
	return submit.scheduleSubmit(sourceStat, sourcePath, targetRootPath, targetPath)
}

func (submit *SubmitCommand) submitDir(sourceStat fs.FileInfo, sourcePath string, targetRootPath string, targetPath string) error {
	commons.MarkIRODSPathMap(submit.updatedPathMap, targetPath)

	targetEntry, err := submit.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a directory with new name
			err = submit.filesystem.MakeDir(targetPath, true)
			if err != nil {
				return xerrors.Errorf("failed to make a collection %q: %w", targetPath, err)
			}

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodPut,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourcePath,
				DestPath:   targetPath,
				Notes:      []string{"directory"},
			}

			submit.transferReportManager.AddFile(reportFile)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetEntry.IsDir() {
			return commons.NewNotDirError(targetPath)
		}
	}

	// get entries
	entries, err := os.ReadDir(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to list a directory %q: %w", sourcePath, err)
	}

	for _, entry := range entries {
		newEntryPath := commons.MakeTargetIRODSFilePath(submit.filesystem, entry.Name(), targetPath)

		entryPath := filepath.Join(sourcePath, entry.Name())

		entryStat, err := os.Stat(entryPath)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(entryPath)
			}

			return xerrors.Errorf("failed to stat %q: %w", entryPath, err)
		}

		if entryStat.IsDir() {
			// dir
			err = submit.submitDir(entryStat, entryPath, targetRootPath, newEntryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			err = submit.submitFile(entryStat, entryPath, targetRootPath, newEntryPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (submit *SubmitCommand) calculateThreadForTransferJob(size int64) int {
	threads := commons.CalculateThreadForTransferJob(size, submit.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to upload
	if submit.parallelTransferFlagValues.SingleThread || submit.parallelTransferFlagValues.ThreadNumber == 1 || submit.parallelTransferFlagValues.ThreadNumberPerFile == 1 {
		return 1
	} else if submit.parallelTransferFlagValues.Icat && !submit.filesystem.SupportParallelUpload() {
		return 1
	} else if submit.parallelTransferFlagValues.RedirectToResource || submit.parallelTransferFlagValues.Icat {
		return threads
	}

	//if size < commons.RedirectToResourceMinSize && !put.filesystem.SupportParallelUpload() {
	//	// icat
	//	return 1
	//}

	if !submit.filesystem.SupportParallelUpload() {
		return 1
	}

	return threads
}

func (submit *SubmitCommand) determineTransferMode(size int64) commons.TransferMode {
	if submit.parallelTransferFlagValues.RedirectToResource {
		return commons.TransferModeRedirect
	} else if submit.parallelTransferFlagValues.Icat {
		return commons.TransferModeICAT
	}

	// auto
	//if size >= commons.RedirectToResourceMinSize {
	//	return commons.TransferModeRedirect
	//}

	return commons.TransferModeICAT
}
