// Package versions looks up the current releases of Vaadin and Spring Boot.
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
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	vaadinBOM  = "https://repo1.maven.org/maven2/com/vaadin/vaadin-bom/maven-metadata.xml"
	bootParent = "https://repo1.maven.org/maven2/org/springframework/boot/spring-boot-starter-parent/maven-metadata.xml"

	// The Vaadin generation this tool's templates target. Vaadin 25 is the first
	// to sit on Spring Boot 4, and the two generations differ in ways a single
	// pom template cannot straddle honestly: Boot 4 splits autoconfiguration into
	// a module per technology and renames several starters. Supporting 24 means a
	// second set of templates, not a conditional.
	VaadinMajor = 25

	// The Spring Boot generation that goes with it.
	BootMajor = 4

	// How many releases to offer. Enough to pick the previous patch or an older
	// minor deliberately, few enough that the list fits a short terminal without
	// scrolling — anyone who wants an older release than these can type it.
	offered = 5
)

// metadata is the part of maven-metadata.xml worth reading.
type metadata struct {
	Versioning struct {
		Versions []string `xml:"versions>version"`
	} `xml:"versioning"`
}

// Available is what a lookup found: releases newest first, ready to be offered
// as options.
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
func Lookup(ctx context.Context, client *http.Client) Available {
	return lookup(ctx, client, vaadinBOM, bootParent)
}

// lookup is Lookup with the documents named, so that a test can point it
// somewhere other than Maven Central.
func lookup(ctx context.Context, client *http.Client, vaadinURL, bootURL string) Available {
	vaadinCh := make(chan []string, 1)
	bootCh := make(chan []string, 1)

	go func() {
		list, _ := stableVersions(ctx, client, vaadinURL, VaadinMajor)
		vaadinCh <- list
	}()
	go func() {
		list, _ := stableVersions(ctx, client, bootURL, BootMajor)
		bootCh <- list
	}()

	return Available{
		Vaadin: <-vaadinCh,
		Boot:   <-bootCh,
	}
}

// stableVersions reads a maven-metadata.xml and returns the release versions of
// one major line, newest first.
func stableVersions(ctx context.Context, client *http.Client, url string, major int) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, response.Status)
	}

	// Capped: this is an untrusted length from the network, and the real
	// documents are tens of kilobytes.
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	var parsed metadata
	if err := xml.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	var stable []version
	for _, raw := range parsed.Versioning.Versions {
		v, ok := parseVersion(raw)
		if !ok || v.major != major || v.qualifier != "" {
			continue
		}
		stable = append(stable, v)
	}

	// Newest first, by number rather than by string: the metadata is roughly in
	// release order, but "25.1.10" sorts before "25.1.9" as text, so trusting
	// either the file's order or a lexical sort offers the wrong release.
	sort.Slice(stable, func(i, j int) bool { return stable[i].after(stable[j]) })

	list := make([]string, 0, offered)
	for _, v := range stable {
		if len(list) == offered {
			break
		}
		list = append(list, v.raw)
	}
	return list, nil
}

type version struct {
	raw                 string
	major, minor, patch int
	qualifier           string
}

var versionPattern = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?(?:[-.]([A-Za-z0-9.]+))?$`)

// parseVersion splits a Maven version far enough to compare it and to tell a
// release from a pre-release. Anything it does not recognise is reported as
// unparsed rather than guessed at, and the caller then skips it.
func parseVersion(raw string) (version, bool) {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if match == nil {
		return version{}, false
	}
	number := func(s string) int {
		n, _ := strconv.Atoi(s)
		return n
	}
	return version{
		raw:       raw,
		major:     number(match[1]),
		minor:     number(match[2]),
		patch:     number(match[3]),
		qualifier: match[4],
	}, true
}

func (v version) after(other version) bool {
	if v.major != other.major {
		return v.major > other.major
	}
	if v.minor != other.minor {
		return v.minor > other.minor
	}
	return v.patch > other.patch
}
