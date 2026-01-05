package commons

import (
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/dustin/go-humanize"
)

const (
	KiloBytes int64 = 1024
	MegaBytes int64 = KiloBytes * 1024
	GigaBytes int64 = MegaBytes * 1024
	TeraBytes int64 = GigaBytes * 1024

	Minute int = 60
	Hour   int = Minute * 60
	Day    int = Hour * 24
)

func ParseSize(size string) (uint64, error) {
	sizeNum, err := humanize.ParseBytes(size)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to parse size string %q to uint64", size)
	}

	return sizeNum, nil
}

func ToSizeString(size uint64) string {
	return humanize.IBytes(size)
}

func ParseTime(t string) (int, error) {
	t = strings.TrimSpace(t)
	t = strings.ToUpper(t)

	tNum := int64(0)
	var err error

	switch t[len(t)-1] {
	case 'S', 'M', 'H', 'D':
		tNum, err = strconv.ParseInt(t[:len(t)-1], 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert string %q to int", t)
		}
	default:
		tNum, err = strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, errors.Wrapf(err, "failed to convert string %q to int", t)
		}
		return int(tNum), nil
	}

	switch t[len(t)-1] {
	case 'M':
		return int(tNum) * Minute, nil
	case 'H':
		return int(tNum) * Hour, nil
	case 'D':
		return int(tNum) * Day, nil
	default:
		return int(tNum), nil
	}
}
