package report

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/rbrus/c4-guardrails/internal/clauses"
	"github.com/rbrus/c4-guardrails/internal/model"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

// hostile names try Markdown/HTML/terminal injection.
var hostile = strings.ReplaceAll(`C4Container
title <script>alert(1)</script> | x
Person(u, "Eve](javascript:alert(1)) <img src=x onerror=alert(1)> @org/team | cell", "descr")
System_Ext(x, "Evil `+"`code`"+` **bold** ::error::pwn \u202Eevil", "", $tags="critical")
Rel(u, x, "line1 &amp; <b>bold</b> #1 ![img](http://e/x.png)", "HTTP")
`, `\u202E`, "\u202E")

func result(t *testing.T, src string) *Result {
	t.Helper()
	m, err := model.ParseMermaid("in/<x>.mmd", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return &Result{Version: "test", Models: []*model.Model{m}, Findings: rules.Evaluate(m), FailOn: rules.High}
}

func TestOneLine(t *testing.T) {
	cases := map[string]string{
		"a\nb\r\nc":            "a b c",
		"  x \t y  ":           "x y",
		"esc\x1b[31mred":       "esc [31mred",
		"bidi\u202Eevil\u2066": "bidi evil",
		"nul\x00byte":          "nul byte",
	}
	for in, want := range cases {
		if got := OneLine(in, 0); got != want {
			t.Errorf("OneLine(%q) = %q, want %q", in, got, want)
		}
	}
	if got := OneLine("abcdef", 4); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
}

func TestMarkdownEscaping(t *testing.T) {
	cases := []struct{ in, mustNot string }{
		{"[x](javascript:alert(1))", "](javascript"},
		{"<img src=x onerror=alert(1)>", "<img"},
		{"a | b", " | b"},
		{"@org/team", "@org"},
		{"![i](http://e/x.png)", "!["},
		{"`code`", "`code`"},
		{"**b**", "**b**"},
		{"line\n# heading", "\n"},
		{"https://evil.example", "https://"},
	}
	for _, c := range cases {
		if got := MD(c.in); strings.Contains(got, c.mustNot) {
			t.Errorf("MD(%q) = %q still contains %q", c.in, got, c.mustNot)
		}
	}
	var buf bytes.Buffer
	if err := WriteMarkdown(&buf, result(t, hostile)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, bad := range []string{"<script", "<img", "<b>", "](javascript", "![img]", "@org", "\u202E"} {
		if strings.Contains(out, bad) {
			t.Errorf("markdown output contains %q:\n%s", bad, out)
		}
	}
	// Every table row keeps exactly 6 pipes (5 cells), so no cell was split.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "| ") && strings.Contains(line, "CG") {
			if n := strings.Count(line, "|"); n != 6 {
				t.Errorf("row has %d pipes: %s", n, line)
			}
		}
	}
	if !strings.Contains(out, "not a compliance certification") {
		t.Error("disclaimer missing")
	}
}

func TestHTMLEscaping(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteHTML(&buf, result(t, hostile)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, bad := range []string{"<script>alert", "<img src=x", "<b>bold</b>", "onerror=alert(1)>", "\u202E"} {
		if strings.Contains(out, bad) {
			t.Errorf("html output contains %q", bad)
		}
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Error("expected escaped title")
	}
	if strings.Contains(out, "<script") || strings.Contains(out, "src=\"http") || strings.Contains(out, "<link") {
		t.Error("report must be self-contained: no scripts or external resources")
	}
	if !strings.Contains(out, "Content-Security-Policy") {
		t.Error("CSP meta missing")
	}
	// Clause chips link to EUR-Lex only.
	for _, m := range regexp.MustCompile(`href="([^"]+)"`).FindAllStringSubmatch(out, -1) {
		if !strings.HasPrefix(m[1], "https://eur-lex.europa.eu/") {
			t.Errorf("unexpected link %q", m[1])
		}
	}
}

func TestSVGHighlightsOffenders(t *testing.T) {
	r := result(t, `C4Container
Person(u, "U")
Container(a, "A")
ContainerDb(d, "D", "", "", $tags="pii;at_rest:aes256")
Rel(u, a, "uses", "HTTP")
Rel(a, d, "reads", "JDBC/TLS", $tags="auth:managed_identity")
`)
	svg := DiagramSVG(r.Models[0], r.Findings)
	if got := strings.Count(svg, `class="edge flagged"`); got != 1 {
		t.Errorf("flagged edges = %d, want 1", got)
	}
	if !strings.Contains(svg, "CG001") || !strings.Contains(svg, "#cf222e") {
		t.Error("expected red CG001 highlight")
	}
	if strings.Count(svg, `class="node`) != 3 {
		t.Error("expected 3 nodes")
	}
}

func TestSVGLayoutHandlesCycles(t *testing.T) {
	r := result(t, `C4Container
Container(a, "A")
Container(b, "B")
Container(c, "C")
Rel(a, b, "x", "HTTPS")
Rel(b, c, "x", "HTTPS")
Rel(c, a, "x", "HTTPS")
Rel(a, a, "self", "HTTPS")
`)
	svg := DiagramSVG(r.Models[0], r.Findings)
	if !strings.HasPrefix(svg, "<svg") || !strings.HasSuffix(svg, "</svg>") || strings.Contains(svg, "NaN") {
		t.Fatalf("bad svg: %.200s", svg)
	}
}

func TestSARIFShape(t *testing.T) {
	var buf bytes.Buffer
	r := result(t, hostile)
	if err := WriteSARIF(&buf, r); err != nil {
		t.Fatal(err)
	}
	var log struct {
		Version string `json:"version"`
		Schema  string `json:"$schema"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID         string `json:"id"`
						Help       struct{ Text, Markdown string }
						Properties struct {
							Tags     []string `json:"tags"`
							Severity string   `json:"security-severity"`
						} `json:"properties"`
						Default struct {
							Level string `json:"level"`
						} `json:"defaultConfiguration"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID    string `json:"ruleId"`
				RuleIndex int    `json:"ruleIndex"`
				Level     string `json:"level"`
				Message   struct{ Text string }
				Locations []struct {
					PhysicalLocation struct {
						ArtifactLocation struct{ URI string } `json:"artifactLocation"`
						Region           struct {
							StartLine int `json:"startLine"`
						} `json:"region"`
					} `json:"physicalLocation"`
				} `json:"locations"`
				Fingerprints map[string]string `json:"partialFingerprints"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatal(err)
	}
	if log.Version != "2.1.0" || !strings.Contains(log.Schema, "sarif-2.1.0") || len(log.Runs) != 1 {
		t.Fatalf("bad header: %+v", log)
	}
	run := log.Runs[0]
	if run.Tool.Driver.Name != ToolName || len(run.Tool.Driver.Rules) != len(rules.All()) {
		t.Fatalf("driver = %+v", run.Tool.Driver.Name)
	}
	for i, ru := range run.Tool.Driver.Rules {
		want, _ := rules.ByID(ru.ID)
		for _, c := range want.Clauses {
			if !contains(ru.Properties.Tags, c) || !strings.Contains(ru.Help.Text, c) || !strings.Contains(ru.Help.Markdown, "eur-lex.europa.eu") {
				t.Errorf("rule %s: clause %s missing from tags/help", ru.ID, c)
			}
		}
		if len(want.Clauses) == 0 && !strings.Contains(ru.Help.Text, clauses.NoClause) {
			t.Errorf("rule %s: expected %q in help", ru.ID, clauses.NoClause)
		}
		if ru.Default.Level != SARIFLevel(want.Severity) || ru.Properties.Severity == "" {
			t.Errorf("rule %d level/severity wrong", i)
		}
	}
	if len(run.Results) != len(r.Findings) || len(run.Results) == 0 {
		t.Fatalf("results = %d, findings = %d", len(run.Results), len(r.Findings))
	}
	for i, res := range run.Results {
		f := r.Findings[i]
		if res.RuleID != f.RuleID || run.Tool.Driver.Rules[res.RuleIndex].ID != f.RuleID || res.Level != SARIFLevel(f.Severity) {
			t.Errorf("result %d mismatched", i)
		}
		loc := res.Locations[0].PhysicalLocation
		if loc.ArtifactLocation.URI != "in/<x>.mmd" || loc.Region.StartLine < 1 || res.Fingerprints["c4guardrails/v1"] == "" {
			t.Errorf("result %d location = %+v", i, loc)
		}
		if strings.ContainsAny(res.Message.Text, "\n\r\u202E") {
			t.Errorf("result %d message not one line", i)
		}
	}
	for s, want := range map[rules.Severity]string{rules.Critical: "error", rules.High: "error", rules.Medium: "warning", rules.Low: "note", rules.Info: "note"} {
		if SARIFLevel(s) != want {
			t.Errorf("level(%s) = %s", s, SARIFLevel(s))
		}
	}
}

func TestTableIsOneLinePerRecord(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteTable(&buf, result(t, hostile)); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "\x1b\r\u202E") {
		t.Error("control characters leaked into the table")
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "::") {
			t.Errorf("a line starts with a workflow command: %q", line)
		}
	}
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	r := result(t, hostile)
	if err := WriteJSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var out struct {
		Tool     string                     `json:"tool"`
		Summary  map[string]int             `json:"summary"`
		Findings []map[string]any           `json:"findings"`
		Clauses  map[string]json.RawMessage `json:"clauses"`
	}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Tool != ToolName || len(out.Findings) != len(r.Findings) || out.Summary["high"] == 0 {
		t.Fatalf("json = %+v", out)
	}
	for _, f := range r.Findings {
		for _, id := range f.Clauses {
			if _, ok := out.Clauses[id]; !ok {
				t.Errorf("cited clause %s missing from clauses map", id)
			}
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
