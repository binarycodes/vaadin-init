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

	got, err := stableVersions(context.Background(), server.Client(), server.URL, 25)
	if err != nil {
		t.Fatalf("stableVersions: %v", err)
	}

	// Newest first. Truncated to what the tool offers rather than repeating that
	// number here, so changing it is one edit and this test keeps testing order.
	want := []string{"25.2.6", "25.2.5", "25.2.0", "25.1.11", "25.1.10", "25.1.9"}
	if len(want) > offered {
		want = want[:offered]
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestStableVersionsCapsTheList(t *testing.T) {
	var many []string
	for patch := 0; patch < offered*3; patch++ {
		many = append(many, fmt.Sprintf("25.1.%d", patch))
	}
	server := serve(t, http.StatusOK, metadataDocument(many...))

	got, err := stableVersions(context.Background(), server.Client(), server.URL, 25)
	if err != nil {
		t.Fatalf("stableVersions: %v", err)
	}
	if len(got) != offered {
		t.Fatalf("got %d versions, want the list capped at %d", len(got), offered)
	}
	if got[0] != fmt.Sprintf("25.1.%d", offered*3-1) {
		t.Errorf("the newest release should be first, got %q", got[0])
	}
}

func TestStableVersionsReportsAnErrorStatus(t *testing.T) {
	server := serve(t, http.StatusInternalServerError, "nope")
	if _, err := stableVersions(context.Background(), server.Client(), server.URL, 25); err == nil {
		t.Fatal("a 500 should be an error")
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

	if _, err := stableVersions(context.Background(), client, server.URL, 25); err == nil {
		t.Fatal("an unreachable host should be an error at this level")
	}

	// Lookup swallows that error, because there is nothing the caller can do
	// with it that is better than keeping the default it already has.
	available := lookup(context.Background(), client, server.URL, server.URL)
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

func TestParseVersion(t *testing.T) {
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
		v, ok := parseVersion(c.raw)
		if ok != c.ok {
			t.Errorf("parseVersion(%q) ok = %v, want %v", c.raw, ok, c.ok)
			continue
		}
		if ok && v.qualifier != c.qualifier {
			t.Errorf("parseVersion(%q) qualifier = %q, want %q", c.raw, v.qualifier, c.qualifier)
		}
	}
}
