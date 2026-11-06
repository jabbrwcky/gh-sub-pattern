package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/alecthomas/kong"
	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/cli/go-gh/v2/pkg/prompter"
	"github.com/cli/go-gh/v2/pkg/repository"
	"github.com/cli/go-gh/v2/pkg/term"
)

type CLI struct {
	Repo        string   `arg:"" optional:"" help:"Repository as OWNER/REPO. Defaults to the current repository."`
	Environment []string `short:"e" sep:"none" help:"Deployment environment name (repeatable)."`
	Ref         []string `short:"r" sep:"none" help:"Exact branch (main), full ref (refs/tags/v1.0), or pull_request (repeatable)."`
	Refs        []string `sep:"none" help:"Branch prefix, '*' appended: release/ -> refs/heads/release/* (repeatable)."`
	Tag         []string `short:"t" sep:"none" help:"Exact tag name (repeatable)."`
	Tags        []string `sep:"none" help:"Tag prefix, '*' appended: v1. -> refs/tags/v1.* (repeatable)."`
	Mutable     bool     `help:"Use the legacy OWNER/REPO segment instead of OWNER@ID/REPO@ID. Ignored when the repo reports its subject format."`
}

type repoInfo struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Owner struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	} `json:"owner"`
}

// Swapped in tests.
var (
	newClient           = func(host string) (*api.RESTClient, error) { return api.NewRESTClient(api.ClientOptions{Host: host}) }
	stdout    io.Writer = os.Stdout
)

func main() {
	var cli CLI
	kong.Parse(&cli,
		kong.Name("gh sub-pattern"),
		kong.Description("Print GitHub Actions OIDC token 'sub' claim patterns, one per line. Prompts when no target flag is given."))
	if err := run(cli); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cli CLI) error {
	repo, err := resolveRepo(cli.Repo)
	if err != nil {
		return err
	}
	client, err := newClient(repo.Host)
	if err != nil {
		return err
	}
	// Also yields canonical casing; trust policies compare case-sensitively.
	var info repoInfo
	if err := client.Get("repos/"+repo.Owner+"/"+repo.Name, &info); err != nil {
		return err
	}
	fullName := info.Owner.Login + "/" + info.Name
	immutable := !cli.Mutable
	// The repo's own setting decides; the flag is only a fallback.
	if detected := subFormat(client, fullName); detected != nil {
		if *detected != immutable {
			fmt.Fprintf(os.Stderr, "note: %s reports immutable subjects=%t; using that\n", fullName, *detected)
		}
		immutable = *detected
	}

	subs := suffixes(cli)
	if len(subs) == 0 {
		// Never fall back to a permissive repo:O/R:* silently.
		if !term.IsTerminal(os.Stdin) {
			return errors.New("no --environment, --ref(s) or --tag(s) given and stdin is not a terminal")
		}
		choices, err := pick(repo.Host, fullName)
		if err != nil {
			return err
		}
		for _, c := range choices {
			suffix, err := parseChoice(c)
			if err != nil {
				return err
			}
			subs = append(subs, suffix)
		}
	}
	for _, s := range subs {
		fmt.Fprintln(stdout, "repo:"+repoSegment(info, immutable)+":"+s)
	}
	return nil
}

func repoSegment(r repoInfo, immutable bool) string {
	if !immutable {
		return r.Owner.Login + "/" + r.Name
	}
	return fmt.Sprintf("%s@%d/%s@%d", r.Owner.Login, r.Owner.ID, r.Name, r.ID)
}

// suffixes builds one sub suffix per flag value, in flag order.
func suffixes(cli CLI) []string {
	var out []string
	for _, e := range cli.Environment {
		out = append(out, "environment:"+e)
	}
	for _, r := range cli.Ref {
		out = append(out, refSuffix(r))
	}
	for _, r := range cli.Refs {
		out = append(out, refSuffix(r)+"*")
	}
	for _, t := range cli.Tag {
		out = append(out, "ref:refs/tags/"+t)
	}
	for _, t := range cli.Tags {
		out = append(out, "ref:refs/tags/"+t+"*")
	}
	return out
}

func resolveRepo(s string) (repository.Repository, error) {
	if s == "" {
		return repository.Current()
	}
	return repository.Parse(s)
}

// refSuffix turns a user-supplied ref or glob into the sub suffix of the default template.
func refSuffix(ref string) string {
	switch {
	case ref == "pull_request", ref == "*":
		return ref
	case strings.HasPrefix(ref, "refs/heads/"), strings.HasPrefix(ref, "refs/tags/"):
		return "ref:" + ref
	default:
		return "ref:refs/heads/" + ref
	}
}

// parseChoice accepts a picked candidate as-is and treats free text as a ref.
func parseChoice(s string) (string, error) {
	if s == "environment:" || s == "ref:" {
		return "", fmt.Errorf("%q needs a name after the colon", s)
	}
	if strings.HasPrefix(s, "environment:") || strings.HasPrefix(s, "ref:") {
		return s, nil
	}
	return refSuffix(s), nil
}

// candidates lists sub suffixes: environments, pull_request, refs and every parent-path glob.
func candidates(envs, branches, tags []string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, e := range envs {
		add("environment:" + e)
	}
	add("pull_request")
	for i, names := range [][]string{branches, tags} {
		prefix := []string{"refs/heads/", "refs/tags/"}[i]
		add("ref:" + prefix + "*")
		for _, n := range names {
			for i := strings.LastIndex(n, "/"); i > 0; i = strings.LastIndex(n[:i], "/") {
				add("ref:" + prefix + n[:i] + "/*")
			}
			add("ref:" + prefix + n)
		}
	}
	return out
}

func pick(host, fullName string) ([]string, error) {
	list := func(path, jq string) []string {
		out, _, err := gh.Exec("api", "--hostname", host, "--paginate", "repos/"+fullName+"/"+path, "--jq", jq)
		if err != nil {
			return nil // e.g. no environments; still offer the rest
		}
		return strings.Fields(out.String())
	}
	items := candidates(
		list("environments", ".environments[].name"),
		list("branches", ".[].name"),
		list("tags", ".[].name"),
	)
	if _, err := exec.LookPath("fzf"); err == nil {
		return fzf(items)
	}
	// shortcut: no free-text glob without fzf; add prompter.Input if non-fzf users need it.
	idxs, err := prompter.New(os.Stdin, os.Stderr, os.Stderr).MultiSelect("sub for "+fullName, nil, items)
	if err != nil {
		return nil, err
	}
	if len(idxs) == 0 {
		return nil, errors.New("nothing selected")
	}
	picked := make([]string, len(idxs))
	for i, idx := range idxs {
		picked[i] = items[idx]
	}
	return picked, nil
}

func fzf(items []string) ([]string, error) {
	cmd := exec.Command("fzf", "--print-query", "--exact", "--multi", "--prompt", "sub> ",
		"--header", "Tab marks several candidates; or type a ref/glob (e.g. release/*). Enter to finish")
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	var exit *exec.ExitError
	noMatch := errors.As(err, &exit) && exit.ExitCode() == 1
	switch {
	case err == nil && len(lines) > 1:
		return lines[1:], nil
	case (err == nil || noMatch) && lines[0] != "":
		return lines[:1], nil // free-text query
	default:
		return nil, errors.New("selection cancelled")
	}
}

// subFormat reads the repo's sub template; it returns nil when the subject format is unknown.
func subFormat(client *api.RESTClient, fullName string) (immutable *bool) {
	var c struct {
		UseDefault          bool     `json:"use_default"`
		UseImmutableSubject *bool    `json:"use_immutable_subject"`
		IncludeClaimKeys    []string `json:"include_claim_keys"`
	}
	// Errors ignored: token may lack scope to read the template.
	if err := client.Get("repos/"+fullName+"/actions/oidc/customization/sub", &c); err != nil {
		return nil
	}
	if !c.UseDefault {
		fmt.Fprintf(os.Stderr, "warning: %s uses a custom sub template (%s); the default-format patterns below will not match\n",
			fullName, strings.Join(c.IncludeClaimKeys, ", "))
	}
	return c.UseImmutableSubject
}
