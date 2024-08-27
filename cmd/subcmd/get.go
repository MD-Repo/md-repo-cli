package subcmd

import (
	"bytes"
	"encoding/hex"
	"os"
	"time"

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
	flag.SetParallelTransferFlags(getCmd, false)
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

	tokenFlagValues            *flag.TokenFlagValues
	forceFlagValues            *flag.ForceFlagValues
	parallelTransferFlagValues *flag.ParallelTransferFlagValues
	progressFlagValues         *flag.ProgressFlagValues
	retryFlagValues            *flag.RetryFlagValues
	transferReportFlagValues   *flag.TransferReportFlagValues

	maxConnectionNum int

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPath string

	parallelJobManager    *commons.ParallelJobManager
	transferReportManager *commons.TransferReportManager
	updatedPathMap        map[string]bool
}

func NewGetCommand(command *cobra.Command, args []string) (*GetCommand, error) {
	get := &GetCommand{
		command: command,

		tokenFlagValues:            flag.GetTokenFlagValues(),
		forceFlagValues:            flag.GetForceFlagValues(),
		parallelTransferFlagValues: flag.GetParallelTransferFlagValues(),
		progressFlagValues:         flag.GetProgressFlagValues(),
		retryFlagValues:            flag.GetRetryFlagValues(),
		transferReportFlagValues:   flag.GetTransferReportFlagValues(command),

		updatedPathMap: map[string]bool{},
	}

	get.maxConnectionNum = get.parallelTransferFlagValues.ThreadNumber + 2 // 2 for metadata op

	// path
	get.targetPath = "./"

	if len(args) > 0 {
		get.targetPath = args[0]
	}

	return get, nil
}

func (get *GetCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(get.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	config := commons.GetConfig()

	// handle token
	if len(get.tokenFlagValues.TicketString) > 0 {
		config.TicketString = get.tokenFlagValues.TicketString
	}

	if len(get.tokenFlagValues.Token) > 0 {
		config.Token = get.tokenFlagValues.Token
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	if len(config.Token) > 0 && len(config.TicketString) == 0 {
		config.TicketString, err = commons.GetMDRepoTicketStringFromToken(get.tokenFlagValues.ServiceURL, config.Token)
		if err != nil {
			return xerrors.Errorf("failed to read ticket from token: %w", err)
		}
	}

	if len(config.TicketString) == 0 {
		return commons.TokenNotProvidedError
	}

	// handle retry
	if get.retryFlagValues.RetryNumber > 0 && !get.retryFlagValues.RetryChild {
		err = commons.RunWithRetry(get.retryFlagValues.RetryNumber, get.retryFlagValues.RetryIntervalSeconds)
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", get.retryFlagValues.RetryNumber, err)
		}
		return nil
	}

	// get ticket
	mdRepoTickets, err := commons.GetMDRepoTicketsFromString(config.TicketString)
	if err != nil {
		return xerrors.Errorf("failed to retrieve tickets: %w", err)
	}

	// transfer report
	get.transferReportManager, err = commons.NewTransferReportManager(get.transferReportFlagValues.Report, get.transferReportFlagValues.ReportPath, get.transferReportFlagValues.ReportToStdout)
	if err != nil {
		return xerrors.Errorf("failed to create transfer report manager: %w", err)
	}
	defer get.transferReportManager.Release()

	// run
	if len(mdRepoTickets) >= 2 {
		// multi-source, target must be a dir
		err = get.ensureTargetIsDir(get.targetPath)
		if err != nil {
			return err
		}
	}

	// we may further optimize this by run it parallel
	for _, mdRepoTicket := range mdRepoTickets {
		// we create filesystem, job manager for every ticket as they require separate auth
		// Create a file system
		get.account, err = commons.GetAccount(&mdRepoTicket)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS Account: %w", err)
		}

		get.filesystem, err = commons.GetIRODSFSClientAdvanced(get.account, get.maxConnectionNum, get.parallelTransferFlagValues.TCPBufferSize)
		if err != nil {
			return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
		}
		defer get.filesystem.Release()

		// parallel job manager
		get.parallelJobManager = commons.NewParallelJobManager(get.filesystem, get.parallelTransferFlagValues.ThreadNumber, !get.progressFlagValues.NoProgress, false)
		get.parallelJobManager.Start()

		// run
		err = get.getOne(mdRepoTicket, get.targetPath)
		if err != nil {
			return xerrors.Errorf("failed to get %q to %q: %w", mdRepoTicket.IRODSDataPath, get.targetPath, err)
		}

		// release parallel job manager
		get.parallelJobManager.DoneScheduling()
		err = get.parallelJobManager.Wait()
		if err != nil {
			get.filesystem.Release()
			return xerrors.Errorf("failed to perform parallel jobs: %w", err)
		}
	}
	return nil
}

func (get *GetCommand) ensureTargetIsDir(targetPath string) error {
	targetPath = commons.MakeLocalPath(targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// not exist
			return commons.NewNotDirError(targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	if !targetStat.IsDir() {
		return commons.NewNotDirError(targetPath)
	}

	return nil
}

func (get *GetCommand) getOne(mdRepoTicket commons.MDRepoTicket, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "getOne",
	})

	// run
	sourcePath := commons.MakeIRODSReleasePath(mdRepoTicket.IRODSDataPath)
	targetPath = commons.MakeLocalPath(targetPath)

	logger.Debugf("download %q to %q (ticket: %q)", sourcePath, targetPath, mdRepoTicket.IRODSTicket)

	sourceEntry, err := get.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		// dir
		targetPath = commons.MakeTargetLocalFilePath(sourcePath, targetPath)
		return get.getDir(sourceEntry, targetPath)
	}

	// file
	targetPath = commons.MakeTargetLocalFilePath(sourcePath, targetPath)
	return get.getFile(sourceEntry, "", targetPath)

}

func (get *GetCommand) scheduleGet(sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "scheduleGet",
	})

	getTask := func(job *commons.ParallelJob) error {
		manager := job.GetManager()
		fs := manager.GetFilesystem()

		callbackGet := func(processed int64, total int64) {
			job.Progress(processed, total, false)
		}

		job.Progress(0, sourceEntry.Size, false)

		logger.Debugf("downloading a data object %q to %q", sourceEntry.Path, targetPath)

		var downloadErr error
		var downloadResult *irodsclient_fs.FileTransferResult
		notes := []string{}

		downloadPath := targetPath
		if len(tempPath) > 0 {
			downloadPath = tempPath
		}

		// determine how to download
		if get.parallelTransferFlagValues.SingleTread || get.parallelTransferFlagValues.ThreadNumber == 1 {
			downloadResult, downloadErr = fs.DownloadFile(sourceEntry.Path, "", downloadPath, true, callbackGet)
			notes = append(notes, "icat", "single-thread")
		} else if get.parallelTransferFlagValues.RedirectToResource {
			downloadResult, downloadErr = fs.DownloadFileRedirectToResource(sourceEntry.Path, "", downloadPath, 0, true, callbackGet)
			notes = append(notes, "redirect-to-resource")
		} else if get.parallelTransferFlagValues.Icat {
			downloadResult, downloadErr = fs.DownloadFileParallel(sourceEntry.Path, "", downloadPath, 0, true, callbackGet)
			notes = append(notes, "icat", "multi-thread")
		} else {
			// auto
			if sourceEntry.Size >= commons.RedirectToResourceMinSize {
				// redirect-to-resource
				downloadResult, downloadErr = fs.DownloadFileRedirectToResource(sourceEntry.Path, "", downloadPath, 0, true, callbackGet)
				notes = append(notes, "redirect-to-resource")
			} else {
				downloadResult, downloadErr = fs.DownloadFileParallel(sourceEntry.Path, "", downloadPath, 0, true, callbackGet)
				notes = append(notes, "icat", "multi-thread")
			}
		}

		if downloadErr != nil {
			job.Progress(-1, sourceEntry.Size, true)
			return xerrors.Errorf("failed to download %q to %q: %w", sourceEntry.Path, targetPath, downloadErr)
		}

		err := get.transferReportManager.AddTransfer(downloadResult, commons.TransferMethodGet, downloadErr, notes)
		if err != nil {
			job.Progress(-1, sourceEntry.Size, true)
			return xerrors.Errorf("failed to add transfer report: %w", err)
		}

		logger.Debugf("downloaded a data object %q to %q", sourceEntry.Path, targetPath)
		job.Progress(sourceEntry.Size, sourceEntry.Size, false)

		job.Done()
		return nil
	}

	threadsRequired := irodsclient_util.GetNumTasksForParallelTransfer(sourceEntry.Size)
	err := get.parallelJobManager.Schedule(sourceEntry.Path, getTask, threadsRequired, progress.UnitsBytes)
	if err != nil {
		return xerrors.Errorf("failed to schedule download %q to %q: %w", sourceEntry.Path, targetPath, err)
	}

	logger.Debugf("scheduled a data object download %q to %q", sourceEntry.Path, targetPath)

	return nil
}

func (get *GetCommand) getFile(sourceEntry *irodsclient_fs.Entry, tempPath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "GetCommand",
		"function": "getFile",
	})

	commons.MarkPathMap(get.updatedPathMap, targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a file with new name
			return get.scheduleGet(sourceEntry, tempPath, targetPath)
		}

		return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
	}

	// target exists
	// target must be a file
	if targetStat.IsDir() {
		return commons.NewNotFileError(targetPath)
	}

	if !get.forceFlagValues.Force {
		if targetStat.Size() == sourceEntry.Size {
			// compare hash
			if len(sourceEntry.CheckSum) > 0 {
				localChecksum, err := irodsclient_util.HashLocalFile(targetPath, string(sourceEntry.CheckSumAlgorithm))
				if err != nil {
					return xerrors.Errorf("failed to get hash of %q: %w", targetPath, err)
				}

				if bytes.Equal(sourceEntry.CheckSum, localChecksum) {
					// skip
					now := time.Now()
					reportFile := &commons.TransferReportFile{
						Method:            commons.TransferMethodGet,
						StartAt:           now,
						EndAt:             now,
						SourcePath:        sourceEntry.Path,
						SourceSize:        sourceEntry.Size,
						SourceChecksum:    hex.EncodeToString(sourceEntry.CheckSum),
						DestPath:          targetPath,
						DestSize:          targetStat.Size(),
						DestChecksum:      hex.EncodeToString(localChecksum),
						ChecksumAlgorithm: string(sourceEntry.CheckSumAlgorithm),
						Notes:             []string{"differential", "same checksum", "skip"},
					}

					get.transferReportManager.AddFile(reportFile)

					commons.Printf("skip downloading a data object %q to %q. The file with the same hash already exists!\n", sourceEntry.Path, targetPath)
					logger.Debugf("skip downloading a data object %q to %q. The file with the same hash already exists!", sourceEntry.Path, targetPath)
					return nil
				}
			}
		}
	}

	// schedule
	return get.scheduleGet(sourceEntry, tempPath, targetPath)
}

func (get *GetCommand) getDir(sourceEntry *irodsclient_fs.Entry, targetPath string) error {
	commons.MarkPathMap(get.updatedPathMap, targetPath)

	targetStat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// target does not exist
			// target must be a directorywith new name
			err = os.MkdirAll(targetPath, 0766)
			if err != nil {
				return xerrors.Errorf("failed to make a directory %q: %w", targetPath, err)
			}

			now := time.Now()
			reportFile := &commons.TransferReportFile{
				Method:     commons.TransferMethodGet,
				StartAt:    now,
				EndAt:      now,
				SourcePath: sourceEntry.Path,
				DestPath:   targetPath,
				Notes:      []string{"directory"},
			}

			get.transferReportManager.AddFile(reportFile)
		} else {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		// target exists
		if !targetStat.IsDir() {
			return commons.NewNotDirError(targetPath)
		}
	}

	// get entries
	entries, err := get.filesystem.List(sourceEntry.Path)
	if err != nil {
		return xerrors.Errorf("failed to list a directory %q: %w", sourceEntry.Path, err)
	}

	for _, entry := range entries {
		newEntryPath := commons.MakeTargetLocalFilePath(entry.Path, targetPath)

		if entry.IsDir() {
			// dir
			err = get.getDir(entry, newEntryPath)
			if err != nil {
				return err
			}
		} else {
			// file
			err = get.getFile(entry, "", newEntryPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
