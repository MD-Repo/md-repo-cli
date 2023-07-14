package subcmd

import (
	"fmt"
	"sort"
	"strings"

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

	// display
	logger.Debugf("submission iRODS ticket: %s", mdRepoTicket.IRODSTicket)
	logger.Debugf("submission path: %s", mdRepoTicket.IRODSDataPath)

	err = listOne(filesystem, mdRepoTicket.IRODSDataPath)
	if err != nil {
		return xerrors.Errorf("failed to list %s: %w", mdRepoTicket.IRODSDataPath, err)
	}

	return nil
}

func listOne(fs *irodsclient_fs.FileSystem, targetPath string) error {
	targetPath = commons.MakeIRODSLandingPath(targetPath)

	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	collection, err := irodsclient_irodsfs.GetCollection(connection, targetPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return xerrors.Errorf("failed to get collection %s: %w", targetPath, err)
		}
		return err
	}

	colls, err := irodsclient_irodsfs.ListSubCollections(connection, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to list sub-collections in %s: %w", targetPath, err)
	}

	objs, err := irodsclient_irodsfs.ListDataObjects(connection, collection)
	if err != nil {
		return xerrors.Errorf("failed to list data-objects in %s: %w", targetPath, err)
	}

	fmt.Printf("  NAME\tOWNER\tSIZE\tCHECKSUM\tMOD_TIME\n")
	printDataObjects(objs)
	printCollections(colls)
	return nil
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
		modTime := commons.MakeDateTimeString(replica.ModifyTime)
		fmt.Printf("  %s\t%s\t%d\t%s\t%s\n", entry.Name, replica.Owner, entry.Size, replica.CheckSum, modTime)
		break
	}
}

func printCollections(entries []*irodsclient_types.IRODSCollection) {
	// sort by name
	sort.SliceStable(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	for _, entry := range entries {
		fmt.Printf("  %s\t%s\t%d\t%s\t%s\n", entry.Name, entry.Owner, 0, "", entry.ModifyTime)
	}
}
