package flag

import (
	"github.com/spf13/cobra"
)

type SubmissionFlagValues struct {
	ExpectedSimulations int
	OrcID               string
}

var (
	submissionFlagValues SubmissionFlagValues
)

func SetSubmissionFlags(command *cobra.Command) {
	command.Flags().IntVarP(&submissionFlagValues.ExpectedSimulations, "expected_simulations", "n", 0, "Specify the number of expected simulations")
	command.Flags().StringVar(&submissionFlagValues.OrcID, "orcid", "", "Specify ORC-ID")
}

func GetSubmissionFlagValues() *SubmissionFlagValues {
	return &submissionFlagValues
}
