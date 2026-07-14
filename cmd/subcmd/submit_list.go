package subcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/MD-Repo/md-repo-cli/commons/checksum"
	"github.com/MD-Repo/md-repo-cli/commons/config"
	"github.com/MD-Repo/md-repo-cli/commons/format"
	"github.com/MD-Repo/md-repo-cli/commons/irods"
	"github.com/MD-Repo/md-repo-cli/commons/mdrepo"
	"github.com/MD-Repo/md-repo-cli/commons/path"
	commons_path "github.com/MD-Repo/md-repo-cli/commons/path"
	"github.com/MD-Repo/md-repo-cli/commons/terminal"
	"github.com/MD-Repo/md-repo-cli/commons/types"
	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var submitListCmd = &cobra.Command{
	Use:     "submitls",
	Short:   "List MD-Repo data",
	Long:    `This command lists MD-Repo submission data associated with the given token.`,
	Aliases: []string{"submit_ls", "list_submission", "list_submit"},
	RunE:    processSubmitListCommand,
	Args:    cobra.NoArgs,
}

func AddSubmitListCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitListCmd)
	flag.SetOutputFormatFlags(submitListCmd, false)
	flag.SetTokenFlags(submitListCmd)
	flag.SetSubmissionListFlags(submitListCmd)

	rootCmd.AddCommand(submitListCmd)
}

func processSubmitListCommand(command *cobra.Command, args []string) error {
	submit, err := NewSubmitListCommand(command, args)
	if err != nil {
		return err
	}

	return submit.Process()
}

type SubmitListCommand struct {
	command *cobra.Command

	commonFlagValues         *flag.CommonFlagValues
	outputFormatFlagValues   *flag.OutputFormatFlagValues
	tokenFlagValues          *flag.TokenFlagValues
	submissionListFlagValues *flag.SubmissionListFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
	config     *config.Config

	dataRootPath string
}

func NewSubmitListCommand(command *cobra.Command, args []string) (*SubmitListCommand, error) {
	submitls := &SubmitListCommand{
		command: command,

		commonFlagValues:         flag.GetCommonFlagValues(command),
		outputFormatFlagValues:   flag.GetOutputFormatFlagValues(),
		tokenFlagValues:          flag.GetTokenFlagValues(),
		submissionListFlagValues: flag.GetSubmissionListFlagValues(),

		config: config.GetConfig(),
	}

	return submitls, nil
}

func (submitls *SubmitListCommand) Process() error {
	fmt.Printf("submitls %s\n", commons.GetClientVersion())
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(submitls.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle token
	if len(submitls.tokenFlagValues.TicketString) > 0 {
		submitls.config.TicketString = submitls.tokenFlagValues.TicketString
	}

	if len(submitls.tokenFlagValues.Token) > 0 {
		submitls.config.Token = submitls.tokenFlagValues.Token
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	if len(submitls.config.Token) > 0 && len(submitls.config.TicketString) == 0 {
		// orcID
		// override ORC-ID
		orcID := ""
		if len(submitls.submissionListFlagValues.OrcID) > 0 {
			orcID = submitls.submissionListFlagValues.OrcID
		} else {
			orcID = terminal.Input("Input ORCID")
		}

		// encrypt
		tokenBytes, err := checksum.Base64Decode(submitls.config.Token)
		if err != nil {
			return errors.Wrapf(err, "failed to decode token using BASE64")
		}

		newToken, err := checksum.HMACStringSHA224(tokenBytes, orcID)
		if err != nil {
			return errors.Wrapf(err, "failed to encrypt token using SHA3-224 HMAC")
		}

		logger.Debugf("encrypted token: %s", newToken)

		submitls.config.TicketString, err = mdrepo.GetMDRepoTicketStringFromToken(submitls.tokenFlagValues.ServiceURL, newToken)
		if err != nil {
			return errors.Wrapf(err, "failed to read ticket from token %q", newToken)
		}
	}

	if len(submitls.config.TicketString) == 0 {
		return types.NewTokenNotProvidedError()
	}

	// get ticket
	mdRepoTicket, err := mdrepo.GetMDRepoTicketFromString(submitls.config.TicketString)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve ticket")
	}

	// Create a file system
	account, err := mdRepoTicket.GetAccount()
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS Account")
	}

	submitls.account = account

	submitls.filesystem, err = irods.GetIRODSFSClient(submitls.account, true, submitls.commonFlagValues.Timeout)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer submitls.filesystem.Release()

	// run
	outputFormatter := format.NewOutputFormatter(terminal.GetTerminalWriter())

	sourcePath := commons_path.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

	submitls.dataRootPath = sourcePath

	logger.Debugf("list submission %q (ticket: %q)", sourcePath, mdRepoTicket.IRODSTicket)

	err = submitls.listSourcePath(outputFormatter, sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to list path %q", sourcePath)
	}

	outputFormatter.Render(submitls.outputFormatFlagValues.Format)

	return nil
}

func (submitls *SubmitListCommand) listSourcePath(outputFormatter *format.OutputFormatter, sourcePath string) error {
	connection, err := submitls.filesystem.GetMetadataConnection(true)
	if err != nil {
		return errors.Wrapf(err, "failed to get connection")
	}
	defer submitls.filesystem.ReturnMetadataConnection(connection)

	err = submitls.printStatusFile(outputFormatter, sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to print status file")
	}

	err = submitls.listCollection(outputFormatter, sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to list collection %q", sourcePath)
	}

	return nil
}

func (submitls *SubmitListCommand) printStatusFile(outputFormatter *format.OutputFormatter, sourcePath string) error {
	logger := log.WithFields(log.Fields{})

	connection, err := submitls.filesystem.GetMetadataConnection(true)
	if err != nil {
		return errors.Wrapf(err, "failed to get connection")
	}
	defer submitls.filesystem.ReturnMetadataConnection(connection)

	objs, err := irodsclient_irodsfs.ListDataObjects(connection, sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to list data-objects in %q", sourcePath)
	}

	statusFilePath := ""

	// find status file
	var latestObjs *irodsclient_types.IRODSDataObject

	for _, obj := range objs {
		if !mdrepo.IsStatusFile(obj.Name) {
			continue
		}

		if len(obj.Replicas) == 0 {
			continue
		}

		if latestObjs == nil {
			latestObjs = obj
			continue
		}

		if obj.Replicas[0].ModifyTime.After(latestObjs.Replicas[0].ModifyTime) {
			latestObjs = obj
		}
	}

	if len(statusFilePath) == 0 {
		logger.Debugf("no status file found in %q", sourcePath)
		return nil
	}

	buffer := bytes.Buffer{}

	_, err = submitls.filesystem.DownloadFileToBuffer(statusFilePath, "", &buffer, false, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to download file %q", statusFilePath)
	}

	status := mdrepo.SubmitStatusFile{}
	err = json.Unmarshal(buffer.Bytes(), &status)
	if err != nil {
		return errors.Wrapf(err, "failed to decode json")
	}

	outputFormatterTable := outputFormatter.NewTable("Submission Status")

	outputFormatterTable.SetHeader([]string{
		"Total Files",
		"Total Size",
		"Token",
		"Status",
		"Time",
	})

	outputFormatterTable.AppendRow([]interface{}{
		fmt.Sprintf("%d", status.TotalFileNumber),
		types.SizeString(status.TotalFileSize),
		status.Token,
		status.Status,
		status.Time,
	})

	return nil
}

func (submitls *SubmitListCommand) listCollection(outputFormatter *format.OutputFormatter, sourcePath string) error {
	connection, err := submitls.filesystem.GetMetadataConnection(true)
	if err != nil {
		return errors.Wrapf(err, "failed to get connection")
	}
	defer submitls.filesystem.ReturnMetadataConnection(connection)

	outputFormatterTable := outputFormatter.NewTable("Content of " + sourcePath)

	// collection
	if submitls.outputFormatFlagValues.Format == format.OutputFormatLegacy {
		outputFormatterTable.SetHeader([]string{
			"Path",
		})

		outputFormatterTable.AppendRow([]interface{}{
			path.GetIRODSRelativePath(submitls.dataRootPath, sourcePath) + ":",
		})
	} else {
		outputFormatterTable.SetHeader([]string{
			"Type",
			"Path",
		})
		// outputFormatterTable.SetColumnWidthMax([]int{0, 50})

		outputFormatterTable.AppendRow([]interface{}{
			"collection",
			path.GetIRODSRelativePath(submitls.dataRootPath, sourcePath),
		})
	}

	// sub-collections and data-objects
	colls, err := irodsclient_irodsfs.ListSubCollections(connection, sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to list sub-collections in %q", sourcePath)
	}

	objs, err := irodsclient_irodsfs.ListDataObjects(connection, sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to list data-objects in %q", sourcePath)
	}

	submitls.printDataObjectsAndCollections(outputFormatter, sourcePath, objs, colls, false)

	// call recursively
	for _, coll := range colls {
		terminal.Printf("\n")
		err = submitls.listCollection(outputFormatter, coll.Path)
		if err != nil {
			return errors.Wrapf(err, "failed to list %q", coll.Path)
		}
	}

	return nil
}

func (submitls *SubmitListCommand) printDataObjectsAndCollections(outputFormatter *format.OutputFormatter, parentPath string, objectEntries []*irodsclient_types.IRODSDataObject, collectionEntries []*irodsclient_types.IRODSCollection, showFullPath bool) {
	title := "Submission Data"
	if parentPath != "" {
		title = fmt.Sprintf("Content of %s", parentPath)
	}

	outputFormatterTable := outputFormatter.NewTable(title)

	pathTitle := "Name"
	if showFullPath {
		pathTitle = "Path"
	}

	if submitls.outputFormatFlagValues.Format == format.OutputFormatLegacy {
		outputFormatterTable.SetHeader([]string{
			pathTitle,
		})
	} else {
		outputFormatterTable.SetHeader([]string{
			"Type",
			pathTitle,
			"Size",
			"Modify Time",
		})
		// outputFormatterTable.SetColumnWidthMax([]int{0, 50, 20})
	}

	sort.SliceStable(objectEntries, submitls.getDataObjectSortFunction(objectEntries))
	sort.SliceStable(collectionEntries, submitls.getCollectionSortFunction(collectionEntries))

	for _, entry := range objectEntries {
		newName := entry.Name
		if showFullPath {
			newName = entry.Path
		}

		size := fmt.Sprintf("%v", entry.Size)
		modifyTime := ""
		if len(entry.Replicas) > 0 {
			modifyTime = types.MakeDateTimeStringHM(entry.Replicas[0].ModifyTime)
		}

		if submitls.outputFormatFlagValues.Format == format.OutputFormatLegacy {
			outputFormatterTable.AppendRow([]interface{}{
				"  " + newName,
			})
		} else {
			outputFormatterTable.AppendRow([]interface{}{
				"data-object",
				newName,
				size,
				modifyTime,
			})
		}
	}

	for _, entry := range collectionEntries {
		newName := entry.Name
		if showFullPath {
			newName = entry.Path
		}

		modifyTime := types.MakeDateTimeStringHM(entry.ModifyTime)

		if submitls.outputFormatFlagValues.Format == format.OutputFormatLegacy {
			outputFormatterTable.AppendRow([]interface{}{
				"  C- " + entry.Path,
			})
		} else {
			outputFormatterTable.AppendRow([]interface{}{
				"collection",
				newName,
				"",
				modifyTime,
			})
		}
	}
}

func (submitls *SubmitListCommand) getDataObjectSortFunction(entries []*irodsclient_types.IRODSDataObject) func(i int, j int) bool {
	return func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	}
}

func (submitls *SubmitListCommand) getDataObjectModifyTime(object *irodsclient_types.IRODSDataObject) time.Time {
	// ModifyTime of data object is considered to be ModifyTime of replica modified most recently
	maxTime := object.Replicas[0].ModifyTime
	for _, t := range object.Replicas[1:] {
		if t.ModifyTime.After(maxTime) {
			maxTime = t.ModifyTime
		}
	}
	return maxTime
}

func (submitls *SubmitListCommand) getCollectionSortFunction(entries []*irodsclient_types.IRODSCollection) func(i int, j int) bool {
	return func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	}
}
