package flag

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeFlagName(t *testing.T) {
	newCommand := func() (*cobra.Command, *string, *bool) {
		root := &cobra.Command{Use: "root"}
		root.SetGlobalNormalizationFunc(NormalizeFlagName)

		var svcURL string
		var noID bool
		sub := &cobra.Command{Use: "sub", RunE: func(*cobra.Command, []string) error { return nil }}
		sub.Flags().StringVar(&svcURL, "svc_url", "", "")
		sub.Flags().BoolVar(&noID, "no-id", false, "") // registered with a dash
		root.AddCommand(sub)

		return root, &svcURL, &noID
	}

	for _, args := range [][]string{
		{"sub", "--svc_url", "x", "--no_id"},
		{"sub", "--svc-url", "x", "--no-id"},
		{"sub", "--svc-url=x", "--no_id"},
	} {
		root, svcURL, noID := newCommand()
		root.SetArgs(args)
		require.NoError(t, root.Execute(), "%v", args)
		assert.Equal(t, "x", *svcURL, "%v", args)
		assert.True(t, *noID, "%v", args)
	}

	// Changed() and Lookup() find a flag under either spelling
	root, _, _ := newCommand()
	root.SetArgs([]string{"sub", "--svc-url", "x"})
	require.NoError(t, root.Execute())
	sub, _, err := root.Find([]string{"sub"})
	require.NoError(t, err)
	assert.True(t, sub.Flags().Changed("svc_url"))
	assert.True(t, sub.Flags().Changed("svc-url"))
	assert.NotNil(t, sub.Flags().Lookup("no-id"))
}
