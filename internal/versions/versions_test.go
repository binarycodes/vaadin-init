package versions

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func metadataDocument(versions ...string) string {
	var body strings.Builder
	body.WriteString(`<metadata><versioning><versions>`)
	for _, v := range versions {
		fmt.Fprintf(&body, "<version>%s</version>", v)
	}
	body.WriteString(`</versions></versioning></metadata>`)
	return body.String()
}

func serve(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestStableVersionsSortsNumericallyAndDropsPreReleases(t *testing.T) {
	// Deliberately in the order the real document has them, with 25.1.10 before
	// 25.1.9 — the case a lexical sort gets wrong.
	server := serve(t, http.StatusOK, metadataDocument(
		"24.9.1",
		"25.1.9", "25.1.10", "25.1.11",
		"25.2.0-beta1", "25.2.0-rc1", "25.2.0",
		"25.2.5", "25.2.6",
		"25.3.0-alpha2",
	))

	got, err := stableVersions(context.Background(), server.Client(), server.URL, []string{"25"})
	if err != nil {
		t.Fatalf("stableVersions: %v", err)
	}

	// Newest first, and every release of the line: how many to offer is the
	// caller's decision.
	want := "25.2.6 25.2.5 25.2.0 25.1.11 25.1.10 25.1.9"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// The lines come from the compatibility rules, and a lookup asked for two lines
// keeps both.
func TestStableVersionsKeepsEveryLineAskedFor(t *testing.T) {
	server := serve(t, http.StatusOK, metadataDocument("3.5.15", "4.0.8", "4.1.1", "5.0.0"))

	got, err := stableVersions(context.Background(), server.Client(), server.URL, []string{"4", "3.5"})
	if err != nil {
		t.Fatalf("stableVersions: %v", err)
	}
	if want := "4.1.1 4.0.8 3.5.15"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %v", got, want)
	}

	// No lines named: every release, for the check that looks for lines the
	// rules have not heard of.
	got, err = stableVersions(context.Background(), server.Client(), server.URL, nil)
	if err != nil {
		t.Fatalf("stableVersions: %v", err)
	}
	if want := "5.0.0 4.1.1 4.0.8 3.5.15"; strings.Join(got, " ") != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStableVersionsReportsAnErrorStatus(t *testing.T) {
	server := serve(t, http.StatusInternalServerError, "nope")
	if _, err := stableVersions(context.Background(), server.Client(), server.URL, []string{"25"}); err == nil {
		t.Fatal("a 500 should be an error")
	}
}

// Two captured starter poms, cut down to the dependency that matters: Vaadin 24
// depends on spring-boot-starter-web, Vaadin 25 on spring-boot-starter-webmvc.
func starterPom(artifact, boot string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.vaadin</groupId>
  <artifactId>vaadin-spring-boot-starter</artifactId>
  <dependencies>
    <dependency>
      <groupId>com.vaadin</groupId>
      <artifactId>vaadin-spring</artifactId>
      <version>25.2.6</version>
    </dependency>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>` + artifact + `</artifactId>
      <version>` + boot + `</version>
    </dependency>
  </dependencies>
</project>`
}

func TestPinnedBootReadsTheStarterPom(t *testing.T) {
	for _, c := range []struct{ artifact, boot string }{
		{"spring-boot-starter-web", "3.5.15"},
		{"spring-boot-starter-webmvc", "4.1.0"},
	} {
		server := serve(t, http.StatusOK, starterPom(c.artifact, c.boot))
		got, err := pinnedBoot(context.Background(), server.Client(), server.URL)
		if err != nil {
			t.Fatalf("pinnedBoot(%s): %v", c.artifact, err)
		}
		if got != c.boot {
			t.Errorf("pinnedBoot(%s) = %q, want %q", c.artifact, got, c.boot)
		}
	}
}

// A release Maven Central has no starter for — one that was typed — is not a
// failure; there is simply no pin, and the caller falls back to the rules.
func TestPinnedBootIsEmptyForAnUnknownRelease(t *testing.T) {
	server := serve(t, http.StatusNotFound, "not here")
	got, err := pinnedBoot(context.Background(), server.Client(), server.URL)
	if err != nil || got != "" {
		t.Errorf("pinnedBoot = %q, %v; want empty and no error", got, err)
	}

	server = serve(t, http.StatusOK, `<project><dependencies/></project>`)
	if _, err := pinnedBoot(context.Background(), server.Client(), server.URL); err == nil {
		t.Error("a pom with no Boot dependency should be an error")
	}
}

// The tool must generate a project whether or not Maven Central answers, so an
// unreachable host has to come back as an empty list rather than as a failure.
//
// A server that has already been shut down, rather than a timeout against a real
// address: the outcome is the same and this test needs no network and no waiting.
func TestLookupDegradesWhenUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := server.Client()
	server.Close()

	if _, err := stableVersions(context.Background(), client, server.URL, []string{"25"}); err == nil {
		t.Fatal("an unreachable host should be an error at this level")
	}

	// Lookup swallows that error, because there is nothing the caller can do
	// with it that is better than keeping the default it already has.
	available := lookup(context.Background(), client, server.URL, server.URL, Lines{Vaadin: []string{"25"}, Boot: []string{"4"}})
	if len(available.Vaadin) != 0 || len(available.Boot) != 0 {
		t.Errorf("expected empty lists from a failed lookup, got %+v", available)
	}
}

func TestLatest(t *testing.T) {
	if Latest(nil) != "" {
		t.Error("Latest of an empty list should be empty")
	}
	if got := Latest([]string{"25.2.6", "25.2.5"}); got != "25.2.6" {
		t.Errorf("Latest = %q, want the first entry", got)
	}
}
