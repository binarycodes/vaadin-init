package version

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		raw       string
		ok        bool
		qualifier string
	}{
		{"25.2.6", true, ""},
		{"25.2", true, ""},
		{"25.2.0-beta1", true, "beta1"},
		{"4.1.1", true, ""},
		{"not-a-version", false, ""},
		{"", false, ""},
	}
	for _, c := range cases {
		v, ok := Parse(c.raw)
		if ok != c.ok {
			t.Errorf("Parse(%q) ok = %v, want %v", c.raw, ok, c.ok)
			continue
		}
		if ok && v.Qualifier != c.qualifier {
			t.Errorf("Parse(%q) qualifier = %q, want %q", c.raw, v.Qualifier, c.qualifier)
		}
		if ok && v.Stable() != (c.qualifier == "") {
			t.Errorf("Parse(%q).Stable() = %v", c.raw, v.Stable())
		}
	}
}

// By number, not by text: "25.1.10" is newer than "25.1.9".
func TestCompareIsNumeric(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"25.1.10", "25.1.9", 1},
		{"25.1.9", "25.1.10", -1},
		{"4.1.0", "4.0.8", 1},
		{"25.2.6", "25.2.6", 0},
		{"25.3.0-beta1", "25.3.0", 0},
		{"25.2", "25.2.0", 0},
		{"garbage", "1.0.0", -1},
		{"1.0.0", "garbage", 1},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		if (got < 0) != (c.want < 0) || (got > 0) != (c.want > 0) {
			t.Errorf("Compare(%q, %q) = %d, want sign of %d", c.a, c.b, got, c.want)
		}
	}
}

func TestInLine(t *testing.T) {
	v, _ := Parse("25.2.6")
	for line, want := range map[string]bool{
		"25":       true,
		"25.2":     true,
		"25.2.6":   true,
		"25.1":     false,
		"24":       false,
		"2":        false,
		"25.2.6.1": false,
		"x":        false,
	} {
		if got := v.InLine(line); got != want {
			t.Errorf("25.2.6 InLine(%q) = %v, want %v", line, got, want)
		}
	}
	if got := v.MinorLine(); got != "25.2" {
		t.Errorf("MinorLine = %q", got)
	}
}

// A floor is the lowest version a prefix names, and only a number names one.
func TestParseFloor(t *testing.T) {
	for name, want := range map[string]string{"25": "25.0.0", "25.2": "25.2.0", "25.2.3": "25.2.3"} {
		v, ok := ParseFloor(name)
		if !ok {
			t.Errorf("ParseFloor(%q) refused", name)
			continue
		}
		if got := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch); got != want {
			t.Errorf("ParseFloor(%q) = %s, want %s", name, got, want)
		}
	}
	for _, name := range []string{"v25", "latest", "25.2-beta1", "25.2.0-beta1", "", "25..2", "25.2.3.4", "025"} {
		if _, ok := ParseFloor(name); ok {
			t.Errorf("ParseFloor(%q) accepted", name)
		}
	}

	// Against releases: a floor is at or below everything in its line, and
	// above the line before it.
	floor, _ := ParseFloor("25.2")
	for raw, want := range map[string]int{"25.2.6": -1, "25.2.0": 0, "25.1.9": 1, "25.3.0-beta1": -1} {
		v, _ := Parse(raw)
		if got := floor.Compare(v); (got < 0) != (want < 0) || (got > 0) != (want > 0) {
			t.Errorf("floor 25.2 against %s = %d, want sign of %d", raw, got, want)
		}
	}
}
