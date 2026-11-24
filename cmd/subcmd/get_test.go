package subcmd

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSplitConnections(t *testing.T) {
	cases := []struct {
		total int
		jobs  int
		want  []int
	}{
		{5, 1, []int{5}},              // --jobs 1: one worker gets them all (v0.3.12)
		{5, 4, []int{2, 1, 1, 1}},     // defaults
		{16, 4, []int{4, 4, 4, 4}},    // even split
		{10, 4, []int{3, 3, 2, 2}},    // remainder to the first jobs
		{5, 16, []int{1, 1, 1, 1, 1}}, // never more jobs than connections
		{0, 4, []int{1}},              // at least one connection
		{5, 0, []int{5}},              // at least one job
	}

	for _, c := range cases {
		got := splitConnections(c.total, c.jobs)
		assert.Equal(t, c.want, got, "total %d, jobs %d", c.total, c.jobs)

		sum := 0
		for _, n := range got {
			sum += n
		}
		assert.Equal(t, max(c.total, 1), sum, "total %d, jobs %d: shares must add up", c.total, c.jobs)
	}
}

func TestRunGroupsRunsEveryGroupOnce(t *testing.T) {
	const numGroups = 200
	const numWorkers = 4

	var counts [numGroups]int64
	var busy [numWorkers]int32
	var overlap atomic.Bool
	var maxRunning, running int64

	err := runGroups(numGroups, numWorkers, func(workerIdx int, groupIdx int) error {
		if !atomic.CompareAndSwapInt32(&busy[workerIdx], 0, 1) {
			overlap.Store(true) // a worker ran two groups at once
		}
		defer atomic.StoreInt32(&busy[workerIdx], 0)

		now := atomic.AddInt64(&running, 1)
		for {
			prev := atomic.LoadInt64(&maxRunning)
			if now <= prev || atomic.CompareAndSwapInt64(&maxRunning, prev, now) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		atomic.AddInt64(&running, -1)

		atomic.AddInt64(&counts[groupIdx], 1)
		return nil
	})

	assert.NoError(t, err)
	assert.False(t, overlap.Load(), "a worker ran two groups at once")
	assert.LessOrEqual(t, maxRunning, int64(numWorkers))
	assert.Greater(t, maxRunning, int64(1), "groups never ran in parallel")
	for i, n := range counts {
		assert.Equal(t, int64(1), n, "group %d", i)
	}
}

func TestRunGroupsStopsAfterError(t *testing.T) {
	const numGroups = 1000
	boom := errors.New("boom")

	var started int64
	err := runGroups(numGroups, 4, func(workerIdx int, groupIdx int) error {
		atomic.AddInt64(&started, 1)
		if groupIdx == 3 {
			return boom
		}
		time.Sleep(time.Millisecond)
		return nil
	})

	assert.ErrorIs(t, err, boom)
	assert.Less(t, atomic.LoadInt64(&started), int64(numGroups), "groups kept starting after the error")
}

func TestRunGroupsReturnsEveryError(t *testing.T) {
	errA := errors.New("a")
	errB := errors.New("b")

	// both workers fail on their first group; both errors come back
	var wg sync.WaitGroup
	wg.Add(2)
	err := runGroups(2, 2, func(workerIdx int, groupIdx int) error {
		wg.Done()
		wg.Wait() // both groups are running before either fails
		if groupIdx == 0 {
			return errA
		}
		return errB
	})

	assert.ErrorIs(t, err, errA)
	assert.ErrorIs(t, err, errB)
}

func TestRunGroupsOneWorkerKeepsOrder(t *testing.T) {
	var order []int
	err := runGroups(5, 1, func(workerIdx int, groupIdx int) error {
		order = append(order, groupIdx)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, []int{0, 1, 2, 3, 4}, order)
}

func TestRunGroupsNoGroups(t *testing.T) {
	called := false
	err := runGroups(0, 4, func(workerIdx int, groupIdx int) error {
		called = true
		return nil
	})

	assert.NoError(t, err)
	assert.False(t, called)
}
