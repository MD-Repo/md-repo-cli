package flag

import (
	"github.com/spf13/cobra"
)

type ProgressFlagValues struct {
	NoProgress bool
}

var (
	progressFlagValues ProgressFlagValues
)

func SetProgressFlags(command *cobra.Command) {
	command.Flags().BoolVar(&progressFlagValues.NoProgress, "no_progress", false, "Do not display progress bars")
}

func GetProgressFlagValues() *ProgressFlagValues {
	return &progressFlagValues
}
