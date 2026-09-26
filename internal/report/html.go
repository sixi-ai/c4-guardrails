package report

import (
	"html/template"
	"io"
	"strings"

	"github.com/rbrus/c4-guardrails/internal/clauses"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

type htmlChip struct {
	ID, Title, URL string
	Known          bool
}

type htmlFinding struct {
	Severity    string
	RuleID      string
	Title       string
	Subject     string
	Message     string
	Line        int
	Chips       []htmlChip
	Remediation string
}

type htmlModel struct {
	Source   string
	Title    string
	Elements int
	Rels     int
	SVG      template.HTML
	Findings []htmlFinding
	Warnings []string
}

type htmlCard struct {
	Label string
	Count int
}

type htmlPage struct {
	Version     string
	Heading     string
	Cards       []htmlCard
	Failing     int
	FailOn      string
	Models      []htmlModel
	Cited       []clauses.Clause
	Attribution string
	Disclaimer  string
	NoClause    string
}

// WriteHTML writes one self-contained HTML page (no external requests).
func WriteHTML(w io.Writer, r *Result) error {
	p := htmlPage{Version: r.Version, Failing: r.Failing(), FailOn: r.FailOnLabel(), Cited: r.Cited(),
		Attribution: clauses.Attribution(), Disclaimer: Disclaimer, NoClause: clauses.NoClause}
	c := r.Counts()
	for _, s := range severities {
		p.Cards = append(p.Cards, htmlCard{s.String(), c[s]})
	}
	for _, m := range r.Models {
		fs := r.FindingsFor(m)
		hm := htmlModel{Source: OneLine(m.Source, 200), Title: OneLine(m.Title, 200), Elements: len(m.Nodes), Rels: len(m.Edges),
			SVG: template.HTML(DiagramSVG(m, fs)), Warnings: m.Warnings} // #nosec G203 -- every string inside DiagramSVG is escaped
		for _, f := range fs {
			hf := htmlFinding{Severity: f.Severity.String(), RuleID: f.RuleID, Title: f.Title, Subject: OneLine(f.Subject, 120),
				Message: OneLine(f.Message, 400), Line: f.Line, Remediation: f.Remediation}
			for _, id := range f.Clauses {
				cl, ok := clauses.Lookup(id)
				hf.Chips = append(hf.Chips, htmlChip{ID: id, Title: cl.Title, URL: cl.SourceURL, Known: ok})
			}
			hm.Findings = append(hm.Findings, hf)
		}
		p.Models = append(p.Models, hm)
	}
	if len(r.Models) == 1 {
		p.Heading = firstNonEmpty(r.Models[0].Title, r.Models[0].Source)
	} else {
		p.Heading = "Architecture guardrails report"
	}
	p.Heading = OneLine(p.Heading, 120)
	return pageTmpl.Execute(w, p)
}

var pageTmpl = template.Must(template.New("page").Funcs(template.FuncMap{
	"upper": strings.ToUpper,
	"sevclass": func(s string) string {
		if _, ok := rules.ParseSeverity(s); ok {
			return "sev-" + s
		}
		return "sev-info"
	},
}).Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">
<meta name="generator" content="c4-guardrails {{.Version}}">
<title>{{.Heading}} · c4-guardrails</title>
<style>
:root{--bg:#f6f8fa;--card:#ffffff;--ink:#1f2328;--muted:#59636e;--line:#d1d9e0;--crit:#8b0000;--high:#cf222e;--med:#bc4c00;--low:#9a6700;--info:#0969da;--ok:#1a7f37}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--ink);font:15px/1.5 Inter,"Segoe UI",system-ui,-apple-system,Helvetica,Arial,sans-serif}
header{background:#0d1117;color:#f0f6fc;padding:22px 40px 20px}
header .brand{font-size:13px;letter-spacing:.08em;text-transform:uppercase;color:#9198a1}
header h1{margin:4px 0 0;font-size:26px;font-weight:700}
header .meta{color:#9198a1;font-size:13px;margin-top:4px}
main{max-width:1500px;margin:0 auto;padding:24px 40px 40px}
.verdict{display:flex;align-items:center;gap:12px;padding:12px 16px;border-radius:10px;font-weight:600;margin-bottom:18px}
.verdict.fail{background:#ffebe9;color:#82071e;border:1px solid #ffcecb}
.verdict.pass{background:#dafbe1;color:#116329;border:1px solid #aceebb}
.verdict .dot{width:12px;height:12px;border-radius:50%;background:currentColor}
.cards{display:grid;grid-template-columns:repeat(5,1fr);gap:14px;margin-bottom:22px}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:14px 18px;border-top:5px solid var(--c)}
.card .n{font-size:34px;font-weight:800;line-height:1.1;color:var(--c)}
.card .l{font-size:13px;text-transform:uppercase;letter-spacing:.06em;color:var(--muted)}
.card.zero .n{color:#afb8c1}
.sev-critical{--c:var(--crit)}.sev-high{--c:var(--high)}.sev-medium{--c:var(--med)}.sev-low{--c:var(--low)}.sev-info{--c:var(--info)}
section.model{background:var(--card);border:1px solid var(--line);border-radius:14px;margin-bottom:26px;overflow:hidden}
section.model h2{margin:0;padding:14px 20px;font-size:18px;border-bottom:1px solid var(--line);display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap}
section.model h2 small{font-weight:500;color:var(--muted);font-size:13px}
.diagram{padding:12px;overflow-x:auto;background:#fbfcfd;border-bottom:1px solid var(--line)}
.diagram svg{display:block;margin:0 auto;max-width:100%;max-height:640px;width:auto;height:auto}
table{width:100%;border-collapse:collapse;font-size:14px}
th{text-align:left;font-size:12px;text-transform:uppercase;letter-spacing:.05em;color:var(--muted);padding:10px 14px;border-bottom:1px solid var(--line);background:#f6f8fa}
td{padding:11px 14px;border-bottom:1px solid #eaeef2;vertical-align:top}
tr:last-child td{border-bottom:0}
.pill{display:inline-block;padding:2px 10px;border-radius:999px;font-size:12px;font-weight:700;color:#fff;background:var(--c);text-transform:uppercase;letter-spacing:.04em}
.rule{font-weight:700;white-space:nowrap}
.rule small{display:block;font-weight:500;color:var(--muted);white-space:normal}
.subject{font-weight:600}
.where{color:var(--muted);font-size:12px}
.fix{color:var(--muted);font-size:13px;margin-top:4px}
.chip{display:inline-block;margin:0 4px 4px 0;padding:2px 9px;border-radius:6px;border:1px solid #b6d4fe;background:#ddf4ff;color:#0550ae;font:600 12px ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;text-decoration:none;white-space:nowrap}
.chip:hover{background:#b6e3ff}
.chip.none{border-color:var(--line);background:#f6f8fa;color:var(--muted);font-style:italic}
.empty{padding:22px;color:var(--ok);font-weight:600}
.clauses{background:var(--card);border:1px solid var(--line);border-radius:14px;padding:6px 20px 14px}
.clauses h2{font-size:17px}
.clauses dt{font-weight:700;margin-top:10px}
.clauses dd{margin:2px 0 0;color:var(--muted);font-size:14px}
footer{max-width:1500px;margin:0 auto;padding:0 40px 40px;color:var(--muted);font-size:12.5px}
.warn{padding:8px 20px;color:var(--low);font-size:13px}
</style>
</head>
<body>
<header>
  <div class="brand">c4-guardrails · design-time architecture guardrails</div>
  <h1>{{.Heading}}</h1>
  <div class="meta">Deterministic rules, no AI · version {{.Version}} · threshold: {{.FailOn}}</div>
</header>
<main>
{{if .Failing}}<div class="verdict fail"><span class="dot"></span>{{.Failing}} finding(s) at or above “{{.FailOn}}”. The rules propose fixes below.</div>
{{else}}<div class="verdict pass"><span class="dot"></span>No findings at or above “{{.FailOn}}” for the facts this diagram declares.</div>{{end}}
<div class="cards">{{range .Cards}}<div class="card {{sevclass .Label}}{{if eq .Count 0}} zero{{end}}"><div class="n">{{.Count}}</div><div class="l">{{.Label}}</div></div>{{end}}</div>
{{range .Models}}
<section class="model">
  <h2><span>{{if .Title}}{{.Title}}{{else}}{{.Source}}{{end}}</span><small>{{.Source}} · {{.Elements}} elements · {{.Rels}} relationships</small></h2>
  {{range .Warnings}}<div class="warn">{{.}}</div>{{end}}
  <div class="diagram">{{.SVG}}</div>
  {{if .Findings}}
  <table>
    <thead><tr><th style="width:92px">Severity</th><th style="width:230px">Rule</th><th>Finding</th><th style="width:300px">Clauses</th></tr></thead>
    <tbody>
    {{range .Findings}}<tr>
      <td><span class="pill {{sevclass .Severity}}">{{.Severity}}</span></td>
      <td class="rule">{{.RuleID}}<small>{{.Title}}</small></td>
      <td><div class="subject">{{.Subject}} <span class="where">line {{.Line}}</span></div><div>{{.Message}}</div><div class="fix">Proposed fix: {{.Remediation}}</div></td>
      <td>{{range .Chips}}{{if .Known}}<a class="chip" href="{{.URL}}" title="{{.Title}}" rel="noopener noreferrer" target="_blank">{{.ID}}</a>{{else}}<span class="chip none">{{.ID}} ({{$.NoClause}})</span>{{end}}{{else}}<span class="chip none">{{$.NoClause}}</span>{{end}}</td>
    </tr>{{end}}
    </tbody>
  </table>
  {{else}}<div class="empty">No findings for this diagram.</div>{{end}}
</section>
{{end}}
{{if .Cited}}<div class="clauses"><h2>Clauses cited</h2><dl>
{{range .Cited}}<dt><a class="chip" href="{{.SourceURL}}" rel="noopener noreferrer" target="_blank">{{.ID}}</a> {{.Title}}</dt><dd>{{.Instrument}}, {{.Ref}}. {{.Summary}}</dd>
{{end}}</dl></div>{{end}}
</main>
<footer>
  <p>{{.Disclaimer}}</p>
  <p>Clause titles and links: {{.Attribution}}. Summaries are paraphrases, not legal text; only the Official Journal of the European Union is authentic.</p>
</footer>
</body>
</html>
`))
