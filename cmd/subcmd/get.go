package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/avast/retry-go"
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/jedib0t/go-pretty/v6/progress"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons/config"
	"github.com/MD-Repo/md-repo-cli/commons/irods"
	"github.com/MD-Repo/md-repo-cli/commons/mdrepo"
	"github.com/MD-Repo/md-repo-cli/commons/parallel"
	commons_path "github.com/MD-Repo/md-repo-cli/commons/path"
	"github.com/MD-Repo/md-repo-cli/commons/terminal"
	"github.com/MD-Repo/md-repo-cli/commons/transfer"
	"github.com/MD-Repo/md-repo-cli/commons/types"
	"github.com/MD-Repo/md-repo-cli/commons/webdav"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:     "get [local dir]",
	Short:   "Download MD-Repo data to a local directory",
	Aliases: []string{"download", "down"},
	Long:    `This downloads MD-Repo data to the specified local directory.`,
	RunE:    processGetCommand,
	Args:    cobra.MaximumNArgs(1),
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(getCmd)

	flag.SetTokenFlags(getCmd)
	flag.SetParallelTransferFlags(getCmd, false, false)
	flag.SetForceFlags(getCmd, false)
	flag.SetProgressFlags(getCmd)
	flag.SetRetryFlags(getCmd)
	flag.SetTransferReportFlags(getCmd)

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	get, err := NewGetCommand(command, args)
	if err != nil {
		return err
	}

	return get.Process()
}

type GetCommand struct {
	command *cobra.Command

	commonFlagValues           *flag.CommonFlagValues
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

	targetPath string

	parallelTransferJobManager *parallel.ParallelJobManager

	transferReportManager *transfer.TransferReportManager
	config                *config.Config

	totalDownloadedFiles int
	totalDownloadedBytes int64
	startTime            time.Time
}

func NewGetCommand(command *cobra.Command, args []string) (*GetCommand, error) {
	get := &GetCommand{
		command: command,

		commonFlagValues:           flag.GetCommonFlagValues(command),
		tokenFlagValues:            flag.GetTokenFlagValues(),
		parallelTransferFlagValues: flag.GetParallelTransferFlagValues(),
		forceFlagValues:            flag.GetForceFlagValues(),
		progressFlagValues:         flag.GetProgressFlagValues(),
		retryFlagValues:            flag.GetRetryFlagValues(),
		transferReportFlagValues:   flag.GetTransferReportFlagValues(command),

		config:               config.GetConfig(),
		totalDownloadedFiles: 0,
		totalDownloadedBytes: 0,
		startTime:            time.Now(),
	}

	get.maxConnectionNum = get.parallelTransferFlagValues.ThreadNumber

	// path
	get.targetPath = "./"

	if len(args) > 0 {
		get.targetPath = args[0]
	}

	return get, nil
}

func (get *GetCommand) Process() error {
	terminal.Printf("downloading MD-Repo data to a local directory\n")

	cont, err := flag.ProcessCommonFlags(get.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle token
	if len(get.tokenFlagValues.TicketString) > 0 {
		get.config.TicketString = get.tokenFlagValues.TicketString
	}

	if len(get.tokenFlagValues.Token) > 0 {
		get.config.Token = get.tokenFlagValues.Token
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	if len(get.config.Token) > 0 && len(get.config.TicketString) == 0 {
		get.config.TicketString, err = mdrepo.GetMDRepoTicketStringFromToken(get.tokenFlagValues.ServiceURL, get.config.Token)
		if err != nil {
			return errors.Wrapf(err, "failed to read ticket from token %q", get.config.Token)
		}
	}

	if len(get.config.TicketString) == 0 {
		return types.NewTokenNotProvidedError()
	}

	// get ticket
	mdRepoTickets, err := mdrepo.GetMDRepoTicketsFromString(get.config.TicketString)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve tickets")
	}

	// transfer report
	get.transferReportManager, err = transfer.NewTransferReportManager(get.transferReportFlagValues.Report, get.transferReportFlagValues.ReportPath, get.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return errors.Wrapf(err, "failed to create transfer report manager")
	}
	defer get.transferReportManager.Release()

	// run
	err = get.ensureTargetIsDir(get.targetPath)
	if err != nil {
		return err
	}

	// group tickets by IRODSTicket to share filesystem and parallel job manager
	ticketGroups := make(map[string][]mdrepo.MDRepoTicket)
	ticketGroupOrder := []string{}
	for _, mdRepoTicket := range mdRepoTickets {
		if _, exists := ticketGroups[mdRepoTicket.IRODSTicket]; !exists {
			ticketGroupOrder = append(ticketGroupOrder, mdRepoTicket.IRODSTicket)
		}
		ticketGroups[mdRepoTicket.IRODSTicket] = append(ticketGroups[mdRepoTicket.IRODSTicket], mdRepoTicket)
	}

	for _, irodsTicket := range ticketGroupOrder {
		group := ticketGroups[irodsTicket]
		err = get.processTicketGroup(group)
		if err != nil {
			return err
		}
	}

	// print final summary
	if !get.progressFlagValues.NoProgress {
		timeTaken := time.Since(get.startTime).Seconds()
		totalDownloadedSize := types.SizeString(get.totalDownloadedBytes)
		bps := float64(get.totalDownloadedBytes) / timeTaken
		bpsString := fmt.Sprintf("%s/s", types.SizeString(int64(bps)))
		terminal.Printf("Downloaded %d files, %s in total, time taken: %.2f seconds, average speed: %s\n", get.totalDownloadedFiles, totalDownloadedSize, timeTaken, bpsString)
	}

	return nil
}

func (get *GetCommand) processTicketGroup(mdRepoTickets []mdrepo.MDRepoTicket) error {
	if len(mdRepoTickets) == 0 {
		return nil
	}

	// all tickets in the group share the same IRODSTicket, so they use the same account
	account, err := mdRepoTickets[0].GetAccount()
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS Account")
	}

	get.account = account

	get.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(get.account, get.maxConnectionNum, get.parallelTransferFlagValues.TCPBufferSize, true, get.commonFlagValues.Timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer func() {
		get.filesystem.Release()
		get.filesystem = nil
	}()

	if get.parallelTransferFlagValues.WebDAV {
		webdavClient, err := webdav.NewWebDAVClient(get.filesystem, config.MDRepoWebDAVServerURL+config.MDRepoWebDAVPrefix, get.account.ProxyUser, get.account.Password)
		if err != nil {
			return errors.Wrap(err, "failed to create WebDAV client")
		}

		get.webdavClient = webdavClient
	}

	// parallel job manager - created once for the entire ticket group
	ioSession := get.filesystem.GetIOSession()
	get.parallelTransferJobManager = parallel.NewParallelJobManager(ioSession.GetMaxConnections(), !get.progressFlagValues.NoProgress, get.progressFlagValues.ShowFullPath, get.parallelTransferFlagValues.StopOnError)

	// schedule all paths in this ticket group
	terminal.Printf("scheduling transfer...\n")

	for i := range mdRepoTickets {
		mdRepoTicket := &mdRepoTickets[i]

		dataRelPath, err := mdrepo.GetMDRepoSimulationRelPath(mdRepoTicket.IRODSDataPath)
		if err != nil {
			return errors.Wrapf(err, "failed to extract data path from %q", mdRepoTicket.IRODSDataPath)
		}

		dataTargetPath := filepath.Join(get.targetPath, filepath.FromSlash(dataRelPath))
		targetParentDir := filepath.Dir(dataTargetPath)
		err = os.MkdirAll(targetParentDir, 0766)
		if err != nil {
			return errors.Wrapf(err, "failed to make a directory %q", targetParentDir)
		}

		err = get.getOne(mdRepoTicket, dataTargetPath)
		if err != nil {
			return errors.Wrapf(err, "failed to get %q to %q", mdRepoTicket.IRODSDataPath, dataTargetPath)
		}
	}

	// start all scheduled transfers at once
	terminal.Printf("start transfer...\n")

	transferErr := get.parallelTransferJobManager.Start()
	if transferErr != nil {
		return errors.Wrap(transferErr, "failed to perform transfer jobs")
	}

	terminal.Printf("done transfer...\n")

	return nil
}

func (get *GetCommand) ensureTargetIsDir(targetPath string) error {
	targetPath = commons_path.MakeLocalPath(targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// not exist
			return types.NewNotDirError(targetPath)
		}

		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	if !targetStat.IsDir() {
		return types.NewNotDirError(targetPath)
	}

	return nil
}

func (get *GetCommand) hasTransferStatusFile(targetPath string) bool {
	// check transfer status file
	trxStatusFilePath := irodsclient_irodsfs.GetDataObjectTransferStatusFilePath(targetPath)
	_, err := os.Stat(trxStatusFilePath)
	return err == nil
}

func (get *GetCommand) getOne(mdRepoTicket *mdrepo.MDRepoTicket, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"irods_data_path": mdRepoTicket.IRODSDataPath,
		"irods_ticket":    mdRepoTicket.IRODSTicket,
		"targetPath":      targetPath,
	})

	sourcePath := commons_path.MakeIRODSReleasePath(mdRepoTicket.IRODSDataPath)
	targetPath = commons_path.MakeLocalPath(targetPath)

	logger.Debugf("download %q to %q (ticket: %q)", sourcePath, targetPath, mdRepoTicket.IRODSTicket)

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceEntry.IsDir() {
		// dir
		// save content to target path without creating a subdir for the source dir
		return get.getDir(mdRepoTicket, sourceEntry, targetPath)
	}

	// file
	targetPath = commons_path.MakeLocalTargetFilePath(sourcePath, targetPath)
	return get.getFile(mdRepoTicket, sourceEntry, "", targetPath)
}

func (get *GetCommand) scheduleGet(mdRepoTicket *mdrepo.MDRepoTicket, sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) {
	logger := log.WithFields(log.Fields{
		"irods_data_path": mdRepoTicket.IRODSDataPath,
		"irods_ticket":    mdRepoTicket.IRODSTicket,
		"source_path":     sourceEntry.Path,
		"target_path":     targetPath,
	})

	defaultNotes := []string{"get"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodGet,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	reportTransfer := func(result *irodsclient_fs.FileTransferResult, err error, additionalNotes ...string) {
		newNotes := append(defaultNotes, additionalNotes...)

		get.transferReportManager.AddTransfer(result, transfer.TransferMethodGet, err, newNotes)
	}

	transferMode, threadsRequired := get.determineTransferMethod(sourceEntry.Size)

	getTask := func(job *parallel.ParallelJob) error {
		if job.IsCanceled() {
			// job is canceled, do not run
			job.Progress("download", -1, sourceEntry.Size, true)

			reportSimple(nil, "canceled")
			logger.Debug("canceled a task for downloading a data object")
			return nil
		}

		logger.Debug("downloading a data object")

		progressCallbackGet := func(taskType string, processed int64, total int64) {
			job.Progress(taskType, processed, total, false)
		}

		job.Progress("download", 0, sourceEntry.Size, false)

		downloadPath := targetPath
		if len(tempPath) > 0 {
			downloadPath = tempPath
		}

		parentDownloadPath := filepath.Dir(downloadPath)
		_, statErr := os.Stat(parentDownloadPath)
		if statErr != nil {
			// must exist, mkdir is performed at getDir
			job.Progress("download", -1, sourceEntry.Size, true)

			reportSimple(statErr)
			return errors.Wrapf(statErr, "failed to stat %q", parentDownloadPath)
		}

		notes := []string{string(transferMode), fmt.Sprintf("%d threads", threadsRequired)}

		var downloadErr error
		var downloadResult *irodsclient_fs.FileTransferResult

		retryNum := get.retryFlagValues.GetRetryNumber()
		retryInterval := get.retryFlagValues.GetRetryIntervalSeconds()

		attempt := 0
		retryErr := retry.Do(func() error {
			attempt++
			if attempt > 1 {
				logger.Debugf("retrying download attempt %d/%d for %q", attempt, retryNum+1, sourceEntry.Path)
			}

			switch transferMode {
			case transfer.TransferModeWebDAV:
				downloadResult, downloadErr = get.webdavClient.DownloadFile(sourceEntry, downloadPath, "", true, progressCallbackGet)
				notes = append(notes, "webdav")
			case transfer.TransferModeICAT:
				fallthrough
			default:
				downloadResult, downloadErr = get.filesystem.DownloadFileParallelResumable(sourceEntry.Path, "", downloadPath, threadsRequired, true, progressCallbackGet)
				notes = append(notes, "icat", fmt.Sprintf("%d threads", threadsRequired))
			}
			return downloadErr
		}, retry.Attempts(uint(retryNum+1)), retry.Delay(retryInterval), retry.LastErrorOnly(true))

		if retryErr != nil {
			job.Progress("download", -1, sourceEntry.Size, true)
			job.Progress("checksum", -1, sourceEntry.Size, true)

			reportTransfer(downloadResult, retryErr, notes...)
			return errors.Wrapf(retryErr, "failed to download %q to %q after %d attempts", sourceEntry.Path, targetPath, retryNum+1)
		}

		get.totalDownloadedFiles++
		get.totalDownloadedBytes += sourceEntry.Size

		reportTransfer(downloadResult, downloadErr, notes...)

		logger.Debugf("downloaded a data object %q to %q", sourceEntry.Path, targetPath)

		return nil
	}

	get.parallelTransferJobManager.Schedule(sourceEntry.Path, getTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a data object download %q to %q, %d threads", sourceEntry.Path, targetPath, threadsRequired)
}

func (get *GetCommand) getFile(mdRepoTicket *mdrepo.MDRepoTicket, sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"irods_data_path": mdRepoTicket.IRODSDataPath,
		"irods_ticket":    mdRepoTicket.IRODSTicket,
		"source_path":     sourceEntry.Path,
		"temp_path":       tempPath,
		"target_path":     targetPath,
	})

	logger.Debug("download a data object")

	defaultNotes := []string{"get"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "file")

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodGet,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	reportOverwrite := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a file with new name
			get.scheduleGet(mdRepoTicket, sourceEntry, tempPath, targetPath)
			return nil
		}

		reportSimple(err)
		return errors.Wrapf(err, "failed to stat %q", targetPath)
	}

	// target exists
	// target must be a file
	if targetStat.IsDir() {
		notFileErr := types.NewNotFileError(targetPath)
		reportOverwrite(notFileErr, "directory")
		return notFileErr
	}

	// check transfer status file
	if get.hasTransferStatusFile(targetPath) {
		// incomplete file - resume downloading
		terminal.Printf("resume downloading a data object %q\n", targetPath)
		logger.Debug("resume downloading a data object")

		get.scheduleGet(mdRepoTicket, sourceEntry, tempPath, targetPath)
		return nil
	}

	if !get.forceFlagValues.Force {
		if targetStat.Size() == sourceEntry.Size {
			// compare hash
			if len(sourceEntry.CheckSum) > 0 {
				localChecksum, err := irodsclient_util.HashLocalFile(targetPath, string(sourceEntry.CheckSumAlgorithm), nil)
				if err != nil {
					reportSimple(err, "differential")
					return errors.Wrapf(err, "failed to get hash of %q", targetPath)
				}

				if bytes.Equal(sourceEntry.CheckSum, localChecksum) {
					// skip
					now := time.Now()
					reportFile := &transfer.TransferReportFile{
						Method:                  transfer.TransferMethodGet,
						StartAt:                 now,
						EndAt:                   now,
						SourcePath:              sourceEntry.Path,
						SourceSize:              sourceEntry.Size,
						SourceChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
						SourceChecksum:          hex.EncodeToString(sourceEntry.CheckSum),
						DestPath:                targetPath,
						DestSize:                targetStat.Size(),
						DestChecksum:            hex.EncodeToString(localChecksum),
						DestChecksumAlgorithm:   string(sourceEntry.CheckSumAlgorithm),

						Notes: []string{"get", "file", "differential", "same checksum", "skipped"},
					}

					get.transferReportManager.AddFile(reportFile)

					terminal.Printf("skip downloading a data object %q to %q. The file with the same hash already exists!\n", sourceEntry.Path, targetPath)
					logger.Debug("skip downloading a data object. The file with the same hash already exists!")
					return nil
				}
			}
		}
	}

	// schedule
	get.scheduleGet(mdRepoTicket, sourceEntry, tempPath, targetPath)
	return nil
}

func (get *GetCommand) getDir(mdRepoTicket *mdrepo.MDRepoTicket, sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"irods_data_path": mdRepoTicket.IRODSDataPath,
		"irods_ticket":    mdRepoTicket.IRODSTicket,
		"source_path":     sourceEntry.Path,
		"target_path":     targetPath,
	})

	logger.Debug("download a collection")

	defaultNotes := []string{"get", "directory"}

	reportSimple := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)

		reportFile := &transfer.TransferReportFile{
			Method:     transfer.TransferMethodGet,
			StartAt:    now,
			EndAt:      now,
			SourcePath: sourceEntry.Path,
			SourceSize: sourceEntry.Size,
			DestPath:   targetPath,
			Error:      err,
			Notes:      newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	reportOverwrite := func(err error, additionalNotes ...string) {
		now := time.Now()
		newNotes := append(defaultNotes, additionalNotes...)
		newNotes = append(newNotes, "overwrite")

		reportFile := &transfer.TransferReportFile{
			Method:   transfer.TransferMethodDelete,
			StartAt:  now,
			EndAt:    now,
			DestPath: targetPath,
			Error:    err,
			Notes:    newNotes,
		}

		get.transferReportManager.AddFile(reportFile)
	}

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a directorywith new name
			err = os.MkdirAll(targetPath, 0766)
			reportSimple(err)
			if err != nil {
				return errors.Wrapf(err, "failed to make a directory %q", targetPath)
			}

			// fallthrough to get entries
		} else {
			reportSimple(err)
			return errors.Wrapf(err, "failed to stat %q", targetPath)
		}
	} else {
		// target exists
		if !targetStat.IsDir() {
			notDirErr := types.NewNotDirError(targetPath)
			reportOverwrite(notDirErr)
			return notDirErr
		}
	}

	// get entries
	entries, err := get.filesystem.List(sourceEntry.Path)
	if err != nil {
		reportSimple(err)
		return errors.Wrapf(err, "failed to list a directory %q", sourceEntry.Path)
	}

	for _, entry := range entries {
		newEntryPath := commons_path.MakeLocalTargetFilePath(entry.Path, targetPath)

		if entry.IsDir() {
			// dir
			err = get.getDir(mdRepoTicket, entry, newEntryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			err = get.getFile(mdRepoTicket, entry, "", newEntryPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (get *GetCommand) determineTransferMethod(size int64) (transfer.TransferMode, int) {
	logger := log.WithFields(log.Fields{})

	threads := parallel.CalculateThreadForTransferJob(size, get.parallelTransferFlagValues.ThreadNumberPerFile)

	// determine how to download
	if get.parallelTransferFlagValues.SingleThread || get.parallelTransferFlagValues.ThreadNumber <= 2 || get.parallelTransferFlagValues.ThreadNumberPerFile == 1 {
		threads = 1
	}

	if get.parallelTransferFlagValues.Icat {
		logger.Info("using ICAT transfer for downloading a data object")
		return transfer.TransferModeICAT, threads
	} else if get.parallelTransferFlagValues.WebDAV {
		if get.webdavClient == nil {
			// fallback
			logger.Info("WebDAV is not configured. Using ICAT transfer for downloading a data object")
			return transfer.TransferModeICAT, threads
		}

		logger.Info("using WebDAV for downloading a data object")
		return transfer.TransferModeWebDAV, 1
	}

	if get.webdavClient == nil {
		// fallback
		logger.Info("WebDAV is not configured. Using ICAT transfer for downloading a data object")
		return transfer.TransferModeICAT, threads
	}

	logger.Info("using WebDAV for downloading a data object")
	return transfer.TransferModeWebDAV, 1
}
