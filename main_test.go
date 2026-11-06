package main

import (
	"slices"
	"testing"
)

func TestParseChoice(t *testing.T) {
	for in, want := range map[string]string{
		"main":                  "ref:refs/heads/main",
		"release/*":             "ref:refs/heads/release/*",
		"tags/foo":              "ref:refs/heads/tags/foo",
		"refs/tags/v*":          "ref:refs/tags/v*",
		"refs/tags/v1.2":        "ref:refs/tags/v1.2",
		"refs/heads/x":          "ref:refs/heads/x",
		"refs/foo":              "ref:refs/heads/refs/foo",
		"pull_request":          "pull_request",
		"*":                     "*",
		"environment:prod":      "environment:prod",
		"ref:refs/heads/main":   "ref:refs/heads/main",
		"ref:refs/heads/feat/*": "ref:refs/heads/feat/*",
	} {
		if got, err := parseChoice(in); err != nil || got != want {
			t.Errorf("parseChoice(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"environment:", "ref:"} {
		if _, err := parseChoice(in); err == nil {
			t.Errorf("parseChoice(%q): expected error", in)
		}
	}
}

func TestCandidates(t *testing.T) {
	got := candidates([]string{"prod"}, []string{"main", "release/v1/hotfix"}, []string{"v1.0"})
	for _, want := range []string{
		"environment:prod", "pull_request",
		"ref:refs/heads/*", "ref:refs/heads/main",
		"ref:refs/heads/release/*", "ref:refs/heads/release/v1/*", "ref:refs/heads/release/v1/hotfix",
		"ref:refs/tags/*", "ref:refs/tags/v1.0",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("candidates missing %q in %v", want, got)
		}
	}
	if len(got) != 9 {
		t.Errorf("want 9 unique candidates, got %d: %v", len(got), got)
	}
}

func TestSuffixes(t *testing.T) {
	got := suffixes(CLI{
		Environment: []string{"prod"},
		Ref:         []string{"main", "pull_request"},
		Refs:        []string{"release/"},
		Tag:         []string{"v1.0"},
		Tags:        []string{"v2."},
	})
	want := []string{
		"environment:prod",
		"ref:refs/heads/main", "pull_request",
		"ref:refs/heads/release/*",
		"ref:refs/tags/v1.0",
		"ref:refs/tags/v2.*",
	}
	if !slices.Equal(got, want) {
		t.Errorf("suffixes = %v, want %v", got, want)
	}
}

func TestRepoSegment(t *testing.T) {
	var r repoInfo
	r.ID, r.Name, r.Owner.ID, r.Owner.Login = 456789, "octo-repo", 123456, "octo-org"
	if got := repoSegment(r, true); got != "octo-org@123456/octo-repo@456789" {
		t.Errorf("immutable = %q", got)
	}
	if got := repoSegment(r, false); got != "octo-org/octo-repo" {
		t.Errorf("legacy = %q", got)
	}
}
