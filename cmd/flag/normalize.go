package flag

import (
	"strings"

	"github.com/spf13/pflag"
)

// NormalizeFlagName lets every flag be spelled with dashes or underscores
// (--svc-url = --svc_url). Underscores are the canonical form, so help lists
// the flags as before.
func NormalizeFlagName(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	return pflag.NormalizedName(strings.ReplaceAll(name, "-", "_"))
}
