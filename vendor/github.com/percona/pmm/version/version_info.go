// Package version provides helpers for working with versions and build info.
package version

import (
	"fmt"
	"regexp"
	"strconv"
)

var versionRE = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(.*)$`)

type Info struct {
	Major int
	Minor int
	Patch int
	Rest  string // pre-release version and/or build metadata
}

func Parse(version string) (Info, error) {
	m := versionRE.FindStringSubmatch(version)
	if len(m) != 5 {
		return Info{}, fmt.Errorf("failed to parse %q", version)
	}
	pv := Info{Rest: m[4]}
	var err error
	if pv.Major, err = strconv.Atoi(m[1]); err != nil {
		return Info{}, err
	}
	if pv.Minor, err = strconv.Atoi(m[2]); err != nil {
		return Info{}, err
	}
	if pv.Patch, err = strconv.Atoi(m[3]); err != nil {
		return Info{}, err
	}
	return pv, nil
}

func (i Info) String() string {
	res := fmt.Sprintf("%d.%d.%d", i.Major, i.Minor, i.Patch)
	if i.Rest != "" {
		res += "." + i.Rest
	}
	return res
}

// Less returns true of this (left) Version is less than right.
func (i *Info) Less(right *Info) bool {
	if i.Major != right.Major {
		return i.Major < right.Major
	}
	if i.Minor != right.Minor {
		return i.Minor < right.Minor
	}
	if i.Patch != right.Patch {
		return i.Patch < right.Patch
	}

	// Pre-release versions have a lower precedence than the associated normal version.
	if i.Rest == "" && right.Rest != "" {
		return true
	}
	if i.Rest != "" && right.Rest == "" {
		return false
	}

	// For now, we ignore proper parsing and comparision of pre-release versions.
	return false
}
