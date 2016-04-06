package shared

import (
	"strings"
)

var (
	// EnabledGroups is map with the group name as field and a boolean indicating wether that group is enabled or not.
	EnabledGroups = make(map[string]bool)
)

// ParseEnabledGroups parses the groups passed by the command line input.
func ParseEnabledGroups(enabledGroupsFlag string) {
	for _, name := range strings.Split(enabledGroupsFlag, ",") {
		name = strings.TrimSpace(name)
		EnabledGroups[name] = true
	}
}
