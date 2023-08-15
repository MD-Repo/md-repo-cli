package flag

import (
	"github.com/spf13/cobra"
)

type SubmissionFlagValues struct {
	ExpectedSimulations int
}

var (
	submissionFlagValues SubmissionFlagValues
)

func SetSubmissionFlags(command *cobra.Command) {
	command.Flags().IntVarP(&submissionFlagValues.ExpectedSimulations, "expected_simulations", "n", 0, "Specify the number of expected simulations")
}

func GetSubmissionFlagValues() *SubmissionFlagValues {
	return &submissionFlagValues
}
