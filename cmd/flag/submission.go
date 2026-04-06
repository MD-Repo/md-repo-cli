package flag

import (
	"github.com/spf13/cobra"
)

type SubmissionFlagValues struct {
	ExpectedSimulations int
	OrcID               string
	NoID                bool
}

var (
	submissionFlagValues SubmissionFlagValues
)

func SetSubmissionFlags(command *cobra.Command) {
	command.Flags().IntVarP(&submissionFlagValues.ExpectedSimulations, "expected_simulations", "n", 0, "Set the number of expected simulations")
	command.Flags().StringVar(&submissionFlagValues.OrcID, "orcid", "", "Set ORC-ID")
	command.Flags().BoolVar(&submissionFlagValues.NoID, "no-id", false, "Submit without an ID")
}

func GetSubmissionFlagValues() *SubmissionFlagValues {
	return &submissionFlagValues
}
