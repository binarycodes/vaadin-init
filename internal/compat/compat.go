// Package compat holds the rules for which Spring Boot and which JDK go with a
// Vaadin release, read from compat.json.
//
// The rules exist only as prose in release notes and documentation pages —
// nobody publishes them as data — so this repository writes them down in a file a
// pull request can change without touching Go, and a scheduled job checks that
// file against what Maven Central publishes. Nothing here touches the network:
// the exact releases come from the lookup, and this package only says which of
// them may go together.
package compat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/binarycodes/vaadin-init/internal/version"
)

// Rules is the shape of compat.json.
type Rules struct {
	// Comment is the editing procedure at the top of the file. Carried so that a
	// file can be decoded strictly, not for anything the tool does with it.
	Comment []string `json:"$comment,omitempty"`

	VaadinRules []VaadinRule `json:"vaadin"`
	BootRules   []BootRule   `json:"boot"`
	Java        JavaRule     `json:"java"`
}

// VaadinRule is one Vaadin line: what it needs, and whether this tool has
// templates for it.
type VaadinRule struct {
	Line      string  `json:"line"`
	Supported bool    `json:"supported"`
	JavaMin   int     `json:"java_min"`
	BootLine  string  `json:"boot_line"`
	BootMin   string  `json:"boot_min"`
	Since     []Since `json:"since,omitempty"`
	Source    string  `json:"source"`
}

// Since tightens a line's Boot minimum from one Vaadin release onwards.
type Since struct {
	Vaadin  string `json:"vaadin"`
	BootMin string `json:"boot_min"`
}

// BootRule is one Spring Boot minor line: the JDKs it runs on, and when its
// support ends. JavaMax is inclusive.
type BootRule struct {
	Line    string `json:"line"`
	JavaMin int    `json:"java_min"`
	JavaMax int    `json:"java_max"`
	EOL     string `json:"eol"`
	Source  string `json:"source"`
}

// JavaRule is what the rules know about Java itself: which majors are
// long-term-support releases.
type JavaRule struct {
	LTS []int `json:"lts"`
}

// UserPath is where a personal rules file is looked for, beside the personal
// defaults file.
func UserPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vaadin-init", "compat.json"), nil
}

// Load decodes the embedded rules, then layers an override the way the defaults
// file is layered: a line named in the override replaces the embedded line of
// the same name, and lines it does not name are kept. The LTS list is replaced
// whole when given.
//
// An explicit path is an instruction, so a missing file there is an error. The
// per-user path is a convention, so a missing file there is the normal case. A
// malformed file is an error naming the entry.
func Load(embedded []byte, explicitPath string) (Rules, error) {
	rules, err := Parse(embedded)
	if err != nil {
		return rules, fmt.Errorf("the built-in compatibility rules: %w", err)
	}

	path := explicitPath
	if path == "" {
		userPath, err := UserPath()
		if err != nil {
			return rules, nil
		}
		if _, err := os.Stat(userPath); err != nil {
			return rules, nil
		}
		path = userPath
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return rules, fmt.Errorf("reading compatibility rules from %s: %w", path, err)
	}
	override, err := Parse(content)
	if err != nil {
		return rules, fmt.Errorf("compatibility rules from %s: %w", path, err)
	}
	merged := rules.layered(override)
	if err := merged.validate(); err != nil {
		return rules, fmt.Errorf("compatibility rules from %s: %w", path, err)
	}
	return merged, nil
}

// Parse reads one file strictly — an unknown key is a misspelt one — and checks
// it stands on its own. Load is the usual way in; this is for a caller that has
// the bytes and wants no layering, such as the CI check.
func Parse(content []byte) (Rules, error) {
	var rules Rules
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rules); err != nil {
		return rules, fmt.Errorf("not valid: %w", err)
	}
	return rules, rules.validate()
}

func (r Rules) layered(override Rules) Rules {
	merged := Rules{
		Comment:     r.Comment,
		VaadinRules: slices.Clone(r.VaadinRules),
		BootRules:   slices.Clone(r.BootRules),
		Java:        r.Java,
	}
	for _, rule := range override.VaadinRules {
		i := slices.IndexFunc(merged.VaadinRules, func(v VaadinRule) bool { return v.Line == rule.Line })
		if i < 0 {
			merged.VaadinRules = append(merged.VaadinRules, rule)
		} else {
			merged.VaadinRules[i] = rule
		}
	}
	for _, rule := range override.BootRules {
		i := slices.IndexFunc(merged.BootRules, func(b BootRule) bool { return b.Line == rule.Line })
		if i < 0 {
			merged.BootRules = append(merged.BootRules, rule)
		} else {
			merged.BootRules[i] = rule
		}
	}
	if override.Java.LTS != nil {
		merged.Java = override.Java
	}
	return merged
}

// validate is what a file has to get right for the rules to be usable at all.
// Every message names the entry, because a rules file is edited by hand and the
// person editing it is looking at the entry, not at a struct.
func (r Rules) validate() error {
	seen := map[string]bool{}
	for _, rule := range r.VaadinRules {
		where := fmt.Sprintf("vaadin line %q", rule.Line)
		if err := validLine(rule.Line); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		if seen["vaadin "+rule.Line] {
			return fmt.Errorf("%s: listed twice", where)
		}
		seen["vaadin "+rule.Line] = true
		if rule.JavaMin <= 0 {
			return fmt.Errorf("%s: java_min must be a Java major version", where)
		}
		if err := validLine(rule.BootLine); err != nil {
			return fmt.Errorf("%s: boot_line: %w", where, err)
		}
		if err := validVersionIn(rule.BootMin, rule.BootLine); err != nil {
			return fmt.Errorf("%s: boot_min: %w", where, err)
		}
		for _, since := range rule.Since {
			if err := validVersionIn(since.Vaadin, rule.Line); err != nil {
				return fmt.Errorf("%s: since: vaadin: %w", where, err)
			}
			if err := validVersionIn(since.BootMin, rule.BootLine); err != nil {
				return fmt.Errorf("%s: since %s: boot_min: %w", where, since.Vaadin, err)
			}
		}
	}
	for _, rule := range r.BootRules {
		where := fmt.Sprintf("boot line %q", rule.Line)
		if err := validLine(rule.Line); err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		if seen["boot "+rule.Line] {
			return fmt.Errorf("%s: listed twice", where)
		}
		seen["boot "+rule.Line] = true
		if rule.JavaMin <= 0 || rule.JavaMax < rule.JavaMin {
			return fmt.Errorf("%s: java_min %d and java_max %d do not make a range", where, rule.JavaMin, rule.JavaMax)
		}
		if rule.EOL != "" {
			if _, err := time.Parse(time.DateOnly, rule.EOL); err != nil {
				return fmt.Errorf("%s: eol %q is not a date like 2027-07-31", where, rule.EOL)
			}
		}
	}
	if len(r.Java.LTS) == 0 {
		return errors.New("java: lts must name at least one release")
	}
	return nil
}

// validLine accepts "25" or "4.1": leading version numbers, nothing else.
func validLine(line string) error {
	parts := strings.Split(line, ".")
	if line == "" || len(parts) > 2 {
		return fmt.Errorf("%q is not a line like 25 or 4.1", line)
	}
	for _, part := range parts {
		if n, err := strconv.Atoi(part); err != nil || n < 0 {
			return fmt.Errorf("%q is not a line like 25 or 4.1", line)
		}
	}
	return nil
}

func validVersionIn(raw, line string) error {
	v, ok := version.Parse(raw)
	if !ok {
		return fmt.Errorf("%q is not a version", raw)
	}
	if !v.InLine(line) {
		return fmt.Errorf("%q is not in line %s", raw, line)
	}
	return nil
}

// Supported lists the Vaadin lines this tool has templates for, and the Boot
// lines they sit on — the filter for what the lookup offers.
func (r Rules) Supported() (vaadin, boot []string) {
	for _, rule := range r.VaadinRules {
		if rule.Supported {
			vaadin = append(vaadin, rule.Line)
			if !slices.Contains(boot, rule.BootLine) {
				boot = append(boot, rule.BootLine)
			}
		}
	}
	return vaadin, boot
}

// Vaadin returns the rule for a version's line, or an error saying the line is
// not one this tool generates.
func (r Rules) Vaadin(raw string) (VaadinRule, error) {
	v, ok := version.Parse(raw)
	if !ok {
		return VaadinRule{}, fmt.Errorf("%q is not a version", raw)
	}
	supported, _ := r.Supported()
	generates := "this tool generates Vaadin " + strings.Join(supported, " and ") + " projects"

	rule, found := longestLine(r.VaadinRules, func(rule VaadinRule) string { return rule.Line }, v)
	if !found {
		return VaadinRule{}, fmt.Errorf("%s; got %s, and there is no rule for Vaadin %d", generates, raw, v.Major)
	}
	if !rule.Supported {
		return VaadinRule{}, fmt.Errorf("%s; got %s, and Vaadin %s sits on Spring Boot %s%s",
			generates, raw, rule.Line, rule.BootLine, cite(rule.Source))
	}
	return rule, nil
}

// longestLine finds the rule whose line the version is in, the most specific
// line winning when a file names both "4" and "4.1".
func longestLine[T any](rules []T, line func(T) string, v version.Version) (T, bool) {
	var best T
	found := false
	for _, rule := range rules {
		if v.InLine(line(rule)) && (!found || len(line(rule)) > len(line(best))) {
			best, found = rule, true
		}
	}
	return best, found
}

// bootRule is the rule for a Boot version's minor line, if one is written down.
func (r Rules) bootRule(v version.Version) (BootRule, bool) {
	return longestLine(r.BootRules, func(rule BootRule) string { return rule.Line }, v)
}

// BootMin is the lowest Spring Boot a Vaadin version accepts, its line's
// `since` entries applied.
func (r Rules) BootMin(vaadin string) (string, error) {
	rule, err := r.Vaadin(vaadin)
	if err != nil {
		return "", err
	}
	return rule.bootMin(vaadin), nil
}

func (rule VaadinRule) bootMin(vaadin string) string {
	lowest := rule.BootMin
	for _, since := range rule.Since {
		if version.Compare(vaadin, since.Vaadin) >= 0 && version.Compare(since.BootMin, lowest) > 0 {
			lowest = since.BootMin
		}
	}
	return lowest
}

// checkBoot says why a Boot version does not go with a Vaadin version, or
// nothing when it does.
func (r Rules) checkBoot(rule VaadinRule, vaadin, boot string) error {
	b, ok := version.Parse(boot)
	if !ok {
		return fmt.Errorf("%q is not a version", boot)
	}
	if !b.InLine(rule.BootLine) {
		return fmt.Errorf("Vaadin %s sits on Spring Boot %s; got %s%s", rule.Line, rule.BootLine, boot, cite(rule.Source))
	}
	if lowest := rule.bootMin(vaadin); version.Compare(boot, lowest) < 0 {
		return fmt.Errorf("Vaadin %s needs Spring Boot %s or newer; got %s%s", vaadin, lowest, boot, cite(rule.Source))
	}
	return nil
}

// CompatibleBoot filters a Boot list, order kept, to the releases a Vaadin
// version accepts. Nothing is compatible with a Vaadin version there is no rule
// for.
func (r Rules) CompatibleBoot(vaadin string, candidates []string) []string {
	rule, err := r.Vaadin(vaadin)
	if err != nil {
		return nil
	}
	var compatible []string
	for _, boot := range candidates {
		if r.checkBoot(rule, vaadin, boot) == nil {
			compatible = append(compatible, boot)
		}
	}
	return compatible
}

// JavaRange is the JDK majors a Vaadin and Boot pair runs on: the higher of the
// two floors, up to Boot's ceiling. The ceiling is 0 when the Boot line has no
// rule yet — a minor newer than the file — which means nobody has written one
// down, not that there is none.
//
// The pair itself is checked first, so an incompatible pair is an error before
// Java comes into it.
func (r Rules) JavaRange(vaadin, boot string) (floor, ceiling int, err error) {
	rule, err := r.Vaadin(vaadin)
	if err != nil {
		return 0, 0, err
	}
	if err := r.checkBoot(rule, vaadin, boot); err != nil {
		return 0, 0, err
	}
	floor = rule.JavaMin
	b, _ := version.Parse(boot)
	if bootRule, ok := r.bootRule(b); ok {
		floor = max(floor, bootRule.JavaMin)
		ceiling = bootRule.JavaMax
	}
	return floor, ceiling, nil
}

// NewestJava is the newest JDK any Boot line in the rules supports: the top of
// what is offered when the chosen Boot has no ceiling written down.
func (r Rules) NewestJava() int {
	newest := 0
	for _, rule := range r.BootRules {
		newest = max(newest, rule.JavaMax)
	}
	return newest
}

// LTS reports whether a Java major is a long-term-support release.
func (r Rules) LTS(java int) bool { return slices.Contains(r.Java.LTS, java) }

// JavaDefault is where the Java question opens: the preferred release when the
// range allows it, otherwise the newest LTS release in the range, otherwise the
// floor. The second value says whether the preference had to be given up.
func (r Rules) JavaDefault(floor, ceiling, preferred int) (int, bool) {
	if inRange(preferred, floor, ceiling) {
		return preferred, false
	}
	chosen := floor
	for _, lts := range r.Java.LTS {
		if inRange(lts, floor, ceiling) && lts > chosen {
			chosen = lts
		}
	}
	return chosen, true
}

// inRange treats a ceiling of 0 as none written down.
func inRange(java, floor, ceiling int) bool {
	return java >= floor && (ceiling == 0 || java <= ceiling)
}

// CheckBoot says why a Boot version does not go with a Vaadin version, or
// nothing when it does. Bare, for the field that asks for Boot.
func (r Rules) CheckBoot(vaadin, boot string) error {
	rule, err := r.Vaadin(vaadin)
	if err != nil {
		return err
	}
	return r.checkBoot(rule, vaadin, boot)
}

// CheckJava says why a JDK major does not go with a Vaadin and Boot pair, or
// nothing when it does. The pair is checked first. Bare, for the field that asks
// for Java.
func (r Rules) CheckJava(vaadin, boot string, java int) error {
	rule, err := r.Vaadin(vaadin)
	if err != nil {
		return err
	}
	if err := r.checkBoot(rule, vaadin, boot); err != nil {
		return err
	}
	if java < rule.JavaMin {
		return fmt.Errorf("Vaadin %s needs Java %d or newer; got %d%s",
			rule.Line, rule.JavaMin, java, cite(rule.Source))
	}
	b, _ := version.Parse(boot)
	if bootRule, ok := r.bootRule(b); ok {
		if java < bootRule.JavaMin {
			return fmt.Errorf("Spring Boot %s needs Java %d or newer; got %d%s",
				bootRule.Line, bootRule.JavaMin, java, cite(bootRule.Source))
		}
		if java > bootRule.JavaMax {
			return fmt.Errorf("Spring Boot %s supports Java up to %d; got %d%s",
				bootRule.Line, bootRule.JavaMax, java, cite(bootRule.Source))
		}
	}
	return nil
}

// Check is every cross-field rule at once, for a whole Config. Each message
// names the field it is about, the rule, and where the rule came from.
func (r Rules) Check(vaadin, boot string, java int) error {
	if _, err := r.Vaadin(vaadin); err != nil {
		return fmt.Errorf("vaadin version: %w", err)
	}
	if err := r.CheckBoot(vaadin, boot); err != nil {
		return fmt.Errorf("spring boot version: %w", err)
	}
	if err := r.CheckJava(vaadin, boot, java); err != nil {
		return fmt.Errorf("java version: %w", err)
	}
	return nil
}

// BootDefault is the Boot to offer for a Vaadin version: the release it was
// built with when that is known and the rules allow it, otherwise the newest
// candidate they allow, otherwise nothing.
//
// The pin is checked against the rules rather than trusted outright, because the
// two can disagree for a week — a pin that has moved ahead of the rules is what
// the scheduled check exists to notice — and a default the validator then
// refuses is worse than an older one it accepts.
func (r Rules) BootDefault(vaadin, pinned string, candidates []string) string {
	if pinned != "" && len(r.CompatibleBoot(vaadin, []string{pinned})) == 1 {
		return pinned
	}
	compatible := r.CompatibleBoot(vaadin, candidates)
	if len(compatible) == 0 {
		return ""
	}
	return compatible[0]
}

// Describe says the Java range in a sentence, for the question that asks it:
// "21 or newer for Vaadin 25, up to 26 for Spring Boot 4.1."
func (r Rules) Describe(vaadin, boot string) string {
	rule, err := r.Vaadin(vaadin)
	if err != nil {
		return ""
	}
	floor := rule.JavaMin
	var bootRule BootRule
	known := false
	if b, ok := version.Parse(boot); ok {
		bootRule, known = r.bootRule(b)
	}
	if known && bootRule.JavaMin > floor {
		return fmt.Sprintf("%d or newer for Spring Boot %s, up to %d.", bootRule.JavaMin, bootRule.Line, bootRule.JavaMax)
	}
	sentence := fmt.Sprintf("%d or newer for Vaadin %s", floor, rule.Line)
	if known {
		sentence += fmt.Sprintf(", up to %d for Spring Boot %s", bootRule.JavaMax, bootRule.Line)
	}
	return sentence + "."
}

// cite is the source on its own line, for the person reading the error to
// follow.
func cite(source string) string {
	if source == "" {
		return ""
	}
	return "\n  (" + source + ")"
}
