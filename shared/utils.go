package shared

import (
	"regexp"
	"strings"
	"strconv"
)

var (
	snakeRegexp        = regexp.MustCompile("\\B[A-Z]+[^_$]")
	parameterizeRegexp = regexp.MustCompile("[^A-Za-z0-9_]+")
)

// SnakeCase converts the given text to snakecase/underscore syntax.
func SnakeCase(text string) string {
	result := snakeRegexp.ReplaceAllStringFunc(text, func(match string) string {
		return "_" + match
	})

	return ParameterizeString(result)
}

// ParameterizeString parameterizes the given string.
func ParameterizeString(text string) string {
	result := parameterizeRegexp.ReplaceAllString(text, "_")
	return strings.ToLower(result)
}

func IsVersionGreater(version string, major int, minor int, release int) bool {
	split := strings.Split(version, ".")
	cmp_major, _ := strconv.Atoi(split[0])
	cmp_minor, _ := strconv.Atoi(split[1])
	cmp_release, _ := strconv.Atoi(split[2])

	if cmp_major >= major && cmp_minor >= minor && cmp_release >= release {
		return true
	}

	return false
}

