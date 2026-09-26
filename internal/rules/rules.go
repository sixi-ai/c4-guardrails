// Package rules holds the deterministic guardrails. A finding exists only
// because one of these rules matched facts declared in the model; nothing
// here guesses, scores or calls a model.
package rules

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/rbrus/c4-guardrails/internal/model"
)

// Severity orders findings.
type Severity int

// Severities, lowest first.
const (
	Info Severity = iota
	Low
	Medium
	High
	Critical
)

var sevNames = []string{"info", "low", "medium", "high", "critical"}

func (s Severity) String() string {
	if s < Info || s > Critical {
		return "unknown"
	}
	return sevNames[s]
}

// MarshalJSON writes the severity as its name.
func (s Severity) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

// ParseSeverity parses a severity name. "none" returns ok with a value above
// Critical so that nothing reaches it.
func ParseSeverity(v string) (Severity, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "none" || v == "never" {
		return Critical + 1, true
	}
	for i, n := range sevNames {
		if n == v {
			return Severity(i), true
		}
	}
	return 0, false
}

// Rule is one deterministic guardrail.
type Rule struct {
	ID          string
	Name        string // kebab-case short name (SARIF rule name)
	Title       string
	Severity    Severity
	Rationale   string
	Remediation string
	Clauses     []string // REGIME:DOC:REF ids from internal/clauses; empty → "no clause found"
	check       func(m *model.Model) []hit
}

type hit struct {
	subj  string // optional subject override
	msg   string
	nodes []string
	edges []int
	line  int
}

// Finding is one rule match.
type Finding struct {
	RuleID      string   `json:"rule_id"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Message     string   `json:"message"`
	Subject     string   `json:"subject"`
	Nodes       []string `json:"nodes,omitempty"`
	Edges       []int    `json:"edges,omitempty"`
	Clauses     []string `json:"clauses"`
	Remediation string   `json:"remediation"`
	Source      string   `json:"source"`
	Line        int      `json:"line"`
}

// Evaluate runs every rule against the model and returns sorted findings.
func Evaluate(m *model.Model) []Finding {
	var out []Finding
	for _, r := range All() {
		for _, h := range r.check(m) {
			f := Finding{
				RuleID: r.ID, Severity: r.Severity, Title: r.Title, Message: h.msg,
				Nodes: h.nodes, Edges: h.edges, Clauses: append([]string{}, r.Clauses...),
				Remediation: r.Remediation, Source: m.Source, Line: h.line,
			}
			f.Subject = subject(m, h)
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Severity != b.Severity {
			return a.Severity > b.Severity
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Subject < b.Subject
	})
	return out
}

func subject(m *model.Model, h hit) string {
	if h.subj != "" {
		return h.subj
	}
	if len(h.edges) > 0 {
		e := m.Edges[h.edges[0]]
		return name(m, e.From) + " → " + name(m, e.To)
	}
	if len(h.nodes) > 0 {
		return name(m, h.nodes[0])
	}
	return "model"
}

func name(m *model.Model, id string) string {
	if n := m.Node(id); n != nil && n.Name != "" {
		return n.Name
	}
	return id
}

// --- helpers ------------------------------------------------------------------

// crosses reports whether an edge leaves a trust zone: it touches a person or
// an external element, or links different boundaries.
func crosses(m *model.Model, e *model.Edge) bool {
	f, t := m.Node(e.From), m.Node(e.To)
	if f.External || t.External || f.IsPerson() || t.IsPerson() {
		return true
	}
	return f.Boundary != t.Boundary
}

// carriesPII reports whether an edge (or its source) is declared to handle
// personal data.
func carriesPII(m *model.Model, e *model.Edge) bool {
	return e.Is("pii") || m.Node(e.From).Is("pii")
}

func internetFacing(m *model.Model, n *model.Node) bool {
	if n.IsPerson() || n.Is("internet") {
		return true
	}
	// Walk up the boundary chain; the depth cap guards against parent cycles.
	b := m.Boundary(n.Boundary)
	for depth := 0; b != nil && depth < 32; depth++ {
		for _, t := range b.Tags {
			if t == "internet" {
				return true
			}
		}
		b = m.Boundary(b.Parent)
	}
	return false
}

func edgeHit(m *model.Model, i int, msg string) hit {
	e := m.Edges[i]
	return hit{msg: msg, edges: []int{i}, nodes: []string{e.From, e.To}, line: e.Line}
}

func nodeHit(n *model.Node, msg string) hit {
	return hit{msg: msg, nodes: []string{n.ID}, line: n.Line}
}

func modelLine(m *model.Model) int {
	if len(m.Nodes) > 0 {
		return m.Nodes[0].Line
	}
	return 0
}

// --- the rules ------------------------------------------------------------------

var all = []Rule{
	{
		ID: "CG001", Name: "plaintext-transport-across-boundary", Severity: High,
		Title:       "Plaintext transport across a trust boundary",
		Rationale:   "A relationship that leaves a trust zone (person, external system or another boundary) declares a protocol without TLS, so data in transit is readable and alterable on the path.",
		Remediation: "Use TLS (HTTPS, JDBC/TLS, AMQPS, mTLS ...) and say so in the technology label, or add $tags=\"enc:tls\".",
		Clauses:     []string{"GDPR:Art32(1)(a)", "NIS2:2022/2555:Art21(2)(h)"},
		check: func(m *model.Model) []hit {
			var out []hit
			for i, e := range m.Edges {
				if tr, why := e.Transport(); tr == model.TransportPlaintext && crosses(m, e) {
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q crosses a trust boundary: %s.", e.Label, why)))
				}
			}
			return out
		},
	},
	{
		ID: "CG002", Name: "external-call-without-auth", Severity: High,
		Title:       "External system called without declared authentication",
		Rationale:   "A call to or from a third-party system declares no authentication (or declares none), so the model does not evidence who may use the interface.",
		Remediation: "Declare the mechanism, e.g. $tags=\"auth:oauth2\", \"auth:mtls\" or \"auth:managed_identity\".",
		Clauses:     []string{"NIS2:2022/2555:Art21(2)(j)", "GDPR:Art32"},
		check: func(m *model.Model) []hit {
			var out []hit
			for i, e := range m.Edges {
				f, t := m.Node(e.From), m.Node(e.To)
				if !(t.External && !t.IsPerson()) && !(f.External && !f.IsPerson()) {
					continue
				}
				switch a := e.Auth(); {
				case a == "":
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q: no authentication declared (missing fact).", e.Label)))
				case model.NoAuth(a):
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q: authentication declared as %q.", e.Label, a)))
				}
			}
			return out
		},
	},
	{
		ID: "CG003", Name: "personal-data-store-unencrypted", Severity: High,
		Title:       "Personal-data store without declared encryption at rest",
		Rationale:   "A database tagged as holding personal data does not declare encryption at rest.",
		Remediation: "Declare it, e.g. $tags=\"pii;at_rest:aes256\" (or at_rest:cmk for customer-managed keys).",
		Clauses:     []string{"GDPR:Art32(1)(a)", "NIS2:2022/2555:Art21(2)(h)"},
		check: func(m *model.Model) []hit {
			var out []hit
			for _, n := range m.Nodes {
				if !n.IsDatabase() || !n.Is("pii") {
					continue
				}
				switch v := n.Attr("at_rest"); {
				case v == "":
					out = append(out, nodeHit(n, "Holds personal data; encryption at rest is not declared (missing fact)."))
				case model.Declined(v):
					out = append(out, nodeHit(n, fmt.Sprintf("Holds personal data; encryption at rest declared as %q.", v)))
				}
			}
			return out
		},
	},
	{
		ID: "CG004", Name: "ai-personal-data-without-oversight", Severity: High,
		Title:       "AI endpoint receives personal data without a human-oversight component",
		Rationale:   "An element tagged ai/llm receives personal data, and the model declares no human-oversight component (review queue, escalation to a person, override).",
		Remediation: "Add the oversight step to the design and tag it, e.g. Container(review, \"Human review\", ..., $tags=\"oversight\").",
		Clauses:     []string{"AIACT:2024/1689:Art14", "GDPR:Art25"},
		check: func(m *model.Model) []hit {
			if m.Any("oversight") {
				return nil
			}
			var out []hit
			for i, e := range m.Edges {
				if m.Node(e.To).Is("ai") && carriesPII(m, e) {
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q sends personal data to an AI endpoint; no element tagged oversight.", e.Label)))
				}
			}
			return out
		},
	},
	{
		ID: "CG005", Name: "critical-third-party-without-exit", Severity: Medium,
		Title:       "Critical third-party ICT service without a declared exit or alternative",
		Rationale:   "An external system tagged critical has no declared exit strategy or alternative provider, so the design shows a single point of third-party dependency.",
		Remediation: "Declare the exit path, e.g. $tags=\"critical;exit:secondary-provider\", and draw the alternative if it exists.",
		Clauses:     []string{"DORA:2022/2554:Art28(8)", "DORA:2022/2554:Art28", "NIS2:2022/2555:Art21(2)(d)"},
		check: func(m *model.Model) []hit {
			var out []hit
			for _, n := range m.Nodes {
				if n.External && !n.IsPerson() && n.Is("critical") && (n.Attr("exit") == "" || model.Declined(n.Attr("exit"))) {
					out = append(out, nodeHit(n, "Tagged critical; no exit strategy or alternative declared."))
				}
			}
			return out
		},
	},
	{
		ID: "CG006", Name: "database-reachable-from-internet", Severity: High,
		Title:       "Database reachable directly from a person or the internet",
		Rationale:   "A person, or an element in an internet-facing zone, connects straight to a database without an application tier in between.",
		Remediation: "Route access through an application or API tier that authenticates and authorises; keep the database in an internal zone.",
		Clauses:     []string{"NIS2:2024/2690:Annex-6.8", "GDPR:Art25"},
		check: func(m *model.Model) []hit {
			var out []hit
			for i, e := range m.Edges {
				f, t := m.Node(e.From), m.Node(e.To)
				if t.IsDatabase() && !t.External && internetFacing(m, f) {
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q connects %s directly to a database.", e.Label, f.Name)))
				}
			}
			return out
		},
	},
	{
		ID: "CG007", Name: "no-logging-for-personal-data", Severity: Medium,
		Title:       "No logging or monitoring component in a system handling personal data",
		Rationale:   "The model declares elements that handle personal data but no logging, monitoring or SIEM element.",
		Remediation: "Add the log sink to the diagram and tag it, e.g. Container(logs, \"Central logging\", \"OpenTelemetry\", $tags=\"logging\").",
		Clauses:     []string{"NIS2:2024/2690:Annex-3.2", "DORA-RTS:2024/1774:Art12"},
		check: func(m *model.Model) []hit {
			if !m.Any("pii") || m.Any("logging") {
				return nil
			}
			var ids []string
			for _, n := range m.Nodes {
				if n.Is("pii") {
					ids = append(ids, n.ID)
				}
			}
			return []hit{{subj: "whole diagram", msg: fmt.Sprintf("%d element(s) handle personal data; no element tagged logging.", len(ids)), nodes: ids, line: modelLine(m)}}
		},
	},
	{
		ID: "CG008", Name: "credentials-without-secrets-store", Severity: Medium,
		Title:       "Credential-based connections without a secrets store",
		Rationale:   "Relationships authenticate with shared secrets (API keys, passwords) but the model has no secrets store, so it does not evidence where those secrets live.",
		Remediation: "Add a secrets store (Key Vault, Vault, KMS) tagged secrets, or switch to workload identity (auth:managed_identity, auth:mtls).",
		Clauses:     []string{"NIS2:2022/2555:Art21(2)(i)", "GDPR:Art32"},
		check: func(m *model.Model) []hit {
			if m.Any("secrets") {
				return nil
			}
			var out []hit
			for i, e := range m.Edges {
				if a := e.Auth(); model.CredentialAuth(a) {
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q authenticates with %s; no element tagged secrets.", e.Label, a)))
				}
			}
			return out
		},
	},
	{
		ID: "CG009", Name: "high-risk-ai-without-records", Severity: High,
		Title:       "High-risk AI system without a record-keeping log store",
		Rationale:   "An element tagged ai and high-risk has no connected store for automatically generated logs.",
		Remediation: "Add an append-only log store tagged records and draw the relationship from the AI system to it.",
		Clauses:     []string{"AIACT:2024/1689:Art12", "AIACT:2024/1689:Art26(6)"},
		check: func(m *model.Model) []hit {
			var out []hit
			for _, n := range m.Nodes {
				if !(n.Is("ai") && n.Is("high-risk")) {
					continue
				}
				linked := false
				for _, e := range m.Edges {
					if e.From == n.ID && m.Node(e.To).Is("records") || e.To == n.ID && m.Node(e.From).Is("records") {
						linked = true
						break
					}
				}
				if !linked {
					out = append(out, nodeHit(n, "Tagged ai + high-risk; no relationship to an element tagged records."))
				}
			}
			return out
		},
	},
	{
		ID: "CG010", Name: "personal-data-to-external-without-region", Severity: Medium,
		Title:       "Personal data sent to an external system with no declared processing region",
		Rationale:   "Personal data flows to a third-party system whose processing location is not declared, so the model cannot show whether a transfer outside the EU/EEA happens.",
		Remediation: "Declare the location on the external system, e.g. $tags=\"region:eu-west\", and record the transfer safeguard if it is outside the EU/EEA.",
		Clauses:     []string{"GDPR:Art44", "GDPR:Art46"},
		check: func(m *model.Model) []hit {
			var out []hit
			for i, e := range m.Edges {
				t := m.Node(e.To)
				if t.External && !t.IsPerson() && carriesPII(m, e) && t.Attr("region") == "" {
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q sends personal data to %s; region not declared (missing fact).", e.Label, t.Name)))
				}
			}
			return out
		},
	},
	{
		ID: "CG011", Name: "missing-fact", Severity: Info,
		Title:       "Missing fact",
		Rationale:   "Some rules could not decide because the model does not state a fact (for example the transport of a relationship that crosses a boundary), or an attribute key is not recognised. This is information, not an error.",
		Remediation: "State the fact in the technology label or with $tags (see the attribute vocabulary in the README).",
		check: func(m *model.Model) []hit {
			var out []hit
			for i, e := range m.Edges {
				if tr, _ := e.Transport(); tr == model.TransportUnknown && crosses(m, e) {
					out = append(out, edgeHit(m, i, fmt.Sprintf("%q crosses a trust boundary; transport encryption not stated.", e.Label)))
				}
				for _, k := range model.UnknownAttrs(e.Attrs) {
					out = append(out, edgeHit(m, i, fmt.Sprintf("Unknown attribute %q ignored.", k)))
				}
			}
			for _, n := range m.Nodes {
				for _, k := range model.UnknownAttrs(n.Attrs) {
					out = append(out, nodeHit(n, fmt.Sprintf("Unknown attribute %q ignored.", k)))
				}
			}
			return out
		},
	},
}

// All returns the rule table in id order.
func All() []Rule { return all }

// ByID returns a rule by id.
func ByID(id string) (Rule, bool) {
	for _, r := range all {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}
