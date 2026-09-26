package report

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func pad(s string, w int) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

// WriteTable writes the human-readable report. Every piece of model text is
// passed through OneLine so a finding never spans or injects lines.
func WriteTable(w io.Writer, r *Result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", ToolName, OneLine(r.Version, 40))
	for _, m := range r.Models {
		fs := r.FindingsFor(m)
		title := ""
		if m.Title != "" {
			title = " (" + OneLine(m.Title, 60) + ")"
		}
		fmt.Fprintf(&b, "\n%s%s: %d elements, %d relationships, %d findings\n",
			OneLine(m.Source, 120), title, len(m.Nodes), len(m.Edges), len(fs))
		for _, wmsg := range m.Warnings {
			fmt.Fprintf(&b, "  warning: %s\n", OneLine(wmsg, 160))
		}
		if len(fs) == 0 {
			b.WriteString("  no findings\n")
			continue
		}
		fmt.Fprintf(&b, "\n  %s %s %s %s\n", pad("SEVERITY", 9), pad("RULE", 6), pad("WHERE", 34), "FINDING")
		for _, f := range fs {
			where := OneLine(f.Subject, 33)
			fmt.Fprintf(&b, "  %s %s %s %s\n", pad(strings.ToUpper(f.Severity.String()), 9), pad(f.RuleID, 6), pad(where, 34), OneLine(f.Message, 200))
			ind := strings.Repeat(" ", 2+9+1+6+1+34+1)
			fmt.Fprintf(&b, "%sline %d · %s\n", ind, f.Line, OneLine(ClauseLabel(f.Clauses), 200))
		}
	}
	verdict := "pass"
	if r.Failing() > 0 {
		verdict = fmt.Sprintf("fail: %d finding(s) at or above %q", r.Failing(), r.FailOnLabel())
	}
	fmt.Fprintf(&b, "\nSummary: %s -> %s\n", r.SummaryLine(), verdict)
	b.WriteString("Assesses declared design facts only; not a compliance certification.\n")
	_, err := io.WriteString(w, b.String())
	return err
}
