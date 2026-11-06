package main

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

// routes maps API paths to JSON bodies; unknown paths return 404.
type routes map[string]string

func (r routes) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := r[req.URL.Path]
	status := http.StatusOK
	if !ok {
		status, body = http.StatusNotFound, `{"message":"Not Found"}`
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

func stubAPI(t *testing.T, r routes) *bytes.Buffer {
	t.Helper()
	oldClient, oldOut := newClient, stdout
	t.Cleanup(func() { newClient, stdout = oldClient, oldOut })
	newClient = func(host string) (*api.RESTClient, error) {
		return api.NewRESTClient(api.ClientOptions{Host: host, AuthToken: "test", Transport: r})
	}
	var buf bytes.Buffer
	stdout = &buf
	return &buf
}

// fakeBin puts an executable shell script named name first on PATH and returns its path.
func fakeBin(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path
}

const (
	repoJSON = `{"id":456789,"name":"octo-repo","owner":{"id":123456,"login":"octo-org"}}`
	subPath  = "/repos/octo-org/octo-repo/actions/oidc/customization/sub"
)

func TestRun(t *testing.T) {
	for name, tc := range map[string]struct {
		cli  CLI
		sub  string // customization response, "" means 404
		want string
	}{
		"immutable by default, canonical casing": {
			CLI{Repo: "OCTO-ORG/OCTO-REPO", Ref: []string{"main"}}, "",
			"repo:octo-org@123456/octo-repo@456789:ref:refs/heads/main\n",
		},
		"mutable flag": {
			CLI{Ref: []string{"main"}, Mutable: true}, "",
			"repo:octo-org/octo-repo:ref:refs/heads/main\n",
		},
		"repo reports mutable": {
			CLI{Tags: []string{"v"}}, `{"use_default":true,"use_immutable_subject":false}`,
			"repo:octo-org/octo-repo:ref:refs/tags/v*\n",
		},
		"repo reports immutable over flag": {
			CLI{Environment: []string{"prod"}, Mutable: true}, `{"use_default":true,"use_immutable_subject":true}`,
			"repo:octo-org@123456/octo-repo@456789:environment:prod\n",
		},
		"field absent falls back to flag, custom template": {
			CLI{Ref: []string{"pull_request"}, Tag: []string{"v1"}, Mutable: true}, `{"use_default":false,"include_claim_keys":["repo","context"]}`,
			"repo:octo-org/octo-repo:pull_request\nrepo:octo-org/octo-repo:ref:refs/tags/v1\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := routes{"/repos/octo-org/octo-repo": repoJSON, "/repos/OCTO-ORG/OCTO-REPO": repoJSON}
			if tc.sub != "" {
				r[subPath] = tc.sub
			}
			out := stubAPI(t, r)
			if tc.cli.Repo == "" {
				tc.cli.Repo = "octo-org/octo-repo"
			}
			if err := run(tc.cli); err != nil {
				t.Fatal(err)
			}
			if out.String() != tc.want {
				t.Errorf("got %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestRunErrors(t *testing.T) {
	stubAPI(t, routes{"/repos/octo-org/octo-repo": repoJSON})
	for name, cli := range map[string]CLI{
		"invalid repo":           {Repo: "a/b/c/d", Ref: []string{"main"}},
		"unknown repo":           {Repo: "octo-org/missing", Ref: []string{"main"}},
		"no target, no terminal": {Repo: "octo-org/octo-repo"},
	} {
		if err := run(cli); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestResolveRepoCurrent(t *testing.T) {
	t.Setenv("GH_REPO", "octo-org/octo-repo")
	r, err := resolveRepo("")
	if err != nil || r.Owner != "octo-org" || r.Name != "octo-repo" {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestFzf(t *testing.T) {
	for name, tc := range map[string]struct {
		script  string
		want    []string
		wantErr bool
	}{
		"selection": {`printf 'ma\nref:refs/heads/main\n'`, []string{"ref:refs/heads/main"}, false},
		"multi":     {`printf '\npull_request\nref:refs/tags/*\n'`, []string{"pull_request", "ref:refs/tags/*"}, false},
		"free text": {`printf 'release/2.x/*\n'; exit 1`, []string{"release/2.x/*"}, false},
		"cancelled": {`exit 130`, nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			fakeBin(t, "fzf", "cat >/dev/null\n"+tc.script)
			got, err := fzf([]string{"ref:refs/heads/main"})
			if (err != nil) != tc.wantErr || !slices.Equal(got, tc.want) {
				t.Errorf("got %q, %v", got, err)
			}
		})
	}
}

func TestPick(t *testing.T) {
	t.Setenv("GH_PATH", fakeBin(t, "gh", `case "$*" in
*environments*) exit 1 ;;
*branches*) printf 'main\nrelease/v1\n' ;;
*tags*) echo v1.0 ;;
esac`))
	cands := filepath.Join(t.TempDir(), "candidates")
	fakeBin(t, "fzf", `cat > "`+cands+`"; printf '\nref:refs/heads/release/*\n'`)

	got, err := pick("github.com", "octo-org/octo-repo")
	if err != nil || !slices.Equal(got, []string{"ref:refs/heads/release/*"}) {
		t.Fatalf("got %q, %v", got, err)
	}
	listed, _ := os.ReadFile(cands)
	for _, want := range []string{"pull_request", "ref:refs/heads/release/*", "ref:refs/tags/v1.0"} {
		if !strings.Contains(string(listed)+"\n", want+"\n") {
			t.Errorf("fzf input missing %q:\n%s", want, listed)
		}
	}
}
