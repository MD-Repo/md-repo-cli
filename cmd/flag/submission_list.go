package flag

import (
	"github.com/spf13/cobra"
)

type SubmissionListFlagValues struct {
	OrcID string
}

var (
	submissionListFlagValues SubmissionListFlagValues
)

func SetSubmissionListFlags(command *cobra.Command) {
	command.Flags().StringVar(&submissionListFlagValues.OrcID, "orcid", "", "Specify ORC-ID")
}

func GetSubmissionListFlagValues() *SubmissionListFlagValues {
	return &submissionListFlagValues
}
