package model

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustMermaid(t *testing.T, src string) *Model {
	t.Helper()
	m, err := ParseMermaid("t.mmd", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}

func TestParseMermaidElements(t *testing.T) {
	src := `%% comment
C4Container
title  Demo, with comma
Person(u, "User, admin", "Uses it")
Person_Ext(p, "Partner")
System_Ext(x, "Ext API", "Third party", $tags="critical;region:eu")
Enterprise_Boundary(e, "Corp") {
  System_Boundary(s, "Sys") {
    Container(api, "API", "Go", "Serves #pii")
    ContainerDb(db, "DB", "PostgreSQL", "Rows", $tags="pii+at_rest:aes256")
    ContainerQueue(q, "Queue", "Kafka")
    Container(kv, "Secrets", "Azure Key Vault")
  }
}
Rel(u, api, "Calls (v2)", "HTTPS", $tags="auth:oidc")
BiRel(api, db, "Reads", "JDBC/TLS")
Rel_Back(q, api, "Publishes", "Kafka")
UpdateRelStyle(u, api, $textColor="red")
`
	m := mustMermaid(t, src)
	if m.Title != "Demo, with comma" {
		t.Errorf("title = %q", m.Title)
	}
	if got := len(m.Nodes); got != 7 {
		t.Fatalf("nodes = %d", got)
	}
	cases := []struct {
		id       string
		kind     Kind
		ext      bool
		boundary string
		tags     []string
		name     string
	}{
		{"u", KindPerson, false, "", nil, "User, admin"},
		{"p", KindPerson, true, "", nil, "Partner"},
		{"x", KindSystem, true, "", []string{"critical"}, "Ext API"},
		{"api", KindContainer, false, "s", []string{"pii"}, "API"},
		{"db", KindDatabase, false, "s", []string{"pii"}, "DB"},
		{"q", KindQueue, false, "s", nil, "Queue"},
		{"kv", KindContainer, false, "s", []string{"secrets"}, "Secrets"},
	}
	for _, c := range cases {
		n := m.Node(c.id)
		if n == nil {
			t.Fatalf("missing node %s", c.id)
		}
		if n.Kind != c.kind || n.External != c.ext || n.Boundary != c.boundary || n.Name != c.name {
			t.Errorf("%s: got kind=%s ext=%v boundary=%q name=%q", c.id, n.Kind, n.External, n.Boundary, n.Name)
		}
		if !reflect.DeepEqual(n.Tags, c.tags) {
			t.Errorf("%s: tags = %v, want %v", c.id, n.Tags, c.tags)
		}
	}
	if m.Node("x").Attr("region") != "eu" || m.Node("db").Attr("at_rest") != "aes256" {
		t.Errorf("attrs not parsed: %v %v", m.Node("x").Attrs, m.Node("db").Attrs)
	}
	if b := m.Boundary("s"); b == nil || b.Parent != "e" || b.Type != "system" {
		t.Errorf("boundary s = %+v", b)
	}
	if got := len(m.Edges); got != 4 { // BiRel expands to two
		t.Fatalf("edges = %d", got)
	}
	if e := m.Edges[0]; e.Label != "Calls (v2)" || e.Technology != "HTTPS" || e.Attr("auth") != "oidc" || e.Line != 15 {
		t.Errorf("edge 0 = %+v", e)
	}
	if e := m.Edges[2]; e.From != "db" || e.To != "api" {
		t.Errorf("BiRel reverse = %+v", e)
	}
	if e := m.Edges[3]; e.From != "api" || e.To != "q" {
		t.Errorf("Rel_Back = %+v", e)
	}
}

func TestParseMermaidErrors(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"no header", `Person(a, "A")`, "not a Mermaid C4 diagram"},
		{"unbalanced", "C4Context\n}", "unbalanced"},
		{"unclosed", "C4Context\nBoundary(b, \"B\") {\nPerson(a, \"A\")", "unclosed"},
		{"unterminated", "C4Context\nPerson(a, \"A)", "unterminated"},
		{"missing paren", "C4Context\nPerson(a, A", "missing ')'"},
		{"unknown ref", "C4Context\nPerson(a, \"A\")\nRel(a, b, \"x\")", "unknown element \"b\""},
		{"duplicate", "C4Context\nPerson(a, \"A\")\nSystem(a, \"B\")", "duplicate id"},
		{"rel arity", "C4Context\nPerson(a, \"A\")\nRel(a)", "needs at least"},
		{"no alias", "C4Context\nPerson()", "needs an alias"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseMermaid("t.mmd", []byte(c.src))
			var pe *ParseError
			if err == nil || !errors.As(err, &pe) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v, want %q", err, c.want)
			}
		})
	}
}

func TestParseMermaidTolerant(t *testing.T) {
	m := mustMermaid(t, "\ufeffC4Context\r\naccTitle: x\r\nwhatever this is\r\nFancyMacro(a)\r\nPerson(a, \"A \\\"quoted\\\"\")\r\nRelIndex(1, a, a, \"self\")\r\n")
	if len(m.Warnings) != 2 {
		t.Errorf("warnings = %v", m.Warnings)
	}
	if m.Node("a").Name != `A "quoted"` {
		t.Errorf("name = %q", m.Node("a").Name)
	}
	if len(m.Edges) != 1 || m.Edges[0].Label != "self" {
		t.Errorf("RelIndex edges = %+v", m.Edges)
	}
}

func TestTransportAndAuth(t *testing.T) {
	cases := []struct {
		tech, tags string
		tr         Transport
		auth       string
	}{
		{"HTTPS", "", TransportTLS, ""},
		{"HTTP", "", TransportPlaintext, ""},
		{"JDBC", "", TransportPlaintext, ""},
		{"JDBC/TLS", "", TransportTLS, ""},
		{"REST/JSON", "", TransportUnknown, ""},
		{"", "", TransportUnknown, ""},
		{"gRPC", "enc:mtls", TransportTLS, ""},
		{"HTTPS", "enc:none", TransportPlaintext, ""},
		{"ISO 20022 over mTLS", "", TransportTLS, "mtls"},
		{"HTTPS, OAuth2", "", TransportTLS, "oauth2"},
		{"HTTPS", "auth:managed_identity", TransportTLS, "managed_identity"},
		{"HTTPS", "authn=None", TransportTLS, "none"},
	}
	for _, c := range cases {
		e := &Edge{Technology: c.tech, Attrs: map[string]string{}}
		ApplyTags(c.tags, &e.Tags, e.Attrs)
		if tr, _ := e.Transport(); tr != c.tr {
			t.Errorf("%q %q: transport = %s, want %s", c.tech, c.tags, tr, c.tr)
		}
		if a := e.Auth(); a != c.auth {
			t.Errorf("%q %q: auth = %q, want %q", c.tech, c.tags, a, c.auth)
		}
	}
}

func TestTagVocabulary(t *testing.T) {
	var tags []string
	attrs := map[string]string{}
	ApplyTags("Personal-Data, LLM;HITL #vault +high_risk colour:blue Region=EU", &tags, attrs)
	want := []string{"pii", "ai", "oversight", "secrets", "high-risk"}
	if !reflect.DeepEqual(tags, want) {
		t.Errorf("tags = %v, want %v", tags, want)
	}
	if attrs["region"] != "eu" {
		t.Errorf("region = %q", attrs["region"])
	}
	if got := UnknownAttrs(attrs); !reflect.DeepEqual(got, []string{"colour"}) {
		t.Errorf("unknown = %v", got)
	}
}

func TestParseJSON(t *testing.T) {
	src := `{
  "title": "J",
  "boundaries": [{"id": "b", "name": "B", "tags": ["internet"]}],
  "nodes": [
    {"id": "u", "type": "person", "name": "U"},
    {"id": "db", "type": "database", "name": "DB", "boundary": "b", "tags": ["pii"], "attrs": {"encryption_at_rest": "AES256"}},
    {"id": "x", "type": "system_ext", "name": "X", "attrs": {"data": "personal"}}
  ],
  "edges": [
    {"from": "u", "to": "db", "label": "reads", "protocol": "HTTP", "auth": "none", "encryption": "none"}
  ]
}`
	m, err := ParseJSON("m.c4.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.Node("db").Attr("at_rest") != "aes256" || !m.Node("x").External || !m.Node("x").Is("pii") {
		t.Errorf("nodes = %+v %+v", m.Node("db"), m.Node("x"))
	}
	e := m.Edges[0]
	if e.Attr("auth") != "none" || e.Attr("enc") != "none" || e.Technology != "HTTP" || e.Line != 10 {
		t.Errorf("edge = %+v", e)
	}
	if m.Node("db").Line != 6 {
		t.Errorf("db line = %d", m.Node("db").Line)
	}
	for name, bad := range map[string]string{
		"unknown field": `{"nodes": [], "edgez": []}`,
		"unknown type":  `{"nodes": [{"id": "a", "type": "spaceship"}]}`,
		"bad ref":       `{"nodes": [{"id": "a", "type": "person"}], "edges": [{"from": "a", "to": "z"}]}`,
		"bad boundary":  `{"nodes": [{"id": "a", "type": "person", "boundary": "nope"}]}`,
		"not json":      `C4Context`,
	} {
		if _, err := ParseJSON("m.json", []byte(bad)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestParseMarkdown(t *testing.T) {
	src := "# Doc\n\n```mermaid\nflowchart LR\n  a-->b\n```\n\ntext\n\n```mermaid\nC4Context\nPerson(a, \"A\")\n```\n"
	m, err := Parse("doc.md", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if n := m.Node("a"); n == nil || n.Line != 12 {
		t.Fatalf("node = %+v", n)
	}
	if _, err := Parse("none.md", []byte("# nothing")); err == nil {
		t.Error("expected error for markdown without C4 block")
	}
}

func TestInputLimit(t *testing.T) {
	big := make([]byte, MaxInputBytes+1)
	if _, err := ParseMermaid("big.mmd", big); err == nil {
		t.Error("expected size error")
	}
	if _, err := ParseJSON("big.json", big); err == nil {
		t.Error("expected size error")
	}
}
