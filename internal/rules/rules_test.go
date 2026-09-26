package rules

import (
	"regexp"
	"testing"

	"github.com/rbrus/c4-guardrails/internal/model"
)

func eval(t *testing.T, body string) []Finding {
	t.Helper()
	m, err := model.ParseMermaid("t.mmd", []byte("C4Container\n"+body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return Evaluate(m)
}

func count(fs []Finding, id string) int {
	n := 0
	for _, f := range fs {
		if f.RuleID == id {
			n++
		}
	}
	return n
}

// ruleCases: each rule has positive (fires) and negative (silent) cases.
var ruleCases = []struct {
	name string
	rule string
	want int
	body string
}{
	// CG001 plaintext across a boundary
	{"plaintext from person", "CG001", 1, `Person(u, "U")
Container(a, "A", "Go")
Rel(u, a, "uses", "HTTP")`},
	{"tls from person", "CG001", 0, `Person(u, "U")
Container(a, "A", "Go")
Rel(u, a, "uses", "HTTPS")`},
	{"plaintext inside one boundary", "CG001", 0, `System_Boundary(s, "S") {
Container(a, "A")
ContainerDb(d, "D")
}
Rel(a, d, "reads", "JDBC")`},
	{"plaintext between boundaries", "CG001", 1, `System_Boundary(s, "S") {
Container(a, "A")
}
System_Boundary(t, "T") {
Container(b, "B")
}
Rel(a, b, "calls", "HTTP")`},
	{"declared enc none", "CG001", 1, `Container(a, "A")
System_Ext(x, "X")
Rel(a, x, "calls", "HTTPS", $tags="enc:none;auth:oauth2")`},

	// CG002 external without auth
	{"external no auth", "CG002", 1, `Container(a, "A")
System_Ext(x, "X")
Rel(a, x, "calls", "HTTPS")`},
	{"external auth none", "CG002", 1, `Container(a, "A")
System_Ext(x, "X")
Rel(a, x, "calls", "HTTPS", $tags="auth:none")`},
	{"inbound from external no auth", "CG002", 1, `Container(a, "A")
System_Ext(x, "X")
Rel(x, a, "pushes", "HTTPS")`},
	{"external with oauth", "CG002", 0, `Container(a, "A")
System_Ext(x, "X")
Rel(a, x, "calls", "HTTPS", $tags="auth:oauth2")`},
	{"external person not a system", "CG002", 0, `Person_Ext(p, "P")
Container(a, "A")
Rel(p, a, "uses", "HTTPS")`},

	// CG003 personal-data store at rest
	{"pii db undeclared", "CG003", 1, `ContainerDb(d, "D", "PostgreSQL", "", $tags="pii")`},
	{"pii db declared none", "CG003", 1, `ContainerDb(d, "D", "PostgreSQL", "", $tags="pii;at_rest:none")`},
	{"pii db encrypted", "CG003", 0, `ContainerDb(d, "D", "PostgreSQL", "", $tags="pii;at_rest:aes256")`},
	{"pii db encrypted tag", "CG003", 0, `ContainerDb(d, "D", "PostgreSQL", "", $tags="pii;encrypted")`},
	{"non-pii db", "CG003", 0, `ContainerDb(d, "D", "PostgreSQL")`},

	// CG004 AI + personal data without oversight
	{"ai gets pii", "CG004", 1, `Container(a, "A", "Go", "", $tags="pii")
System_Ext(l, "LLM", "", $tags="llm")
Rel(a, l, "prompt", "HTTPS")`},
	{"ai gets pii-tagged edge", "CG004", 1, `Container(a, "A")
Container(l, "Model", "Python", "", $tags="ai")
Rel(a, l, "prompt", "HTTPS", $tags="pii")`},
	{"ai with oversight", "CG004", 0, `Container(a, "A", "Go", "", $tags="pii")
Container(r, "Review", "Go", "", $tags="human-in-the-loop")
System_Ext(l, "LLM", "", $tags="llm")
Rel(a, l, "prompt", "HTTPS")`},
	{"ai without pii", "CG004", 0, `Container(a, "A")
System_Ext(l, "LLM", "", $tags="llm")
Rel(a, l, "prompt", "HTTPS")`},

	// CG005 critical third party exit
	{"critical no exit", "CG005", 1, `System_Ext(x, "X", "", $tags="critical")`},
	{"critical exit none", "CG005", 1, `System_Ext(x, "X", "", $tags="critical;exit:none")`},
	{"critical with exit", "CG005", 0, `System_Ext(x, "X", "", $tags="critical;exit:secondary")`},
	{"internal critical", "CG005", 0, `System(x, "X", "", $tags="critical")`},

	// CG006 database reachable from person / internet
	{"person to db", "CG006", 1, `Person(u, "U")
ContainerDb(d, "D")
Rel(u, d, "queries", "HTTPS")`},
	{"internet zone to db", "CG006", 1, `Boundary(dmz, "DMZ", "zone", $tags="internet") {
Container(w, "Web")
}
ContainerDb(d, "D")
Rel(w, d, "queries", "JDBC/TLS")`},
	{"nested internet zone to db", "CG006", 1, `Boundary(dmz, "DMZ", "zone", $tags="public") {
Boundary(inner, "Inner") {
Container(w, "Web")
}
}
ContainerDb(d, "D")
Rel(w, d, "queries", "JDBC/TLS")`},
	{"app tier to db", "CG006", 0, `Person(u, "U")
Container(a, "A")
ContainerDb(d, "D")
Rel(u, a, "uses", "HTTPS")
Rel(a, d, "queries", "JDBC/TLS")`},

	// CG007 logging
	{"pii without logging", "CG007", 1, `Container(a, "A", "Go", "", $tags="pii")`},
	{"pii with logging", "CG007", 0, `Container(a, "A", "Go", "", $tags="pii")
Container(l, "Logs", "OpenTelemetry")`},
	{"no pii", "CG007", 0, `Container(a, "A")`},

	// CG008 secrets store
	{"api key without vault", "CG008", 1, `Container(a, "A")
System_Ext(x, "X")
Rel(a, x, "calls", "HTTPS", $tags="auth:api_key")`},
	{"password with vault", "CG008", 0, `Container(a, "A")
ContainerDb(d, "D")
Container(v, "Vault", "HashiCorp Vault")
Rel(a, d, "reads", "JDBC/TLS", $tags="auth:password")`},
	{"managed identity", "CG008", 0, `Container(a, "A")
ContainerDb(d, "D")
Rel(a, d, "reads", "JDBC/TLS", $tags="auth:managed_identity")`},

	// CG009 high-risk AI record keeping
	{"high-risk no records", "CG009", 1, `Container(m, "Scoring", "Python", "", $tags="ai;high-risk")`},
	{"high-risk records not linked", "CG009", 1, `Container(m, "Scoring", "Python", "", $tags="ai;high-risk")
ContainerDb(r, "Log", "", "", $tags="records")`},
	{"high-risk linked records", "CG009", 0, `Container(m, "Scoring", "Python", "", $tags="ai;high-risk")
ContainerDb(r, "Log", "", "", $tags="audit-log")
Rel(m, r, "writes", "HTTPS")`},
	{"ai not high-risk", "CG009", 0, `Container(m, "Chat", "Python", "", $tags="ai")`},

	// CG010 personal data to external without region
	{"pii to external no region", "CG010", 1, `Container(a, "A", "", "", $tags="pii")
System_Ext(x, "X")
Rel(a, x, "sends", "HTTPS")`},
	{"pii to external with region", "CG010", 0, `Container(a, "A", "", "", $tags="pii")
System_Ext(x, "X", "", $tags="region:eu")
Rel(a, x, "sends", "HTTPS")`},
	{"no pii to external", "CG010", 0, `Container(a, "A")
System_Ext(x, "X")
Rel(a, x, "sends", "HTTPS")`},

	// CG011 missing facts
	{"unknown transport across boundary", "CG011", 1, `Person(u, "U")
Container(a, "A")
Rel(u, a, "uses", "REST")`},
	{"unknown attribute", "CG011", 1, `Container(a, "A", "", "", $tags="colour:blue")`},
	{"known transport", "CG011", 0, `Person(u, "U")
Container(a, "A")
Rel(u, a, "uses", "HTTPS")`},
}

func TestRules(t *testing.T) {
	for _, c := range ruleCases {
		t.Run(c.rule+"/"+c.name, func(t *testing.T) {
			fs := eval(t, c.body)
			if got := count(fs, c.rule); got != c.want {
				t.Fatalf("%s fired %d times, want %d; findings: %+v", c.rule, got, c.want, fs)
			}
		})
	}
}

func TestEveryRuleHasTests(t *testing.T) {
	pos, neg := map[string]bool{}, map[string]bool{}
	for _, c := range ruleCases {
		if c.want > 0 {
			pos[c.rule] = true
		} else {
			neg[c.rule] = true
		}
	}
	for _, r := range All() {
		if !pos[r.ID] || !neg[r.ID] {
			t.Errorf("rule %s needs a positive and a negative case", r.ID)
		}
	}
}

func TestRuleMetadata(t *testing.T) {
	idRe := regexp.MustCompile(`^CG\d{3}$`)
	nameRe := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	banned := regexp.MustCompile(`(?i)\b(certif|compliant|guarantee|confirms)`)
	seen := map[string]bool{}
	for _, r := range All() {
		if !idRe.MatchString(r.ID) || seen[r.ID] {
			t.Errorf("bad or duplicate id %q", r.ID)
		}
		seen[r.ID] = true
		if !nameRe.MatchString(r.Name) || r.Title == "" || r.Rationale == "" || r.Remediation == "" || r.check == nil {
			t.Errorf("%s: incomplete metadata", r.ID)
		}
		for _, s := range []string{r.Title, r.Rationale, r.Remediation} {
			if banned.MatchString(s) {
				t.Errorf("%s: wording must assess/evidence/propose, found %q", r.ID, banned.FindString(s))
			}
		}
		if r.Severity > Info && len(r.Clauses) == 0 {
			t.Errorf("%s: a non-info rule should cite at least one clause", r.ID)
		}
	}
}

func TestSeverity(t *testing.T) {
	for _, c := range []struct {
		in   string
		want Severity
		ok   bool
	}{{"high", High, true}, {" Medium ", Medium, true}, {"none", Critical + 1, true}, {"bogus", 0, false}} {
		got, ok := ParseSeverity(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseSeverity(%q) = %v %v", c.in, got, ok)
		}
	}
	if Severity(42).String() != "unknown" {
		t.Error("out-of-range severity")
	}
}

func TestFindingsSortedAndDeterministic(t *testing.T) {
	body := `Person(u, "U")
ContainerDb(d, "D", "", "", $tags="pii")
System_Ext(x, "X", "", $tags="critical")
Rel(u, d, "q", "HTTP")`
	a, b := eval(t, body), eval(t, body)
	if len(a) == 0 || len(a) != len(b) {
		t.Fatal("expected stable findings")
	}
	for i := range a {
		if a[i].RuleID != b[i].RuleID || a[i].Subject != b[i].Subject {
			t.Fatal("non-deterministic order")
		}
		if i > 0 && a[i].Severity > a[i-1].Severity {
			t.Fatal("not sorted by severity")
		}
	}
}
