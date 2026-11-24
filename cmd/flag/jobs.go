package flag

import (
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

const (
	// JobsDefault is how many simulations `get` downloads at once.
	JobsDefault = 4
	// JobsMax is the most simulations `get` downloads at once.
	JobsMax = 16
)

type JobsFlagValues struct {
	Jobs int
}

var (
	jobsFlagValues JobsFlagValues
)

func SetJobsFlags(command *cobra.Command) {
	command.Flags().IntVarP(&jobsFlagValues.Jobs, "jobs", "j", JobsDefault, "Set the number of simulations to download at once (1-16); they share the --thread_num connections")
}

func GetJobsFlagValues() (*JobsFlagValues, error) {
	err := ValidateJobs(jobsFlagValues.Jobs)
	if err != nil {
		return nil, err
	}

	return &jobsFlagValues, nil
}

// ValidateJobs refuses a number of jobs outside 1..JobsMax.
func ValidateJobs(jobs int) error {
	if jobs < 1 || jobs > JobsMax {
		return errors.Errorf("--jobs must be between 1 and %d, got %d", JobsMax, jobs)
	}

	return nil
}
