package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateJobs(t *testing.T) {
	for _, jobs := range []int{1, JobsDefault, JobsMax} {
		assert.NoError(t, ValidateJobs(jobs), "jobs %d", jobs)
	}

	for _, jobs := range []int{-1, 0, JobsMax + 1} {
		assert.Error(t, ValidateJobs(jobs), "jobs %d", jobs)
	}
}
