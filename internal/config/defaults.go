package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/BurntSushi/toml"
)

// Defaults is the shape of defaults.toml: what the prompts start out showing.
//
// It is kept separate from Config because the two answer different questions. A
// default is a suggestion that may be stale or absent; a Config is a decision the
// generator can act on. Collapsing them would mean a half-filled Config floating
// around with no way to tell which fields anyone had actually chosen.
type Defaults struct {
	GroupID       string `toml:"group_id"`
	ArtifactID    string `toml:"artifact_id"`
	Description   string `toml:"description"`
	JavaVersion   string `toml:"java_version"`
	VaadinVersion string `toml:"vaadin_version"`
	BootVersion   string `toml:"boot_version"`
	Features      struct {
		Database  bool `toml:"database"`
		Auth      bool `toml:"auth"`
		E2E       bool `toml:"e2e"`
		Coverage  bool `toml:"coverage"`
		Traceable bool `toml:"traceable"`
	} `toml:"features"`
}

// UserDefaultsPath is where a personal defaults file is looked for. Resolved
// through os.UserConfigDir so the location is the platform's own convention
// rather than a Unix path that happens to work on Windows too.
func UserDefaultsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vaadin-init", "defaults.toml"), nil
}

// LoadDefaults decodes the embedded defaults, then layers a user file over them.
//
// The layering is what makes a personal file able to name only what it changes:
// the decoder writes only the keys a document actually contains, so anything left
// out keeps the value the embedded copy gave it.
//
// An explicit path is an instruction, so a missing file there is an error. The
// per-user path is a convention, so a missing file there is the normal case.
func LoadDefaults(embedded []byte, explicitPath string) (Defaults, error) {
	var d Defaults
	if err := toml.Unmarshal(embedded, &d); err != nil {
		return d, fmt.Errorf("the built-in defaults are not valid TOML: %w", err)
	}

	path := explicitPath
	if path == "" {
		userPath, err := UserDefaultsPath()
		if err != nil {
			return d, nil
		}
		if _, err := os.Stat(userPath); err != nil {
			return d, nil
		}
		path = userPath
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return d, fmt.Errorf("reading defaults from %s: %w", path, err)
	}
	if err := toml.Unmarshal(content, &d); err != nil {
		return d, fmt.Errorf("parsing defaults from %s: %w", path, err)
	}
	return d, nil
}

// ToConfig turns the defaults into a Config with every derived field filled in,
// which is the value the prompts are seeded from and the value flags override.
// Running it before the TUI rather than inside it is what lets --yes skip the
// TUI entirely and still produce exactly what the TUI would have offered.
func (d Defaults) ToConfig() Config {
	c := Config{
		GroupID:       d.GroupID,
		ArtifactID:    d.ArtifactID,
		Description:   d.Description,
		JavaVersion:   d.JavaVersion,
		VaadinVersion: d.VaadinVersion,
		BootVersion:   d.BootVersion,
		Database:      d.Features.Database,
		Auth:          d.Features.Auth,
		E2E:           d.Features.E2E,
		Coverage:      d.Features.Coverage,
		Traceable:     d.Features.Traceable,
	}
	c.ProjectName = DeriveProjectName(c.ArtifactID)
	c.Package = DerivePackage(c.GroupID, c.ArtifactID)
	c.OutputDir = c.ArtifactID
	return c
}

// DeriveProjectName turns an artifact id into the name a human would write:
// "book-harbor" becomes "Book Harbor". It is only ever a default, so it can be
// wrong about an acronym without costing anything — the prompt is right there.
func DeriveProjectName(artifactID string) string {
	words := strings.FieldsFunc(artifactID, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// DerivePackage appends the artifact id to the group id, with the characters a
// package cannot carry removed rather than replaced: "my-app" under "com.example"
// gives com.example.myapp, not com.example.my_app, because the hyphen is a
// word separator in a coordinate and not part of the name.
func DerivePackage(groupID, artifactID string) string {
	var cleaned strings.Builder
	for _, r := range strings.ToLower(artifactID) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cleaned.WriteRune(r)
		}
	}
	suffix := cleaned.String()
	if suffix == "" {
		return groupID
	}
	// A group id that already ends in the artifact's name would otherwise
	// produce com.example.myapp.myapp, which nobody wants and everybody then
	// has to edit.
	if strings.HasSuffix(groupID, "."+suffix) || groupID == suffix {
		return groupID
	}
	return groupID + "." + suffix
}
