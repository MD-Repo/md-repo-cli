package subcmd

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go"
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_session "github.com/cyverse/go-irodsclient/irods/session"
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
	flag.SetJobsFlags(getCmd)
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
	jobsFlagValues             *flag.JobsFlagValues
	forceFlagValues            *flag.ForceFlagValues
	progressFlagValues         *flag.ProgressFlagValues
	retryFlagValues            *flag.RetryFlagValues
	transferReportFlagValues   *flag.TransferReportFlagValues

	targetPath string

	// set when several jobs run at once: their progress bars and messages
	// all go to this one display
	sharedProgressWriter progress.Writer

	transferReportManager *transfer.TransferReportManager
	config                *config.Config

	totalDownloadedFiles int64
	totalDownloadedBytes int64
	startTime            time.Time
}

func NewGetCommand(command *cobra.Command, args []string) (*GetCommand, error) {
	parallelTransferFlagValues, err := flag.GetParallelTransferFlagValues()
	if err != nil {
		return nil, errors.Wrap(err, "invalid parallel transfer flag")
	}
	jobsFlagValues, err := flag.GetJobsFlagValues()
	if err != nil {
		return nil, err
	}
	get := &GetCommand{
		command: command,

		commonFlagValues:           flag.GetCommonFlagValues(command),
		tokenFlagValues:            flag.GetTokenFlagValues(),
		parallelTransferFlagValues: parallelTransferFlagValues,
		jobsFlagValues:             jobsFlagValues,
		forceFlagValues:            flag.GetForceFlagValues(),
		progressFlagValues:         flag.GetProgressFlagValues(),
		retryFlagValues:            flag.GetRetryFlagValues(),
		transferReportFlagValues:   flag.GetTransferReportFlagValues(command),

		config:               config.GetConfig(),
		totalDownloadedFiles: 0,
		totalDownloadedBytes: 0,
		startTime:            time.Now(),
	}

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
		return errors.Wrap(err, "failed to process common flags")
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
		return errors.Wrap(err, "failed to input missing fields")
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
		return errors.Wrap(err, "failed to create transfer report manager")
	}
	defer get.transferReportManager.Release()

	// run
	err = get.ensureTargetIsDir(get.targetPath)
	if err != nil {
		return errors.Wrapf(err, "target path %q is not a directory", get.targetPath)
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

	err = get.runTicketGroups(ticketGroups, ticketGroupOrder)
	if err != nil {
		return err
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

// getWorker downloads ticket groups (one per simulation) one after another
// with its own iRODS client. With --jobs N, N workers run at once and share
// the --thread_num connections.
type getWorker struct {
	*GetCommand

	maxConnectionNum int

	account      *irodsclient_types.IRODSAccount
	filesystem   *irodsclient_fs.FileSystem
	webdavClient *webdav.WebDAVClient

	parallelTransferJobManager *parallel.ParallelJobManager
}

// runTicketGroups downloads every ticket group, --jobs groups at once.
func (get *GetCommand) runTicketGroups(ticketGroups map[string][]mdrepo.MDRepoTicket, ticketGroupOrder []string) error {
	if len(ticketGroupOrder) == 0 {
		return nil
	}

	connections := splitConnections(get.parallelTransferFlagValues.ThreadNumber, min(get.jobsFlagValues.Jobs, len(ticketGroupOrder)))
	if len(connections) < get.jobsFlagValues.Jobs && len(connections) < len(ticketGroupOrder) {
		terminal.Printf("running %d jobs, not %d: each job needs one of the %d connections (--thread_num)\n", len(connections), get.jobsFlagValues.Jobs, get.parallelTransferFlagValues.ThreadNumber)
	}

	workers := make([]*getWorker, len(connections))
	for i, conns := range connections {
		workers[i] = &getWorker{
			GetCommand:       get,
			maxConnectionNum: conns,
		}
	}

	defer func() {
		for _, worker := range workers {
			worker.release()
		}
	}()

	if len(workers) > 1 {
		terminal.Printf("downloading %d simulations, %d at once\n", len(ticketGroupOrder), len(workers))

		if !get.progressFlagValues.NoProgress {
			get.sharedProgressWriter = terminal.GetProgressWriter(true)
			go get.sharedProgressWriter.Render()

			defer func() {
				get.sharedProgressWriter.Stop()
				for get.sharedProgressWriter.IsRenderInProgress() {
					time.Sleep(10 * time.Millisecond)
				}
				get.sharedProgressWriter = nil
			}()
		}
	}

	return runGroups(len(ticketGroupOrder), len(workers), func(workerIdx int, groupIdx int) error {
		return workers[workerIdx].processTicketGroup(ticketGroups[ticketGroupOrder[groupIdx]])
	})
}

// splitConnections shares total connections among jobs, as evenly as it can,
// and returns each job's share. There are never more jobs than connections,
// since a job needs at least one.
func splitConnections(total int, jobs int) []int {
	if total < 1 {
		total = 1
	}

	if jobs < 1 {
		jobs = 1
	}

	if jobs > total {
		jobs = total
	}

	shares := make([]int, jobs)
	for i := range shares {
		shares[i] = total / jobs
		if i < total%jobs {
			shares[i]++
		}
	}

	return shares
}

// runGroups calls run(worker, group) for groups 0..numGroups-1, with
// numWorkers workers each taking the next group when it finishes one, so a
// worker never runs two groups at once. After the first error no worker
// starts another group; the groups already running finish. It returns every
// error.
func runGroups(numGroups int, numWorkers int, run func(workerIdx int, groupIdx int) error) error {
	if numWorkers < 1 {
		numWorkers = 1
	}

	var next int64 = -1
	var failed atomic.Bool
	var errMutex sync.Mutex
	var errs []error

	wg := sync.WaitGroup{}
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerIdx int) {
			defer wg.Done()

			for !failed.Load() {
				groupIdx := int(atomic.AddInt64(&next, 1))
				if groupIdx >= numGroups {
					return
				}

				err := run(workerIdx, groupIdx)
				if err != nil {
					failed.Store(true)

					errMutex.Lock()
					errs = append(errs, err)
					errMutex.Unlock()
					return
				}
			}
		}(w)
	}

	wg.Wait()

	return errors.Join(errs...)
}

// printf prints a message, above the progress bars when jobs share them.
func (get *GetCommand) printf(format string, a ...any) {
	if get.sharedProgressWriter != nil {
		get.sharedProgressWriter.Log(strings.TrimRight(fmt.Sprintf(format, a...), "\n"))
		return
	}

	terminal.Printf(format, a...)
}

// release closes the worker's iRODS client, if it opened one.
func (get *getWorker) release() {
	if get.filesystem != nil {
		get.filesystem.Release()
		get.filesystem = nil
	}
}

// openFilesystem creates the iRODS client the worker uses for all its ticket
// groups, logged in with the first one's ticket. Every simulation has its own
// ticket, and logging in again for each one cost ~5 s per simulation
// (MDR-117), so later groups switch the ticket on the open connections.
func (get *getWorker) openFilesystem(firstTicket mdrepo.MDRepoTicket) error {
	account, err := firstTicket.GetAccount()
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS Account")
	}

	get.account = account

	get.filesystem, err = irods.GetIRODSFSClientForLargeFileIO(get.account, get.maxConnectionNum, get.parallelTransferFlagValues.TCPSendBufferSize, get.parallelTransferFlagValues.TCPRecvBufferSize, true, get.commonFlagValues.Timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}

	if get.parallelTransferFlagValues.WebDAV {
		webdavClient, err := webdav.NewWebDAVClient(get.filesystem, config.MDRepoWebDAVServerURL+config.MDRepoWebDAVPrefix, get.account.ProxyUser, get.account.Password)
		if err != nil {
			get.filesystem.Release()
			get.filesystem = nil
			return errors.Wrap(err, "failed to create WebDAV client")
		}

		get.webdavClient = webdavClient
	}

	return nil
}

// switchTicket makes the open iRODS client use another ticket. Connections
// opened from now on log in with it (the sessions share get.account), and the
// connections already open are given it with a "session" ticket request,
// which replaces the connection's previous ticket. Called between ticket
// groups, when no transfer is running.
func (get *getWorker) switchTicket(irodsTicket string) error {
	logger := log.WithFields(log.Fields{
		"ticket": irodsTicket,
	})

	get.account.Ticket = irodsTicket

	sessions := []*irodsclient_session.IRODSSession{
		get.filesystem.GetMetadataSession(),
		get.filesystem.GetIOSession(),
	}

	for _, sess := range sessions {
		idle := sess.GetIdleConnections()
		if idle == 0 {
			continue
		}

		conns, err := sess.AcquireConnectionsMulti(idle, false)
		for _, conn := range conns {
			if !conn.IsConnected() {
				// connected after the account change, so it already has the ticket
				_ = sess.ReturnConnection(conn)
				continue
			}

			supplyErr := irodsclient_irodsfs.SupplyTicket(conn, irodsTicket)
			if supplyErr != nil {
				// drop it rather than leave a connection with the old ticket
				logger.WithError(supplyErr).Debug("failed to switch ticket, discarding connection")
				sess.DiscardConnection(conn)
				continue
			}

			_ = sess.ReturnConnection(conn)
		}

		if err != nil && len(conns) == 0 {
			return errors.Wrapf(err, "failed to switch to iRODS ticket %q", irodsTicket)
		}
	}

	logger.Debug("switched ticket")
	return nil
}

func (get *getWorker) processTicketGroup(mdRepoTickets []mdrepo.MDRepoTicket) error {
	if len(mdRepoTickets) == 0 {
		return nil
	}

	// the worker's first group opens its client; later ones switch the ticket
	if get.filesystem == nil {
		err := get.openFilesystem(mdRepoTickets[0])
		if err != nil {
			return err
		}
	} else {
		err := get.switchTicket(mdRepoTickets[0].IRODSTicket)
		if err != nil {
			return err
		}
	}

	// parallel job manager - created once for the entire ticket group
	ioSession := get.filesystem.GetIOSession()
	get.parallelTransferJobManager = parallel.NewParallelJobManager(ioSession.GetMaxConnections(), !get.progressFlagValues.NoProgress, get.progressFlagValues.ShowFullPath, get.parallelTransferFlagValues.StopOnError)
	if get.sharedProgressWriter != nil {
		get.parallelTransferJobManager.SetProgressWriter(get.sharedProgressWriter)
	}

	// with several jobs these per-simulation lines would only be noise
	stepf := terminal.Printf
	if get.jobsFlagValues.Jobs > 1 {
		stepf = func(format string, a ...any) (int, error) {
			log.Debugf(format, a...)
			return 0, nil
		}
	}

	// schedule all paths in this ticket group
	stepf("scheduling transfer...\n")

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
	stepf("start transfer...\n")

	transferErr := get.parallelTransferJobManager.Start()
	if transferErr != nil {
		return errors.Wrap(transferErr, "failed to perform transfer jobs")
	}

	stepf("done transfer...\n")

	return nil
}

func (get *GetCommand) ensureTargetIsDir(targetPath string) error {
	targetPath = commons_path.MakeLocalPath(targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// create it, and any missing parents; the token has already been
			// checked, so a bad token never leaves an empty directory
			mkdirErr := os.MkdirAll(targetPath, 0755)
			if mkdirErr != nil {
				return errors.Wrapf(mkdirErr, "failed to create directory %q", targetPath)
			}

			terminal.Printf("Created directory %s\n", targetPath)
			return nil
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

func (get *getWorker) getOne(mdRepoTicket *mdrepo.MDRepoTicket, targetPath string) error {
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

func (get *getWorker) scheduleGet(mdRepoTicket *mdrepo.MDRepoTicket, sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) {
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
			case transfer.TransferModeRedirectToResource:
				downloadResult, downloadErr = get.filesystem.DownloadFileRedirectToResource(sourceEntry.Path, "", downloadPath, threadsRequired, true, progressCallbackGet)
				notes = append(notes, "redirect", fmt.Sprintf("%d threads", threadsRequired))
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

		get.addDownloaded(sourceEntry.Size)

		reportTransfer(downloadResult, nil, notes...)

		logger.Debugf("downloaded a data object %q to %q", sourceEntry.Path, targetPath)

		return nil
	}

	get.parallelTransferJobManager.Schedule(sourceEntry.Path, getTask, threadsRequired, progress.UnitsBytes)
	logger.Debugf("scheduled a data object download %q to %q, %d threads", sourceEntry.Path, targetPath, threadsRequired)
}

func (get *GetCommand) addDownloaded(bytes int64) {
	atomic.AddInt64(&get.totalDownloadedFiles, 1)
	atomic.AddInt64(&get.totalDownloadedBytes, bytes)
}

func (get *GetCommand) downloadedTotals() (int64, int64) {
	return atomic.LoadInt64(&get.totalDownloadedFiles), atomic.LoadInt64(&get.totalDownloadedBytes)
}

func (get *getWorker) getFile(mdRepoTicket *mdrepo.MDRepoTicket, sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) error {
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
		get.printf("resume downloading a data object %q\n", targetPath)
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

					get.printf("skip downloading a data object %q to %q. The file with the same hash already exists!\n", sourceEntry.Path, targetPath)
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

func (get *getWorker) getDir(mdRepoTicket *mdrepo.MDRepoTicket, sourceEntry *irodsclient_fs.Entry, targetPath string) error {
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

func (get *getWorker) determineTransferMethod(size int64) (transfer.TransferMode, int) {
	logger := log.WithFields(log.Fields{})

	// a file never uses more than this worker's share of the connections
	threads := parallel.CalculateThreadForTransferJob(size, min(get.parallelTransferFlagValues.ThreadNumberPerFile, get.maxConnectionNum))

	// determine how to download
	if get.parallelTransferFlagValues.SingleThread || get.maxConnectionNum <= 2 || get.parallelTransferFlagValues.ThreadNumberPerFile == 1 {
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
	} else if get.parallelTransferFlagValues.RedirectToResource {
		logger.Info("using resource server redirection for downloading a data object")
		return transfer.TransferModeRedirectToResource, threads
	}

	if get.webdavClient == nil {
		// fallback
		logger.Info("WebDAV is not configured. Using ICAT transfer for downloading a data object")
		return transfer.TransferModeICAT, threads
	}

	logger.Info("using WebDAV for downloading a data object")
	return transfer.TransferModeWebDAV, 1
}
