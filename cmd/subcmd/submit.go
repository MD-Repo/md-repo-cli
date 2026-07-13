package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons/checksum"
	"github.com/MD-Repo/md-repo-cli/commons/config"
	"github.com/MD-Repo/md-repo-cli/commons/irods"
	"github.com/MD-Repo/md-repo-cli/commons/mdrepo"
	"github.com/MD-Repo/md-repo-cli/commons/parallel"
	commons_path "github.com/MD-Repo/md-repo-cli/commons/path"
	"github.com/MD-Repo/md-repo-cli/commons/terminal"
	"github.com/MD-Repo/md-repo-cli/commons/transfer"
	"github.com/MD-Repo/md-repo-cli/commons/types"
	"github.com/MD-Repo/md-repo-cli/commons/webdav"
	"github.com/avast/retry-go"
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
)

var submitCmd = &cobra.Command{
	Use:     "submit <data dirs> ...",
	Short:   "Submit local data to MD-Repo",
	Long:    "This command submits local data to MD-Repo",
	Aliases: []string{"upload", "up", "put", "contribute"},
	RunE:    processSubmitCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddSubmitCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitCmd)

	flag.SetSubmissionFlags(submitCmd)
	flag.SetTokenFlags(submitCmd)
	flag.SetParallelTransferFlags(submitCmd, false, false)
	flag.SetForceFlags(submitCmd, true)
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
	parallelTransferFlagValues *flag.ParallelTransferFlagValues
	forceFlagValues            *flag.ForceFlagValues
	progressFlagValues         *flag.ProgressFlagValues
	retryFlagValues            *flag.RetryFlagValues
	transferReportFlagValues   *flag.TransferReportFlagValues

	maxConnectionNum int

	account      *irodsclient_types.IRODSAccount
	filesystem   *irodsclient_fs.FileSystem
	webdavClient *webdav.WebDAVClient

	sourcePaths []string

	parallelTransferJobManager *parallel.ParallelJobManager
	transferReportManager      *transfer.TransferReportManager
	config                     *config.Config
	submitStatusFileWriter     *mdrepo.SubmitStatusFileWriter

	totalUploadedFiles int
	totalUploadedBytes int64
	startTime          time.Time
}

func NewSubmitCommand(command *cobra.Command, args []string) (*SubmitCommand, error) {
	submit := &SubmitCommand{
		command: command,

		commonFlagValues:           flag.GetCommonFlagValues(command),
		submissionFlagValues:       flag.GetSubmissionFlagValues(),
		tokenFlagValues:            flag.GetTokenFlagValues(),
		parallelTransferFlagValues: flag.GetParallelTransferFlagValues(),
		forceFlagValues:            flag.GetForceFlagValues(),
		progressFlagValues:         flag.GetProgressFlagValues(),
		retryFlagValues:            flag.GetRetryFlagValues(),
		transferReportFlagValues:   flag.GetTransferReportFlagValues(command),

		config:             config.GetConfig(),
		totalUploadedFiles: 0,
		totalUploadedBytes: 0,
		startTime:          time.Now(),
	}

	submit.maxConnectionNum = submit.parallelTransferFlagValues.ThreadNumber

	// path
	submit.sourcePaths = args

	return submit, nil
}

func (submit *SubmitCommand) Process() error {
	logger := log.WithFields(log.Fields{})
	terminal.Printf("submitting data to MD-Repo\n")

	cont, err := flag.ProcessCommonFlags(submit.command)
	if err != nil {
		return errors.Wrapf(err, "Failed to process common flags")
	}
	if !cont {
		return nil
	}

	// handle token
	if len(submit.tokenFlagValues.TicketString) > 0 {
		submit.config.TicketString = submit.tokenFlagValues.TicketString
	}

	if len(submit.tokenFlagValues.Token) > 0 {
		submit.config.Token = submit.tokenFlagValues.Token
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	// validate source paths
	terminal.Printf("validating metadata files...\n")
	validSourcePaths, invalidSourcePaths, invalidSourcePathsErrors, orcID, err := submit.scanSourcePaths(submit.submissionFlagValues.OrcID)
	if err != nil {
		return errors.Wrapf(err, "Failed to scan source paths")
	}

	// check if the number of simulations matches the expected number
	expectedSimulationNo := 0
	if submit.submissionFlagValues.ExpectedSimulations > 0 {
		expectedSimulationNo = submit.submissionFlagValues.ExpectedSimulations
	} else {
		expectedSimulationNo = terminal.InputInt("Number of simulations expected")
	}

	if expectedSimulationNo != len(validSourcePaths) {
		logger.Debugf("we found %d simulations, but expected %d simulations", len(validSourcePaths), expectedSimulationNo)

		logger.Debugf("the simulations found:")
		for sourceIdx, sourcePath := range validSourcePaths {
			logger.Debugf("[%d] %s", sourceIdx+1, sourcePath)
		}

		logger.Debugf("the directories ignored due to lack of metadata file:")
		for sourceIdx, sourcePath := range invalidSourcePaths {
			if len(invalidSourcePathsErrors) > sourceIdx {
				logger.Debugf("[%d] %s: %s", sourceIdx+1, sourcePath, invalidSourcePathsErrors[sourceIdx])
			} else {
				logger.Debugf("[%d] %s", sourceIdx+1, sourcePath)
			}
		}

		return types.NewSimulationNoNotMatchingError(validSourcePaths, invalidSourcePaths, invalidSourcePathsErrors, expectedSimulationNo)
	}

	if len(orcID) == 0 {
		orcID = terminal.Input("Input ORCID")
	}

	if len(submit.config.Token) > 0 && len(submit.config.TicketString) == 0 {
		// encrypt
		tokenBytes, err := checksum.Base64Decode(submit.config.Token)
		if err != nil {
			return errors.Wrapf(err, "Failed to decode token using BASE64")
		}

		newToken, err := checksum.HMACStringSHA224(tokenBytes, orcID)
		if err != nil {
			return errors.Wrapf(err, "Failed to encrypt token using SHA3-224 HMAC")
		}

		logger.Debugf("encrypted token: %s", newToken)

		submit.config.TicketString, err = mdrepo.GetMDRepoTicketStringFromToken(submit.tokenFlagValues.ServiceURL, newToken)
		if err != nil {
			return errors.Wrapf(err, "Failed to read ticket from token")
		}
	}

	if len(submit.config.TicketString) == 0 {
		return types.NewTokenNotProvidedError()
	}

	// verify metadata first
	for _, sourcePath := range validSourcePaths {
		metadata, err := mdrepo.ParseSubmitMetadataDir(sourcePath)
		if err != nil {
			return errors.Wrapf(err, "Failed to parse submit metadata in dir %q", sourcePath)
		}

		err = metadata.ValidateFiles()
		if err != nil {
			return errors.Wrapf(err, "Failed to validate local files listed in the metadata file %q", metadata.MetadataFilePath)
		}
	}

	// verify metadata via server
	invalidErr := mdrepo.VerifySubmitMetadataViaServer(validSourcePaths, submit.tokenFlagValues.ServiceURL, submit.config.Token, submit.submissionFlagValues.NoID)
	if invalidErr != nil {
		return invalidErr
	}

	terminal.Printf("all metadata files are valid\n")
	logger.Debugf("all submit metadata are valid")

	// get ticket
	mdRepoTickets, err := mdrepo.GetMDRepoTicketsFromString(submit.config.TicketString)
	if err != nil {
		return errors.Wrapf(err, "Failed to retrieve tickets")
	}

	if len(mdRepoTickets) != len(validSourcePaths) {
		logger.Debugf("we found %d simulations, but we got %d tokens", len(mdRepoTickets), len(validSourcePaths))
	}

	// transfer report
	submit.transferReportManager, err = transfer.NewTransferReportManager(submit.transferReportFlagValues.Report, submit.transferReportFlagValues.ReportPath, submit.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return errors.Wrapf(err, "Failed to create transfer report manager")
	}
	defer submit.transferReportManager.Release()

	// run
	for ticketIdx, mdRepoTicket := range mdRepoTickets {
		sourcePath := validSourcePaths[ticketIdx]
		err = submit.processTicket(sourcePath, &mdRepoTicket)
		if err != nil {
			return err
		}
	}

	// print final summary
	if !submit.progressFlagValues.NoProgress {
		timeTaken := time.Since(submit.startTime).Seconds()
		totalUploadedSize := types.SizeString(submit.totalUploadedBytes)
		bps := float64(submit.totalUploadedBytes) / timeTaken
		bpsString := fmt.Sprintf("%s/s", types.SizeString(int64(bps)))
		terminal.Printf("Uploaded %d files, %s in total, time taken: %.2f seconds, average speed: %s\n", submit.totalUploadedFiles, totalUploadedSize, timeTaken, bpsString)
	}

	return nil
}

func (submit *SubmitCommand) processTicket(sourcePath string, mdRepoTicket *mdrepo.MDRepoTicket) error {
	// we create filesystem, job manager for every ticket as they require separate auth
	// Create a file system
	account, err := mdRepoTicket.GetAccount()
	if err != nil {
		return errors.Wrapf(err, "Failed to get iRODS Account")
	}

	submit.account = account

	submit.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(submit.account, submit.maxConnectionNum, submit.parallelTransferFlagValues.TCPBufferSize, true, submit.commonFlagValues.Timeout)
	if err != nil {
		return errors.Wrapf(err, "Failed to get iRODS FS Client")
	}
	defer func() {
		submit.filesystem.Release()
		submit.filesystem = nil
	}()

	if submit.parallelTransferFlagValues.WebDAV {
		webdavClient, err := webdav.NewWebDAVClient(submit.filesystem, config.MDRepoWebDAVServerURL+config.MDRepoWebDAVPrefix, submit.account.ProxyUser, submit.account.Password)
		if err != nil {
			return errors.Wrapf(err, "Failed to create WebDAV client")
		}

		submit.webdavClient = webdavClient
	}

	// parallel job manager
	ioSession := submit.filesystem.GetIOSession()
	submit.parallelTransferJobManager = parallel.NewParallelJobManager(ioSession.GetMaxConnections(), !submit.progressFlagValues.NoProgress, submit.progressFlagValues.ShowFullPath, submit.parallelTransferFlagValues.StopOnError)

	// run
	targetPath := commons_path.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

	// setup submit status file writer
	submit.submitStatusFileWriter = mdrepo.NewSubmitStatusFileWriter(submit.filesystem, submit.config.Token, targetPath)

	// run
	err = submit.submitOne(mdRepoTicket, sourcePath)
	if err != nil {
		submit.submitStatusFileWriter.SetErrored()
		submit.submitStatusFileWriter.CreateStatusFile()
		return errors.Wrapf(err, "Failed to submit %q to %q", sourcePath, targetPath)
	}

	// create a in-progress status file
	submit.submitStatusFileWriter.SetInProgress()
	err = submit.submitStatusFileWriter.CreateStatusFile()
	if err != nil {
		return errors.Wrapf(err, "Failed to create status file on %q", targetPath)
	}

	defer func() {
		submit.submitStatusFileWriter.CreateStatusFile()
		submit.submitStatusFileWriter = nil
	}()

	transferErr := submit.parallelTransferJobManager.Start()
	if transferErr != nil {
		submit.submitStatusFileWriter.SetErrored()
		return errors.Wrap(transferErr, "failed to perform transfer jobs")
	}

	// set completed
	submit.submitStatusFileWriter.SetCompleted()
	return nil
}

// scanSourcePaths scans source paths and return valid sources only
func (submit *SubmitCommand) scanSourcePaths(orcID string) ([]string, []string, []error, string, error) {
	validSourcePaths := []string{}
	invalidSourcePaths := []string{}
	invalidSourcePathsErrors := []error{}

	for _, sourcePath := range submit.sourcePaths {
		sourcePath = commons_path.MakeLocalPath(sourcePath)

		st, stErr := os.Stat(sourcePath)
		if stErr != nil {
			if os.IsNotExist(stErr) {
				return nil, nil, nil, "", errors.Join(stErr, irodsclient_types.NewFileNotFoundError(sourcePath))
			}

			return nil, nil, nil, "", stErr
		}

		if !st.IsDir() {
			return nil, nil, nil, "", types.NewNotDirError(sourcePath)
		}

		err := mdrepo.ValidateSubmissionSourcePath(sourcePath)
		if err == nil {
			// valid
			validSourcePaths = append(validSourcePaths, sourcePath)
			continue
		}

		// may have sub dirs?
		dirEntries, readErr := os.ReadDir(sourcePath)
		if readErr != nil {
			return nil, nil, nil, "", errors.Wrapf(readErr, "Failed to list source %q", sourcePath)
		}

		hasSubDirs := false
		for _, dirEntry := range dirEntries {
			if dirEntry.IsDir() {
				hasSubDirs = true

				entryPath := filepath.Join(sourcePath, dirEntry.Name())
				chkErr := mdrepo.ValidateSubmissionSourcePath(entryPath)
				if chkErr == nil {
					// valid
					validSourcePaths = append(validSourcePaths, entryPath)
				} else {
					// invalid
					invalidSourcePaths = append(invalidSourcePaths, entryPath)
					invalidSourcePathsErrors = append(invalidSourcePathsErrors, chkErr)
				}
			}
		}

		if !hasSubDirs {
			// invalid
			invalidSourcePaths = append(invalidSourcePaths, sourcePath)
			invalidSourcePathsErrors = append(invalidSourcePathsErrors, err)
		}
	}

	// sort source paths by name to match to tickets always in the same order
	slices.Sort(validSourcePaths)

	// check files: no zero-length files and no duplicate MD5 hashes
	hashToFiles := map[string][]string{} // md5 hex -> all file paths sharing it
	for _, validSourcePath := range validSourcePaths {
		entries, err := os.ReadDir(validSourcePath)
		if err != nil {
			return nil, nil, nil, "", errors.Wrapf(err, "Failed to read dir %q", validSourcePath)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filePath := filepath.Join(validSourcePath, entry.Name())

			info, err := entry.Info()
			if err != nil {
				return nil, nil, nil, "", errors.Wrapf(err, "Failed to stat %q", filePath)
			}

			if info.Size() == 0 {
				return nil, nil, nil, "", errors.Errorf("file %q is empty", filePath)
			}

			hash, err := irodsclient_util.HashLocalFile(filePath, "md5", nil)
			if err != nil {
				return nil, nil, nil, "", errors.Wrapf(err, "Failed to compute MD5 for %q", filePath)
			}

			hashStr := hex.EncodeToString(hash)
			hashToFiles[hashStr] = append(hashToFiles[hashStr], filePath)
		}
	}

	duplicateMessages := []string{}
	for hashStr, files := range hashToFiles {
		if len(files) > 1 {
			duplicateMessages = append(duplicateMessages, fmt.Sprintf("hash %s: %s", hashStr, strings.Join(files, ", ")))
		}
	}
	if len(duplicateMessages) > 0 {
		slices.Sort(duplicateMessages)
		return nil, nil, nil, "", errors.Errorf("duplicate MD5 hashes found:\n%s", strings.Join(duplicateMessages, "\n"))
	}

	// if orcID is given, override the orcID
	if len(orcID) > 0 {
		return validSourcePaths, invalidSourcePaths, invalidSourcePathsErrors, orcID, nil
	}

	orcIDFound := ""
	for _, validSourcePath := range validSourcePaths {
		metadataPath := mdrepo.GetSubmitMetadataPath(validSourcePath)
		submitMetadata, err := mdrepo.ParseSubmitMetadataFile(metadataPath)

		if err != nil {
			return nil, nil, nil, "", errors.Wrapf(err, "Failed to parse metadata for %q", validSourcePath)
		}

		myOrcID, err := submitMetadata.GetOrcID()
		if err != nil {
			return nil, nil, nil, "",
				errors.Wrapf(err, "Failed to parse %q", validSourcePath)
		}

		if len(orcIDFound) == 0 {
			orcIDFound = myOrcID
		}

		if orcIDFound != myOrcID {
			return nil, nil, nil, "", errors.Errorf("Lead Contributor's ORCID mismatch for %q, expected %s, but got %s: %w", validSourcePath, orcIDFound, myOrcID, types.NewInvalidOrcIDError(myOrcID, orcIDFound))
		}
	}

	return validSourcePaths, invalidSourcePaths, invalidSourcePathsErrors, orcIDFound, nil
}

func (submit *SubmitCommand) submitOne(mdRepoTicket *mdrepo.MDRepoTicket, sourcePath string) error {
	logger := log.WithFields(log.Fields{
		"irods_data_path": mdRepoTicket.IRODSDataPath,
		"irods_ticket":    mdRepoTicket.IRODSTicket,
		"source_path":     sourcePath,
	})

	targetPath := commons_path.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

	logger.Debugf("upload %q to %q (ticket: %q)", sourcePath, targetPath, mdRepoTicket.IRODSTicket)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Join(err, irodsclient_types.NewFileNotFoundError(sourcePath))
		}

		return errors.Wrapf(err, "Failed to stat %q", sourcePath)
	}

	if !sourceStat.IsDir() {
		// file is provided
		return errors.Errorf("source path must be a directory")
	}

	metadata, err := mdrepo.ParseSubmitMetadataDir(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse submit metadata in dir %q", sourcePath)
	}

	sourceFiles := metadata.GetFiles()

	hasMetadata := false
	for _, sourceFile := range sourceFiles {
		if filepath.Base(sourceFile) == mdrepo.SubmissionMetadataFilename {
			hasMetadata = true
		}
	}

	if !hasMetadata {
		// include submission metadata file itself
		sourceFiles = append(sourceFiles, mdrepo.SubmissionMetadataFilename)
	}

	for _, sourceFile := range sourceFiles {
		sourceFileAbsPath := filepath.Join(metadata.SubmissionPath, sourceFile)
		sourceFileStat, err := os.Stat(sourceFileAbsPath)
		if err != nil {
			return errors.Wrapf(err, "Failed to stat source file %q", sourceFileAbsPath)
		}

		targetFilePath := path.Join(targetPath, sourceFile)

		submitErr := submit.submitFile(mdRepoTicket, sourceFileStat, sourceFileAbsPath, targetPath, targetFilePath)
		if submitErr != nil {
			return submitErr
		}

		// add status entry
		hash, err := irodsclient_util.HashLocalFile(sourcePath, "md5", nil)
		if err != nil {
			return errors.Wrapf(err, "Failed to get hash for %q", sourcePath)
		}

		submitStatusEntry := mdrepo.SubmitStatusEntry{
			IRODSPath: commons_path.GetIRODSRelativePath(targetPath, targetFilePath),
			Size:      sourceStat.Size(),
			MD5Hash:   hex.EncodeToString(hash),
		}
		submit.submitStatusFileWriter.AddFile(submitStatusEntry)
	}

	return nil
}

func (submit *SubmitCommand) submitFile(mdRepoTicket *mdrepo.MDRepoTicket, sourceStat fs.FileInfo, sourcePath string, targetRootPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"irods_data_path":  mdRepoTicket.IRODSDataPath,
		"irods_ticket":     mdRepoTicket.IRODSTicket,
		"source_path":      sourcePath,
		"target_root_path": targetRootPath,
		"target_path":      targetPath,
	})

	defaultNotes := []string{"put"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodPut,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		submit.transferReportManager.AddFile(reportFile)
	}

	reportOverwrite := func(startTime time.Time, endTime time.Time, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  startTime,
			EndAt:    endTime,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		submit.transferReportManager.AddFile(reportFile)
	}

	targetEntry, err := submit.filesystem.Stat(targetPath)
	if err != nil {
		if irodsclient_types.IsFileNotFoundError(err) {
			// target does not exist
			// target must be a file with new name
			submit.scheduleSubmit(mdRepoTicket, sourceStat, sourcePath, targetRootPath, targetPath)
			return nil
		}

		reportSimple(err)
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exists
	// target must be a file
	if targetEntry.IsDir() {
		notFileErr := types.NewNotFileError(targetPath)
		now := time.Now()
		reportOverwrite(now, now, notFileErr, "directory")
		return notFileErr
	}

	if !submit.forceFlagValues.Force {
		if targetEntry.Size == sourceStat.Size() {
			// compare hash
			if len(targetEntry.CheckSum) > 0 {
				localChecksum, err := irodsclient_util.HashLocalFile(sourcePath, string(targetEntry.CheckSumAlgorithm), nil)
				if err != nil {
					return errors.Wrapf(err, "Failed to get hash for %q", sourcePath)
				}

				if bytes.Equal(localChecksum, targetEntry.CheckSum) {
					// skip
					now := time.Now()
					reportFile := &transfer.TransferReportFile{
						Method:                  transfer.TransferMethodPut,
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

						Notes: []string{"put", "file", "differential", "same checksum", "skipped"},
					}

					submit.transferReportManager.AddFile(reportFile)

					terminal.Printf("skip uploading a file %q to %q. The data object with the same hash already exists!\n", sourcePath, targetPath)
					logger.Debug("skip uploading a file. The data object with the same hash already exists!")
					return nil
				}
			}
		}
	}

	// schedule
	return submit.scheduleSubmit(mdRepoTicket, sourceStat, sourcePath, targetRootPath, targetPath)
}

func (submit *SubmitCommand) scheduleSubmit(mdRepoTicket *mdrepo.MDRepoTicket, sourceStat fs.FileInfo, sourcePath string, targetRootPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"irods_data_path":  mdRepoTicket.IRODSDataPath,
		"irods_ticket":     mdRepoTicket.IRODSTicket,
		"source_path":      sourcePath,
		"target_root_path": targetRootPath,
		"target_path":      targetPath,
	})

	defaultNotes := []string{"put"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodPut,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourcePath,
			SourceSize: sourceStat.Size(),
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		submit.transferReportManager.AddFile(reportFile)
	}

	reportTransfer := func(result *irodsclient_fs.FileTransferResult, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		submit.transferReportManager.AddTransfer(result, transfer.TransferMethodPut, err, newNotes)
	}

	transferMode, threadsRequired := submit.determineTransferMethod(sourceStat.Size())

	submitTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("upload", -1, sourceStat.Size(), true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for uploading a file")
			return nil
		}

		logger.Debug("uploading a file")

		notes := []string{}

		progressCallbackPut := func(taskType string, processed int64, total int64) {
			job.Progress(taskType, processed, total, false)
		}

		job.Progress("upload", 0, sourceStat.Size(), false)

		uploadSourcePath := sourcePath

		// check parent is not available using ticket
		// parentTargetPath := path.Dir(targetPath)
		// _, statErr := submit.filesystem.Stat(parentTargetPath)
		// if statErr != nil {
		// 	// must exist, mkdir is performed at putDir
		// 	job.Progress("upload", -1, sourceStat.Size(), true)

		// 	reportSimple(statErr)
		// 	return errors.Wrapf(statErr, "failed to stat %q", parentTargetPath)
		// }

		var uploadErr error
		var uploadResult *irodsclient_fs.FileTransferResult

		notes = append(notes, string(transferMode), fmt.Sprintf("%d threads", threadsRequired))

		retryNum := submit.retryFlagValues.GetRetryNumber()
		retryInterval := submit.retryFlagValues.GetRetryIntervalSeconds()

		attempt := 0
		retryErr := retry.Do(func() error {
			attempt++
			if attempt > 1 {
				logger.Debugf("retrying upload attempt %d/%d for %q", attempt, retryNum+1, sourcePath)
			}

			switch transferMode {
			case transfer.TransferModeWebDAV:
				// this may not work with ticket
				uploadResult, uploadErr = submit.webdavClient.UploadFile(uploadSourcePath, targetPath, "", true, progressCallbackPut)
			case transfer.TransferModeICAT:
				fallthrough
			default:
				uploadResult, uploadErr = submit.filesystem.UploadFileParallel(uploadSourcePath, targetPath, "", threadsRequired, false, true, progressCallbackPut)
			}
			return uploadErr
		}, retry.Attempts(uint(retryNum+1)), retry.Delay(retryInterval), retry.LastErrorOnly(true))

		if retryErr != nil {
			job.Progress("upload", -1, sourceStat.Size(), true)
			job.Progress("checksum", -1, sourceStat.Size(), true)

			reportTransfer(uploadResult, retryErr, notes...)
			return errors.Wrapf(retryErr, "failed to upload %q to %q after %d attempts", sourcePath, targetPath, retryNum+1)
		}

		submit.totalUploadedFiles++
		submit.totalUploadedBytes += sourceStat.Size()

		reportTransfer(uploadResult, nil, notes...)

		logger.Debug("uploaded a file")
		return nil
	}

	submit.parallelTransferJobManager.Schedule(sourcePath, submitTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a file upload, %d threads", threadsRequired)

	return nil
}

func (submit *SubmitCommand) determineTransferMethod(size int64) (transfer.TransferMode, int) {
	logger := log.WithFields(log.Fields{})

	threads := parallel.CalculateThreadForTransferJob(size, submit.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to upload
	if submit.parallelTransferFlagValues.SingleThread || submit.parallelTransferFlagValues.ThreadNumber <= 2 || submit.parallelTransferFlagValues.ThreadNumberPerFile == 1 || !submit.filesystem.SupportParallelUpload() {
		threads = 1
	}

	// we don't support webdav transfer here
	logger.Info("using ICAT transfer for uploading a data object")
	return transfer.TransferModeICAT, threads
}
