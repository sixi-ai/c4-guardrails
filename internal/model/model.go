// Package model holds the small typed graph that every input format is
// parsed into. Rules and reports only ever see this model.
package model

import (
	"sort"
	"strings"
)

// Kind is the normalised C4 element kind.
type Kind string

// Element kinds.
const (
	KindPerson    Kind = "person"
	KindSystem    Kind = "system"
	KindContainer Kind = "container"
	KindComponent Kind = "component"
	KindDatabase  Kind = "database"
	KindQueue     Kind = "queue"
)

// Node is a C4 element (person, system, container, database, ...).
type Node struct {
	ID          string            `json:"id"`
	Kind        Kind              `json:"type"`
	Name        string            `json:"name"`
	External    bool              `json:"external,omitempty"`
	Technology  string            `json:"technology,omitempty"`
	Description string            `json:"description,omitempty"`
	Boundary    string            `json:"boundary,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Attrs       map[string]string `json:"attrs,omitempty"`
	Line        int               `json:"line,omitempty"`
}

// Edge is a relationship between two nodes.
type Edge struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Label      string            `json:"label,omitempty"`
	Technology string            `json:"technology,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
	Line       int               `json:"line,omitempty"`
}

// Boundary groups nodes (Enterprise_Boundary, System_Boundary, ...).
type Boundary struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Type   string   `json:"type,omitempty"`
	Parent string   `json:"parent,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Line   int      `json:"line,omitempty"`
}

// Model is one parsed diagram.
type Model struct {
	Source     string      `json:"source"`
	Title      string      `json:"title,omitempty"`
	Nodes      []*Node     `json:"nodes"`
	Edges      []*Edge     `json:"edges"`
	Boundaries []*Boundary `json:"boundaries,omitempty"`
	// Warnings are non-fatal parser notes (unrecognised statements).
	Warnings []string `json:"warnings,omitempty"`

	byID  map[string]*Node
	bByID map[string]*Boundary
}

// Index (re)builds lookup tables. Parsers call it once the model is complete.
func (m *Model) Index() {
	m.byID = make(map[string]*Node, len(m.Nodes))
	for _, n := range m.Nodes {
		m.byID[n.ID] = n
	}
	m.bByID = make(map[string]*Boundary, len(m.Boundaries))
	for _, b := range m.Boundaries {
		m.bByID[b.ID] = b
	}
}

// Node returns the node with the given id, or nil.
func (m *Model) Node(id string) *Node {
	if m.byID == nil {
		m.Index()
	}
	return m.byID[id]
}

// Boundary returns the boundary with the given id, or nil.
func (m *Model) Boundary(id string) *Boundary {
	if m.bByID == nil {
		m.Index()
	}
	return m.bByID[id]
}

// Any reports whether any node carries the canonical tag.
func (m *Model) Any(tag string) bool {
	for _, n := range m.Nodes {
		if n.Is(tag) {
			return true
		}
	}
	return false
}

// Is reports whether the node carries the canonical tag.
func (n *Node) Is(tag string) bool { return hasTag(n.Tags, tag) }

// Attr returns the canonical attribute value ("" when not declared).
func (n *Node) Attr(key string) string { return n.Attrs[key] }

// Is reports whether the edge carries the canonical tag.
func (e *Edge) Is(tag string) bool { return hasTag(e.Tags, tag) }

// Attr returns the canonical attribute value ("" when not declared).
func (e *Edge) Attr(key string) string { return e.Attrs[key] }

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// IsDatabase reports whether the node stores data.
func (n *Node) IsDatabase() bool { return n.Kind == KindDatabase }

// IsPerson reports whether the node is a person.
func (n *Node) IsPerson() bool { return n.Kind == KindPerson }

// --- vocabulary -------------------------------------------------------------

// tagAliases maps accepted spellings to canonical tags.
var tagAliases = map[string]string{
	"pii": "pii", "personal-data": "pii", "personal_data": "pii", "personaldata": "pii",
	"ai": "ai", "llm": "ai", "genai": "ai", "ml": "ai", "ai-system": "ai",
	"high-risk": "high-risk", "high_risk": "high-risk", "highrisk": "high-risk",
	"logging": "logging", "monitoring": "logging", "siem": "logging", "observability": "logging", "log-store": "logging",
	"secrets": "secrets", "vault": "secrets", "kms": "secrets", "secret-store": "secrets", "secrets-store": "secrets",
	"oversight": "oversight", "human-oversight": "oversight", "human-in-the-loop": "oversight", "hitl": "oversight", "human-review": "oversight",
	"records": "records", "record-keeping": "records", "audit-log": "records", "ai-log": "records",
	"critical": "critical", "critical-function": "critical",
	"internet": "internet", "public": "internet", "dmz": "internet",
	"encrypted": "encrypted",
}

// attrAliases maps accepted attribute keys to canonical keys.
var attrAliases = map[string]string{
	"auth": "auth", "authn": "auth", "authentication": "auth",
	"enc": "enc", "encryption": "enc", "transport": "enc",
	"at_rest": "at_rest", "at-rest": "at_rest", "encryption_at_rest": "at_rest", "rest": "at_rest",
	"exit": "exit", "exit_plan": "exit", "alternative": "exit", "fallback": "exit",
	"region": "region", "location": "region", "data_location": "region",
	"protocol": "protocol",
	"data":     "data",
}

// KnownAttr reports whether key is part of the attribute vocabulary.
func KnownAttr(key string) bool { _, ok := attrAliases[strings.ToLower(key)]; return ok }

// ApplyTags splits a raw tag string ("pii;auth:oauth2, critical") and merges
// it into tags/attrs. Unknown attribute keys are kept under "?key" so the
// missing-fact rule can report them.
func ApplyTags(raw string, tags *[]string, attrs map[string]string) {
	for _, tok := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '+' || r == ' ' || r == '\t'
	}) {
		AddTag(tok, tags, attrs)
	}
}

// AddTag merges one "tag" or "key:value" / "key=value" token.
func AddTag(tok string, tags *[]string, attrs map[string]string) {
	tok = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tok), "#"))
	if tok == "" {
		return
	}
	if i := strings.IndexAny(tok, ":="); i > 0 {
		SetAttr(tok[:i], tok[i+1:], attrs)
		return
	}
	t := strings.ToLower(tok)
	if c, ok := tagAliases[t]; ok {
		t = c
	}
	if !hasTag(*tags, t) {
		*tags = append(*tags, t)
	}
}

// SetAttr stores a key/value using the canonical key.
func SetAttr(k, v string, attrs map[string]string) {
	k = strings.ToLower(strings.TrimSpace(k))
	v = strings.TrimSpace(v)
	if k == "" {
		return
	}
	if c, ok := attrAliases[k]; ok {
		attrs[c] = strings.ToLower(v)
		return
	}
	attrs["?"+k] = v
}

// UnknownAttrs returns the attribute keys that are not in the vocabulary.
func UnknownAttrs(attrs map[string]string) []string {
	var out []string
	for k := range attrs {
		if strings.HasPrefix(k, "?") {
			out = append(out, k[1:])
		}
	}
	sort.Strings(out)
	return out
}

// InferNodeTags adds tags implied by well-known technology names. It is a
// deliberately small, documented list.
func InferNodeTags(n *Node) {
	words := Words(n.Technology + " " + n.Name)
	has := func(ws ...string) bool {
		for _, w := range ws {
			if words[w] {
				return true
			}
		}
		return false
	}
	if has("vault", "kms", "keyvault", "hsm") || (words["secrets"] && words["manager"]) || (words["key"] && words["vault"]) {
		AddTag("secrets", &n.Tags, n.Attrs)
	}
	if has("siem", "splunk", "loki", "opentelemetry", "elasticsearch", "sentinel") || (words["log"] && words["analytics"]) {
		AddTag("logging", &n.Tags, n.Attrs)
	}
}

// Words lower-cases s and splits it on anything that is not a letter or digit.
func Words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	}) {
		out[w] = true
	}
	return out
}

// --- derived edge facts -----------------------------------------------------

// Transport is the derived in-transit protection of an edge.
type Transport string

// Transport values.
const (
	TransportTLS       Transport = "tls"
	TransportPlaintext Transport = "plaintext"
	TransportUnknown   Transport = "unknown"
)

var tlsWords = []string{"tls", "mtls", "https", "ssl", "wss", "sftp", "ftps", "ssh", "ipsec", "vpn",
	"smtps", "ldaps", "amqps", "mqtts", "rediss", "grpcs"}
var plainWords = []string{"http", "jdbc", "odbc", "amqp", "mqtt", "ftp", "telnet", "ws", "smtp", "ldap",
	"redis", "tcp", "sql", "postgres", "postgresql", "mysql", "oracle", "mssql", "tds", "snmp", "nfs", "smb"}

// Transport derives in-transit protection: an explicit enc attribute wins,
// otherwise the technology label is inspected.
func (e *Edge) Transport() (Transport, string) {
	switch v := e.Attr("enc"); v {
	case "":
	case "none", "plaintext", "off", "false", "no":
		return TransportPlaintext, "declared enc:" + v
	default:
		return TransportTLS, "declared enc:" + v
	}
	words := Words(e.Technology + " " + e.Attr("protocol"))
	for _, w := range tlsWords {
		if words[w] {
			return TransportTLS, "technology mentions " + strings.ToUpper(w)
		}
	}
	for _, w := range plainWords {
		if words[w] {
			return TransportPlaintext, strings.ToUpper(w) + " declared without TLS"
		}
	}
	return TransportUnknown, ""
}

var authWords = map[string]string{"oauth": "oauth2", "oauth2": "oauth2", "oidc": "oidc", "jwt": "jwt",
	"mtls": "mtls", "saml": "saml", "kerberos": "kerberos", "apikey": "api_key"}

// Auth returns the declared authentication ("" when not declared). An
// explicit auth attribute wins; otherwise the technology label is inspected.
func (e *Edge) Auth() string {
	if v := e.Attr("auth"); v != "" {
		return v
	}
	words := Words(e.Technology)
	for _, w := range []string{"mtls", "oauth2", "oauth", "oidc", "jwt", "saml", "kerberos", "apikey"} {
		if words[w] {
			return authWords[w]
		}
	}
	return ""
}

// NoAuth reports whether an auth value explicitly declares no authentication.
func NoAuth(v string) bool {
	switch v {
	case "none", "anonymous", "no", "false", "off":
		return true
	}
	return false
}

// CredentialAuth reports whether an auth value relies on a shared secret that
// has to be stored somewhere.
func CredentialAuth(v string) bool {
	switch strings.ReplaceAll(v, "-", "_") {
	case "api_key", "apikey", "password", "basic", "secret", "shared_secret", "sql_login",
		"credentials", "connection_string", "client_secret":
		return true
	}
	return false
}

// Declined reports whether a node attribute value explicitly says "none".
func Declined(v string) bool {
	switch v {
	case "none", "no", "false", "off", "plaintext":
		return true
	}
	return false
}
