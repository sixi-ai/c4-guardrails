// Command c4g checks C4 diagrams (Mermaid C4 or JSON) against deterministic
// architecture guardrails and cites EU regulatory clauses.
//
// Exit codes: 0 no finding at or above -fail-on, 1 findings at or above
// -fail-on, 2 usage, input or output error.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/rbrus/c4-guardrails/internal/clauses"
	"github.com/rbrus/c4-guardrails/internal/model"
	"github.com/rbrus/c4-guardrails/internal/report"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

// version is set with -ldflags "-X main.version=..." for releases.
var version = ""

// baseVersion is reported for local builds that carry no release tag.
const baseVersion = "0.1.0"

var releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func toolVersion() string {
	if version != "" {
		return version
	}
	// `go install ...@v0.1.0` stamps the module version; local builds stamp a
	// pseudo-version, which we do not print.
	if bi, ok := debug.ReadBuildInfo(); ok && releaseTag.MatchString(bi.Main.Version) {
		return strings.TrimPrefix(bi.Main.Version, "v")
	}
	return baseVersion
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `c4g: deterministic architecture guardrails for C4 diagrams, with EU clause citations.

Usage:
  c4g [flags] <file.mmd|file.c4.json|file.md>...

Flags:
  -json            print JSON instead of the table
  -sarif FILE      write SARIF 2.1.0 to FILE ("-" for stdout)
  -md FILE         write a Markdown summary to FILE ("-" for stdout)
  -html FILE       write a self-contained HTML report to FILE
  -fail-on LEVEL   exit 1 when a finding is at or above LEVEL:
                   critical, high (default), medium, low, info, none
  -rules           list the rules and the clauses they cite
  -version         print the version

Exit codes: 0 pass, 1 findings at or above -fail-on, 2 usage or input error.
Assesses what a diagram declares; not a compliance certification.
`

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("c4g", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "")
	sarifPath := fs.String("sarif", "", "")
	mdPath := fs.String("md", "", "")
	htmlPath := fs.String("html", "", "")
	failOn := fs.String("fail-on", "high", "")
	listRules := fs.Bool("rules", false, "")
	showVersion := fs.Bool("version", false, "")
	help := fs.Bool("h", false, "")
	fs.BoolVar(help, "help", false, "")

	// Allow flags after file names: parse, take one positional, repeat.
	var files []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			fmt.Fprintf(stderr, "c4g: %s\n\n%s", report.OneLine(err.Error(), 200), usage)
			return 2
		}
		rest = fs.Args()
		if len(rest) == 0 {
			break
		}
		files = append(files, rest[0])
		rest = rest[1:]
	}
	if *help {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if *showVersion {
		fmt.Fprintf(stdout, "c4g %s\n", toolVersion())
		return 0
	}
	if *listRules {
		printRules(stdout)
		return 0
	}
	threshold, ok := rules.ParseSeverity(*failOn)
	if !ok {
		fmt.Fprintf(stderr, "c4g: invalid -fail-on %q (critical, high, medium, low, info, none)\n", report.OneLine(*failOn, 20))
		return 2
	}
	if len(files) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	res := &report.Result{Version: toolVersion(), FailOn: threshold}
	for _, f := range files {
		m, err := load(f)
		if err != nil {
			fmt.Fprintf(stderr, "c4g: %s\n", report.OneLine(err.Error(), 300))
			return 2
		}
		res.Models = append(res.Models, m)
		res.Findings = append(res.Findings, rules.Evaluate(m)...)
	}

	var err error
	if *jsonOut {
		err = report.WriteJSON(stdout, res)
	} else {
		err = report.WriteTable(stdout, res)
	}
	if err == nil && *sarifPath != "" {
		err = writeTo(*sarifPath, stdout, func(w io.Writer) error { return report.WriteSARIF(w, res) })
	}
	if err == nil && *mdPath != "" {
		err = writeTo(*mdPath, stdout, func(w io.Writer) error { return report.WriteMarkdown(w, res) })
	}
	if err == nil && *htmlPath != "" {
		err = writeTo(*htmlPath, stdout, func(w io.Writer) error { return report.WriteHTML(w, res) })
	}
	if err != nil {
		fmt.Fprintf(stderr, "c4g: %s\n", report.OneLine(err.Error(), 300))
		return 2
	}
	if res.Failing() > 0 {
		return 1
	}
	return 0
}

func load(path string) (*model.Model, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s: refusing to follow a symlink", path)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: not a regular file", path)
	}
	if fi.Size() > model.MaxInputBytes {
		return nil, fmt.Errorf("%s: larger than 2 MiB", path)
	}
	f, err := os.Open(path) // #nosec G304 -- the user names the file to check
	if err != nil {
		return nil, err
	}
	defer f.Close()
	src, err := io.ReadAll(io.LimitReader(f, model.MaxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(src) > model.MaxInputBytes {
		return nil, fmt.Errorf("%s: larger than 2 MiB", path)
	}
	return model.Parse(filepath.ToSlash(path), src)
}

// writeTo renders into memory first so a failed render never leaves a
// half-written file.
func writeTo(path string, stdout io.Writer, render func(io.Writer) error) error {
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		return err
	}
	if path == "-" {
		_, err := stdout.Write(buf.Bytes())
		return err
	}
	if path == "" {
		return errors.New("empty output path")
	}
	return os.WriteFile(path, buf.Bytes(), 0o644) // #nosec G306 -- reports are meant to be shared
}

func printRules(w io.Writer) {
	all := rules.All()
	ids := make([]string, 0, len(all))
	for _, r := range all {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r, _ := rules.ByID(id)
		fmt.Fprintf(w, "%s  %-8s %s\n", r.ID, r.Severity, r.Title)
		if len(r.Clauses) == 0 {
			fmt.Fprintf(w, "        %s\n", clauses.NoClause)
		}
		for _, c := range r.Clauses {
			cl, _ := clauses.Lookup(c)
			fmt.Fprintf(w, "        %s  %s\n", c, cl.Title)
		}
	}
	fmt.Fprintf(w, "\nClause references: %s\n", strings.TrimSpace(clauses.Attribution()))
}
