package flag

import (
	"github.com/spf13/cobra"
)

type ProgressFlagValues struct {
	NoProgress   bool
	ShowFullPath bool
}

var (
	progressFlagValues ProgressFlagValues
)

func SetProgressFlags(command *cobra.Command) {
	command.Flags().BoolVar(&progressFlagValues.NoProgress, "no_progress", false, "Do not display progress bars")
	command.Flags().BoolVar(&progressFlagValues.ShowFullPath, "show_path", false, "Show full file paths in progress bars")
}

func GetProgressFlagValues() *ProgressFlagValues {
	return &progressFlagValues
}
