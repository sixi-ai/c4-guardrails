// Package report renders findings as a terminal table, JSON, SARIF 2.1.0,
// Markdown and a self-contained HTML page.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rbrus/c4-guardrails/internal/clauses"
	"github.com/rbrus/c4-guardrails/internal/model"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

// ToolName and ToolURI identify the tool in every output.
const (
	ToolName = "c4-guardrails"
	ToolURI  = "https://github.com/rbrus/c4-guardrails"
)

// Disclaimer is printed in every human-facing output.
const Disclaimer = "c4-guardrails assesses what a design-time diagram declares against deterministic rules. " +
	"It evidences gaps and proposes fixes; it is not a compliance certification and not legal advice."

// Result is everything a renderer needs.
type Result struct {
	Version  string
	Models   []*model.Model
	Findings []rules.Finding
	FailOn   rules.Severity
}

// Counts returns the number of findings per severity.
func (r *Result) Counts() map[rules.Severity]int {
	c := map[rules.Severity]int{}
	for _, f := range r.Findings {
		c[f.Severity]++
	}
	return c
}

// Failing returns the number of findings at or above the threshold.
func (r *Result) Failing() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity >= r.FailOn {
			n++
		}
	}
	return n
}

// FindingsFor returns the findings for one model source.
func (r *Result) FindingsFor(m *model.Model) []rules.Finding {
	var out []rules.Finding
	for _, f := range r.Findings {
		if f.Source == m.Source {
			out = append(out, f)
		}
	}
	return out
}

// Cited returns the clauses cited by the findings, in id order.
func (r *Result) Cited() []clauses.Clause {
	used := map[string]bool{}
	for _, f := range r.Findings {
		for _, id := range f.Clauses {
			used[id] = true
		}
	}
	var out []clauses.Clause
	for _, c := range clauses.All() {
		if used[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

// FailOnLabel is the threshold as text.
func (r *Result) FailOnLabel() string {
	if r.FailOn > rules.Critical {
		return "none"
	}
	return r.FailOn.String()
}

// severities from most to least severe.
var severities = []rules.Severity{rules.Critical, rules.High, rules.Medium, rules.Low, rules.Info}

// SummaryLine is "0 critical · 3 high · ...".
func (r *Result) SummaryLine() string {
	c := r.Counts()
	parts := make([]string, 0, len(severities))
	for _, s := range severities {
		parts = append(parts, fmt.Sprintf("%d %s", c[s], s))
	}
	return strings.Join(parts, " · ")
}

// OneLine makes untrusted text safe for a single terminal/log line: control
// characters, newlines and bidi overrides become spaces, runs of spaces
// collapse, and the result is capped.
func OneLine(s string, max int) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) || isBidi(r) || unicode.IsSpace(r) {
			if !space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = true
			continue
		}
		space = false
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if max > 0 && utf8.RuneCountInString(out) > max {
		rs := []rune(out)
		out = string(rs[:max-1]) + "…"
	}
	return out
}

func isBidi(r rune) bool {
	return r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 || r == 0x200E || r == 0x200F || r == 0x061C
}

// WriteJSON writes the machine-readable result.
func WriteJSON(w io.Writer, r *Result) error {
	type input struct {
		Source        string   `json:"source"`
		Title         string   `json:"title,omitempty"`
		Elements      int      `json:"elements"`
		Relationships int      `json:"relationships"`
		Warnings      []string `json:"warnings,omitempty"`
	}
	type out struct {
		Tool        string                    `json:"tool"`
		Version     string                    `json:"version"`
		Inputs      []input                   `json:"inputs"`
		FailOn      string                    `json:"fail_on"`
		Summary     map[string]int            `json:"summary"`
		Findings    []rules.Finding           `json:"findings"`
		Clauses     map[string]clauses.Clause `json:"clauses"`
		Attribution string                    `json:"clause_attribution"`
		Disclaimer  string                    `json:"disclaimer"`
	}
	o := out{Tool: ToolName, Version: r.Version, FailOn: r.FailOnLabel(), Summary: map[string]int{},
		Findings: r.Findings, Clauses: map[string]clauses.Clause{}, Attribution: clauses.Attribution(), Disclaimer: Disclaimer}
	if o.Findings == nil {
		o.Findings = []rules.Finding{}
	}
	for _, m := range r.Models {
		o.Inputs = append(o.Inputs, input{m.Source, m.Title, len(m.Nodes), len(m.Edges), m.Warnings})
	}
	c := r.Counts()
	for _, s := range severities {
		o.Summary[s.String()] = c[s]
	}
	for _, cl := range r.Cited() {
		o.Clauses[cl.ID] = cl
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(true)
	return enc.Encode(o)
}

// ClauseLabel renders a clause list, or "no clause found".
func ClauseLabel(ids []string) string {
	if len(ids) == 0 {
		return clauses.NoClause
	}
	return strings.Join(ids, ", ")
}
