package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/rbrus/c4-guardrails/internal/clauses"
)

// MD escapes untrusted text for GitHub-flavoured Markdown (tables, PR
// comments, $GITHUB_STEP_SUMMARY). It flattens the text to one line, turns
// HTML metacharacters into entities and backslash-escapes Markdown syntax,
// so a diagram label cannot open a link, image, HTML tag, table cell or
// mention.
func MD(s string) string {
	s = OneLine(s, 400)
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '@':
			b.WriteString("&#64;")
		case '|':
			b.WriteString("&#124;")
		case '\\', '`', '*', '_', '{', '}', '[', ']', '(', ')', '#', '+', '!', '~', ':':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// WriteMarkdown writes a compact summary for a PR comment or step summary.
func WriteMarkdown(w io.Writer, r *Result) error {
	var b strings.Builder
	c := r.Counts()
	status := "no findings at or above the threshold"
	if n := r.Failing(); n > 0 {
		status = fmt.Sprintf("**%d finding(s) at or above `%s`**", n, r.FailOnLabel())
	}
	fmt.Fprintf(&b, "### c4-guardrails: %s\n\n", status)
	fmt.Fprintf(&b, "| critical | high | medium | low | info |\n|---:|---:|---:|---:|---:|\n| %d | %d | %d | %d | %d |\n\n",
		c[severities[0]], c[severities[1]], c[severities[2]], c[severities[3]], c[severities[4]])
	for _, m := range r.Models {
		fs := r.FindingsFor(m)
		title := ""
		if m.Title != "" {
			title = " (" + MD(m.Title) + ")"
		}
		fmt.Fprintf(&b, "#### %s%s\n\n", MD(m.Source), title)
		if len(fs) == 0 {
			b.WriteString("No findings.\n\n")
			continue
		}
		b.WriteString("| Severity | Rule | Where | Finding | Clauses |\n|---|---|---|---|---|\n")
		for _, f := range fs {
			fmt.Fprintf(&b, "| %s | %s %s | %s (line %d) | %s | %s |\n",
				strings.ToUpper(f.Severity.String()), f.RuleID, MD(f.Title), MD(f.Subject), f.Line, MD(f.Message), mdClauses(f.Clauses))
		}
		b.WriteString("\n")
	}
	if len(r.Findings) > 0 {
		b.WriteString("<details><summary>Proposed fixes</summary>\n\n")
		seen := map[string]bool{}
		for _, f := range r.Findings {
			if seen[f.RuleID] {
				continue
			}
			seen[f.RuleID] = true
			fmt.Fprintf(&b, "- **%s %s**: %s\n", f.RuleID, MD(f.Title), MD(f.Remediation))
		}
		b.WriteString("\n</details>\n\n")
	}
	fmt.Fprintf(&b, "<sub>%s Clause references: %s.</sub>\n", MD(Disclaimer), MD(clauses.Attribution()))
	_, err := io.WriteString(w, b.String())
	return err
}

func mdClauses(ids []string) string {
	if len(ids) == 0 {
		return "_" + clauses.NoClause + "_"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if c, ok := clauses.Lookup(id); ok {
			parts = append(parts, fmt.Sprintf("[%s](%s)", MD(id), c.SourceURL))
		} else {
			parts = append(parts, MD(id)+" ("+clauses.NoClause+")")
		}
	}
	return strings.Join(parts, "<br>")
}
