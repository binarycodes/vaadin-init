// Package versions looks up the current releases of Vaadin and Spring Boot, and
// the Spring Boot a Vaadin release was built with.
//
// A bootstrap tool's headline default is the framework version, and a hard-coded
// one is wrong the week after it ships — the tool then quietly seeds every new
// project with a stale release. So the numbers are read from Maven Central at
// startup, and the defaults file is the fallback rather than the source.
//
// Everything here degrades instead of failing. There is no answer this package
// can give that is worth making the user wait for, or worth refusing to generate
// a project over, so a lookup that does not work out in a couple of seconds is
// simply not used.
package versions

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/binarycodes/vaadin-init/internal/version"
)

const (
	vaadinBOM  = "https://repo1.maven.org/maven2/com/vaadin/vaadin-bom/maven-metadata.xml"
	bootParent = "https://repo1.maven.org/maven2/org/springframework/boot/spring-boot-starter-parent/maven-metadata.xml"

	// The pom of Vaadin's own Spring Boot starter, one per Vaadin release. It
	// declares the Boot the release was built against as a literal dependency
	// version, which is how the generator at start.vaadin.com picks Boot too.
	vaadinStarter = "https://repo1.maven.org/maven2/com/vaadin/vaadin-spring-boot-starter/%s/vaadin-spring-boot-starter-%s.pom"

	bootGroup = "org.springframework.boot"
)

// metadata is the part of maven-metadata.xml worth reading.
type metadata struct {
	Versioning struct {
		Versions []string `xml:"versions>version"`
	} `xml:"versioning"`
}

// pom is the part of a starter pom worth reading: what it depends on.
type pom struct {
	Dependencies []struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
	} `xml:"dependencies>dependency"`
}

// Lines is which release lines to keep: the ones this tool has templates for,
// as the compatibility rules name them — "25" for Vaadin, "4" for Boot.
type Lines struct {
	Vaadin []string
	Boot   []string
}

// Available is what a lookup found: every release of the lines asked for,
// newest first. The caller decides how many to offer.
type Available struct {
	Vaadin []string
	Boot   []string
}

// Latest returns the newest release in a list, or "" for an empty one.
func Latest(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}

// Lookup fetches both version lists. The two requests run together because they
// are independent and the user is waiting on the pair, not on either one.
//
// It returns whatever it managed to get: a nil error with an empty list is a
// normal outcome, meaning the caller should keep the default it already has.
func Lookup(ctx context.Context, client *http.Client, lines Lines) Available {
	return lookup(ctx, client, vaadinBOM, bootParent, lines)
}

// lookup is Lookup with the documents named, so that a test can point it
// somewhere other than Maven Central.
func lookup(ctx context.Context, client *http.Client, vaadinURL, bootURL string, lines Lines) Available {
	vaadinCh := make(chan []string, 1)
	bootCh := make(chan []string, 1)

	go func() {
		list, _ := stableVersions(ctx, client, vaadinURL, lines.Vaadin)
		vaadinCh <- list
	}()
	go func() {
		list, _ := stableVersions(ctx, client, bootURL, lines.Boot)
		bootCh <- list
	}()

	return Available{
		Vaadin: <-vaadinCh,
		Boot:   <-bootCh,
	}
}

// stableVersions reads a maven-metadata.xml and returns the release versions in
// the given lines, newest first.
func stableVersions(ctx context.Context, client *http.Client, url string, lines []string) ([]string, error) {
	body, err := fetch(ctx, client, url)
	if err != nil {
		return nil, err
	}

	var parsed metadata
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	var stable []version.Version
	for _, raw := range parsed.Versioning.Versions {
		v, ok := version.Parse(raw)
		if !ok || !v.Stable() || !inLines(v, lines) {
			continue
		}
		stable = append(stable, v)
	}

	// Newest first, by number rather than by string: the metadata is roughly in
	// release order, but "25.1.10" sorts before "25.1.9" as text, so trusting
	// either the file's order or a lexical sort offers the wrong release.
	sort.Slice(stable, func(i, j int) bool { return stable[i].After(stable[j]) })

	list := make([]string, 0, len(stable))
	for _, v := range stable {
		list = append(list, v.Raw)
	}
	return list, nil
}

func inLines(v version.Version, lines []string) bool {
	for _, line := range lines {
		if v.InLine(line) {
			return true
		}
	}
	return false
}

// PinnedBoot is the Spring Boot release a Vaadin release was built with, read
// from its starter pom. Empty, with no error, when Maven Central has no such pom
// — a version that was typed rather than offered — since that is not a failure
// the caller can do anything about beyond falling back to the rules.
func PinnedBoot(ctx context.Context, client *http.Client, vaadin string) (string, error) {
	return pinnedBoot(ctx, client, fmt.Sprintf(vaadinStarter, vaadin, vaadin))
}

func pinnedBoot(ctx context.Context, client *http.Client, url string) (string, error) {
	body, err := fetch(ctx, client, url)
	if err != nil {
		if errors.Is(err, errNotFound) {
			return "", nil
		}
		return "", err
	}

	var parsed pom
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	// Matched on the group and the starter's prefix rather than its full name:
	// the web starter is spring-boot-starter-web on Boot 3 and -webmvc on Boot 4,
	// and either is the Boot the release was built with.
	for _, d := range parsed.Dependencies {
		if d.GroupID == bootGroup && strings.HasPrefix(d.ArtifactID, "spring-boot-starter") && d.Version != "" {
			return d.Version, nil
		}
	}
	return "", fmt.Errorf("%s: no %s dependency with a version", url, bootGroup)
}

var errNotFound = errors.New("not found")

// fetch reads one document. Capped: the length is untrusted, coming from the
// network, and the real documents are at most tens of kilobytes.
func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: %w", url, errNotFound)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, response.Status)
	}
	return io.ReadAll(io.LimitReader(response.Body, 4<<20))
}
