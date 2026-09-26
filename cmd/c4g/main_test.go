package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/golden")

func repoRoot(t *testing.T) {
	t.Helper()
	t.Chdir(filepath.Join("..", ".."))
	version = "0.1.0"
	t.Cleanup(func() { version = "" })
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run go test ./cmd/c4g -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from golden file; run go test ./cmd/c4g -update and review the diff", name)
	}
}

func TestGoldenExamples(t *testing.T) {
	repoRoot(t)
	examples := []struct {
		file string
		exit int
	}{
		{"examples/rag-chatbot-bad.mmd", 1},
		{"examples/rag-chatbot-good.mmd", 0},
		{"examples/payments-bad.mmd", 1},
		{"examples/payments-good.mmd", 0},
		{"examples/credit-scoring.c4.json", 1},
	}
	tmp := t.TempDir()
	for _, ex := range examples {
		base := strings.TrimSuffix(strings.TrimSuffix(filepath.Base(ex.file), ".mmd"), ".c4.json")
		t.Run(base, func(t *testing.T) {
			var out, errb bytes.Buffer
			sarif := filepath.Join(tmp, base+".sarif")
			md := filepath.Join(tmp, base+".md")
			html := filepath.Join(tmp, base+".html")
			code := run([]string{ex.file, "-sarif", sarif, "-md", md, "-html", html}, &out, &errb)
			if code != ex.exit {
				t.Fatalf("exit = %d, want %d; stderr: %s", code, ex.exit, errb.String())
			}
			golden(t, base+".txt", out.Bytes())
			for ext, p := range map[string]string{".sarif": sarif, ".md": md, ".html": html} {
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				golden(t, base+ext, b)
			}
			out.Reset()
			if code := run([]string{"-json", ex.file}, &out, &errb); code != ex.exit {
				t.Fatalf("json exit = %d", code)
			}
			golden(t, base+".json", out.Bytes())
		})
	}
}

func TestGoodTwinsHaveNoFindingsAboveInfo(t *testing.T) {
	repoRoot(t)
	for _, f := range []string{"examples/rag-chatbot-good.mmd", "examples/payments-good.mmd"} {
		var out, errb bytes.Buffer
		if code := run([]string{"-fail-on", "low", f}, &out, &errb); code != 0 {
			t.Errorf("%s: exit %d with -fail-on low\n%s", f, code, out.String())
		}
	}
}

func TestExitCodes(t *testing.T) {
	repoRoot(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.mmd")
	if err := os.WriteFile(bad, []byte("C4Context\nPerson(a, \"A"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.mmd")
	symlinkOK := os.Symlink(filepath.Join("..", "..", "examples", "rag-chatbot-bad.mmd"), link) == nil
	cases := []struct {
		name string
		args []string
		exit int
	}{
		{"fail-on none", []string{"-fail-on", "none", "examples/rag-chatbot-bad.mmd"}, 0},
		{"fail-on medium", []string{"examples/rag-chatbot-bad.mmd", "-fail-on", "medium"}, 1},
		{"fail-on critical", []string{"-fail-on=critical", "examples/rag-chatbot-bad.mmd"}, 0},
		{"bad fail-on", []string{"-fail-on", "severe", "examples/rag-chatbot-bad.mmd"}, 2},
		{"no files", nil, 2},
		{"missing file", []string{"nope.mmd"}, 2},
		{"parse error", []string{bad}, 2},
		{"unknown flag", []string{"-frobnicate"}, 2},
		{"directory", []string{dir}, 2},
		{"version", []string{"-version"}, 0},
		{"rules", []string{"-rules"}, 0},
		{"help", []string{"-h"}, 0},
		{"multiple files", []string{"examples/rag-chatbot-good.mmd", "examples/payments-good.mmd"}, 0},
		{"unwritable output", []string{"-sarif", filepath.Join(dir, "missing", "x.sarif"), "examples/rag-chatbot-good.mmd"}, 2},
	}
	if symlinkOK {
		cases = append(cases, struct {
			name string
			args []string
			exit int
		}{"symlink refused", []string{link}, 2})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if code := run(c.args, &out, &errb); code != c.exit {
				t.Errorf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, c.exit, out.String(), errb.String())
			}
		})
	}
}

func TestRulesListingCitesClauses(t *testing.T) {
	var out, errb bytes.Buffer
	run([]string{"-rules"}, &out, &errb)
	s := out.String()
	for _, want := range []string{"CG001", "GDPR:Art32(1)(a)", "CG011", "no clause found", "2011/833/EU"} {
		if !strings.Contains(s, want) {
			t.Errorf("-rules output missing %q", want)
		}
	}
}
