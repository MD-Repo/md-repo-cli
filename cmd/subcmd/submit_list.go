package subcmd

import (
	"encoding/json"
	"fmt"
	"io"
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
	Use:     "submitls [mdrepo_ticket]",
	Short:   "List MD-Repo submission data",
	Aliases: []string{"submit_ls"},
	RunE:    processSubmitListCommand,
	Args:    cobra.ExactArgs(1),
}

func AddPutListCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(submitListCmd)

	rootCmd.AddCommand(submitListCmd)
}

func processSubmitListCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processSubmitListCommand",
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

	ticket := strings.TrimSpace(args[0])

	mdRepoTicket, err := commons.GetConfig().GetMDRepoTicket(ticket)
	if err != nil {
		return xerrors.Errorf("failed to parse MD-Repo Ticket: %w", err)
	}

	// Create a file system
	account, err := commons.GetAccount(mdRepoTicket)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS Account: %w", err)
	}

	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := commons.MakeIRODSLandingPath(mdRepoTicket.IRODSDataPath)

	// display
	logger.Debugf("submission iRODS ticket: %s", mdRepoTicket.IRODSTicket)
	logger.Debugf("submission path: %s", targetPath)

	err = listOne(filesystem, targetPath, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to list %s: %w", targetPath, err)
	}

	return nil
}

func listOne(fs *irodsclient_fs.FileSystem, targetRootPath string, targetPath string) error {
	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	collection, err := irodsclient_irodsfs.GetCollection(connection, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to get collection %s: %w", targetPath, err)
	}

	colls, err := irodsclient_irodsfs.ListSubCollections(connection, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to list sub-collections in %s: %w", targetPath, err)
	}

	objs, err := irodsclient_irodsfs.ListDataObjects(connection, collection)
	if err != nil {
		return xerrors.Errorf("failed to list data-objects in %s: %w", targetPath, err)
	}

	// print text
	fmt.Printf("[%s]\n", getDataPath(targetRootPath, targetPath))
	printTextGridHead()
	printDataObjects(objs)
	printCollections(colls)

	// call recursively
	for _, coll := range colls {
		fmt.Printf("\n")
		err = listOne(fs, targetRootPath, coll.Path)
		if err != nil {
			return xerrors.Errorf("failed to list %s: %w", coll.Path, err)
		}
	}

	if targetRootPath == targetPath {
		for _, obj := range objs {
			if obj.Name == commons.GetMDRepoStatusFilename() {
				fmt.Printf("\n")
				err = catStatusFile(fs, obj.Path)
				if err != nil {
					return xerrors.Errorf("failed to cat status file %s: %w", obj.Path, err)
				}
				break
			}
		}
	}

	return nil
}

func catStatusFile(fs *irodsclient_fs.FileSystem, targetPath string) error {
	fh, err := fs.OpenFile(targetPath, "", "r")
	if err != nil {
		return xerrors.Errorf("failed to open file %s: %w", targetPath, err)
	}

	defer fh.Close()

	fmt.Printf("[SUBMISSION STATUS INFO]\n")
	jsonBytes, err := io.ReadAll(fh)
	if err != nil {
		return xerrors.Errorf("failed to read file %s: %w", targetPath, err)
	}

	jsonStr := getPrettyStatusFileJSON(jsonBytes)
	fmt.Printf("%s\n\n", string(jsonStr))

	return nil
}

func getPrettyStatusFileJSON(jsonBytes []byte) string {
	logger := log.WithFields(log.Fields{
		"package":  "main",
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

func getDataPath(targetRootPath string, targetPath string) string {
	rel, err := filepath.Rel(targetRootPath, targetPath)
	if err != nil {
		return targetPath
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

func printDataObjects(entries []*irodsclient_types.IRODSDataObject) {
	// sort by name
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		printDataObject(entry)
	}
}

func printDataObject(entry *irodsclient_types.IRODSDataObject) {
	for _, replica := range entry.Replicas {
		printTextGridRow(false, entry.Name, fmt.Sprintf("%d", entry.Size), replica.CheckSum, replica.ModifyTime)
		break
	}
}

func printCollections(entries []*irodsclient_types.IRODSCollection) {
	// sort by name
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		printTextGridRow(true, entry.Name, "-", "", entry.ModifyTime)
	}
}

func printTextGridHead() {
	printTextGridRowInternal("TYPE", "NAME", "SIZE", "CHECKSUM", "LAST_MODIFIED")
}

func printTextGridRow(isDir bool, name string, size string, checksum string, lastmodified time.Time) {
	typeStr := "File"
	if isDir {
		typeStr = "Dir"
	}

	modTime := commons.MakeDateTimeString(lastmodified)
	printTextGridRowInternal(typeStr, name, size, checksum, modTime)
}

func printTextGridRowInternal(typeStr string, name string, size string, checksum string, lastmodified string) {

	fmt.Printf("%s\t%-50s\t%-12s\t%-32s\t%s\n", typeStr, name, size, checksum, lastmodified)
}
