package subcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MD-Repo/md-repo-cli/cmd/flag"
	"github.com/MD-Repo/md-repo-cli/commons"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var submitListCmd = &cobra.Command{
	Use:     "submitls",
	Short:   "List MD-Repo submission data",
	Aliases: []string{"submit_ls", "list_submission", "list_submit"},
	RunE:    processSubmitListCommand,
	Args:    cobra.NoArgs,
}

func AddSubmitListCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitListCmd)

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

	tokenFlagValues          *flag.TokenFlagValues
	submissionListFlagValues *flag.SubmissionListFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem
}

func NewSubmitListCommand(command *cobra.Command, args []string) (*SubmitListCommand, error) {
	submitls := &SubmitListCommand{
		command: command,

		tokenFlagValues:          flag.GetTokenFlagValues(),
		submissionListFlagValues: flag.GetSubmissionListFlagValues(),
	}

	return submitls, nil
}

func (submitls *SubmitListCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SubmitListCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(submitls.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	config := commons.GetConfig()

	// handle token
	if len(submitls.tokenFlagValues.TicketString) > 0 {
		config.TicketString = submitls.tokenFlagValues.TicketString
	}

	if len(submitls.tokenFlagValues.Token) > 0 {
		config.Token = submitls.tokenFlagValues.Token
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	if len(config.Token) > 0 && len(config.TicketString) == 0 {
		// orcID
		// override ORC-ID
		orcID := ""
		if len(submitls.submissionListFlagValues.OrcID) > 0 {
			orcID = submitls.submissionListFlagValues.OrcID
		} else {
			orcID = commons.InputOrcID()
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

		config.TicketString, err = commons.GetMDRepoTicketStringFromToken(submitls.tokenFlagValues.ServiceURL, newToken)
		if err != nil {
			return xerrors.Errorf("failed to read ticket from token: %w", err)
		}
	}

	if len(config.TicketString) == 0 {
		return commons.TokenNotProvidedError
	}

	// get ticket
	mdRepoTicket, err := commons.GetMDRepoTicketFromString(config.TicketString)
	if err != nil {
		return xerrors.Errorf("failed to retrieve ticket: %w", err)
	}

	// Create a file system
	submitls.account, err = commons.GetAccount(&mdRepoTicket)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS Account: %w", err)
	}

	submitls.filesystem, err = commons.GetIRODSFSClient(submitls.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer submitls.filesystem.Release()

	// run
	sourcePath := commons.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

	logger.Debugf("list submission %q  (ticket: %q)", sourcePath, mdRepoTicket.IRODSTicket)

	err = submitls.listOne(sourcePath, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to list %q: %w", sourcePath, err)
	}

	return nil
}

func (submitls *SubmitListCommand) listOne(sourceRootPath string, sourcePath string) error {
	connection, err := submitls.filesystem.GetMetadataConnection(true)
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer submitls.filesystem.ReturnMetadataConnection(connection)

	// collection
	colls, err := irodsclient_irodsfs.ListSubCollections(connection, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to list sub-collections in %q: %w", sourcePath, err)
	}

	objs, err := irodsclient_irodsfs.ListDataObjects(connection, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to list data-objects in %q: %w", sourcePath, err)
	}

	// print text
	commons.Printf("[%s]\n", submitls.getDataPath(sourceRootPath, sourcePath))
	submitls.printTextGridHead()
	submitls.printDataObjects(objs)
	submitls.printCollections(colls)

	// call recursively
	for _, coll := range colls {
		fmt.Printf("\n")
		err = submitls.listOne(sourceRootPath, coll.Path)
		if err != nil {
			return xerrors.Errorf("failed to list %q: %w", coll.Path, err)
		}
	}

	if sourceRootPath == sourcePath {
		for _, obj := range objs {
			if commons.IsStatusFile(obj.Name) {
				commons.Printf("\n")
				err = submitls.catStatusFile(obj.Path)
				if err != nil {
					return xerrors.Errorf("failed to cat status file %q: %w", obj.Path, err)
				}
				break
			}
		}
	}

	return nil
}

func (submitls *SubmitListCommand) catStatusFile(sourcePath string) error {
	buffer := bytes.Buffer{}

	_, err := submitls.filesystem.DownloadFileToBuffer(sourcePath, "", &buffer, true, nil)
	if err != nil {
		return xerrors.Errorf("failed to download file %q: %w", sourcePath, err)
	}

	fmt.Printf("[SUBMISSION STATUS INFO]\n")

	jsonStr := submitls.getPrettyStatusFileJSON(buffer.Bytes())
	fmt.Printf("%s\n\n", string(jsonStr))
	return nil
}

func (submitls *SubmitListCommand) getPrettyStatusFileJSON(jsonBytes []byte) string {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "SubmitListCommand",
		"function": "getPrettyStatusFileJSON",
	})

	prettyJson := string(jsonBytes)

	status := commons.SubmitStatusFile{}
	err := json.Unmarshal(jsonBytes, &status)
	if err != nil {
		xerr := xerrors.Errorf("failed to decode json: %w", err)
		logger.Error(xerr)
		return prettyJson
	}

	jsonStr, err := json.MarshalIndent(status, "", "    ")
	if err != nil {
		xerr := xerrors.Errorf("failed to marshal to json: %w", err)
		logger.Error(xerr)
		return prettyJson
	}

	prettyJson = string(jsonStr)
	return prettyJson
}

func (submitls *SubmitListCommand) getDataPath(sourceRootPath string, sourcePath string) string {
	rel, err := filepath.Rel(sourceRootPath, sourcePath)
	if err != nil {
		return sourcePath
	}

	if rel == "." {
		return "/"
	}

	if strings.HasPrefix(rel, "./") {
		return rel[1:]
	}

	if rel[0] != '/' {
		return fmt.Sprintf("/%s", rel)
	}

	return rel
}

func (submitls *SubmitListCommand) printCollections(entries []*irodsclient_types.IRODSCollection) {
	sort.SliceStable(entries, submitls.getCollectionSortFunction(entries))
	for _, entry := range entries {
		submitls.printTextGridRow(true, entry.Name, "-", "", entry.ModifyTime)
	}
}

func (submitls *SubmitListCommand) printDataObjects(entries []*irodsclient_types.IRODSDataObject) {
	sort.SliceStable(entries, submitls.getDataObjectSortFunction(entries))
	for _, entry := range entries {
		submitls.printDataObject(entry)
	}
}

func (submitls *SubmitListCommand) printDataObject(entry *irodsclient_types.IRODSDataObject) {
	if len(entry.Replicas) > 0 {
		replica := entry.Replicas[0]

		checksum := ""
		if replica.Checksum != nil {
			checksum = replica.Checksum.IRODSChecksumString
		}

		submitls.printTextGridRow(false, entry.Name, fmt.Sprintf("%d", entry.Size), checksum, replica.ModifyTime)
	}
}

func (submitls *SubmitListCommand) printTextGridHead() {
	submitls.printTextGridRowInternal("TYPE", "NAME", "SIZE", "CHECKSUM", "LAST_MODIFIED")
}

func (submitls *SubmitListCommand) printTextGridRow(isDir bool, name string, size string, checksum string, lastmodified time.Time) {
	typeStr := "File"
	if isDir {
		typeStr = "Dir"
	}

	modTime := commons.MakeDateTimeStringHM(lastmodified)
	submitls.printTextGridRowInternal(typeStr, name, size, checksum, modTime)
}

func (submitls *SubmitListCommand) printTextGridRowInternal(typeStr string, name string, size string, checksum string, lastmodified string) {

	commons.Printf("%s\t%-50s\t%-12s\t%-32s\t%s\n", typeStr, name, size, checksum, lastmodified)
}

func (submitls *SubmitListCommand) getDataObjectSortFunction(entries []*irodsclient_types.IRODSDataObject) func(i int, j int) bool {
	return func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	}
}

func (submitls *SubmitListCommand) getCollectionSortFunction(entries []*irodsclient_types.IRODSCollection) func(i int, j int) bool {
	return func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	}
}
