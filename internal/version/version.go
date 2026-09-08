// Package version parses and compares Maven-style version numbers.
//
// Shared by the lookup, which sorts what Maven Central lists, and by the
// compatibility rules, which decide what goes with what; kept apart from both so
// that neither has to import the other for a comparison.
package version

import (
	"regexp"
	"strconv"
	"strings"
)

// Version is a Maven version split far enough to compare it and to tell a
// release from a pre-release.
type Version struct {
	Raw                 string
	Major, Minor, Patch int
	Qualifier           string
}

var pattern = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?(?:[-.]([A-Za-z0-9.]+))?$`)

// Parse reads a version. Anything it does not recognise is reported as unparsed
// rather than guessed at, and the caller then skips it.
func Parse(raw string) (Version, bool) {
	match := pattern.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return Version{}, false
	}
	number := func(s string) int {
		n, _ := strconv.Atoi(s)
		return n
	}
	return Version{
		Raw:       raw,
		Major:     number(match[1]),
		Minor:     number(match[2]),
		Patch:     number(match[3]),
		Qualifier: match[4],
	}, true
}

// Stable reports whether this is a release rather than an alpha, beta or
// release candidate.
func (v Version) Stable() bool { return v.Qualifier == "" }

// Compare orders two versions by number: negative when v is older than other,
// zero when the numbers are the same, positive when newer. Qualifiers are not
// compared — 25.3.0-beta1 and 25.3.0 sort together.
func (v Version) Compare(other Version) int {
	switch {
	case v.Major != other.Major:
		return v.Major - other.Major
	case v.Minor != other.Minor:
		return v.Minor - other.Minor
	}
	return v.Patch - other.Patch
}

func (v Version) After(other Version) bool   { return v.Compare(other) > 0 }
func (v Version) AtLeast(other Version) bool { return v.Compare(other) >= 0 }

// InLine reports whether the version belongs to a line named by its leading
// numbers: "25" takes every 25.x.y, "4.1" every 4.1.x.
func (v Version) InLine(line string) bool {
	parts := strings.Split(line, ".")
	numbers := []int{v.Major, v.Minor, v.Patch}
	if len(parts) > len(numbers) {
		return false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n != numbers[i] {
			return false
		}
	}
	return true
}

// MinorLine is the version's major.minor, the granularity Spring Boot's rules
// are written at.
func (v Version) MinorLine() string {
	return strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor)
}

// Compare orders two raw versions the way Version.Compare does. Either failing
// to parse sorts as older than anything that does, and two unparsable versions
// sort together.
func Compare(a, b string) int {
	va, okA := Parse(a)
	vb, okB := Parse(b)
	switch {
	case !okA && !okB:
		return 0
	case !okA:
		return -1
	case !okB:
		return 1
	}
	return va.Compare(vb)
}
