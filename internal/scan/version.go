package scan

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a Kubernetes minor version, e.g. 1.29. Patch levels are parsed but
// ignored: no API has ever been removed in a patch release.
type Version struct {
	Major int
	Minor int
}

// ParseVersion accepts "1.29", "v1.29" and "1.29.3".
func ParseVersion(s string) (Version, error) {
	t := strings.TrimSpace(s)
	t = strings.TrimPrefix(t, "v")
	if t == "" {
		return Version{}, fmt.Errorf("empty version")
	}
	parts := strings.Split(t, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Version{}, fmt.Errorf("version %q: want MAJOR.MINOR", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("version %q: bad major", s)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("version %q: bad minor", s)
	}
	if len(parts) == 3 {
		if _, err := strconv.Atoi(parts[2]); err != nil {
			return Version{}, fmt.Errorf("version %q: bad patch", s)
		}
	}
	return Version{Major: major, Minor: minor}, nil
}

// AtLeast reports whether v >= other.
func (v Version) AtLeast(other Version) bool {
	if v.Major != other.Major {
		return v.Major > other.Major
	}
	return v.Minor >= other.Minor
}

func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }
