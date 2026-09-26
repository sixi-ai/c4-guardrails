package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type jsonNode struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	External    bool              `json:"external"`
	Technology  string            `json:"technology"`
	Description string            `json:"description"`
	Boundary    string            `json:"boundary"`
	Tags        []string          `json:"tags"`
	Attrs       map[string]string `json:"attrs"`
}

type jsonEdge struct {
	From       string            `json:"from"`
	To         string            `json:"to"`
	Label      string            `json:"label"`
	Technology string            `json:"technology"`
	Protocol   string            `json:"protocol"`
	Auth       string            `json:"auth"`
	Encryption string            `json:"encryption"`
	Tags       []string          `json:"tags"`
	Attrs      map[string]string `json:"attrs"`
}

type jsonBoundary struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Parent string   `json:"parent"`
	Tags   []string `json:"tags"`
}

type jsonModel struct {
	Title      string         `json:"title"`
	Nodes      []jsonNode     `json:"nodes"`
	Edges      []jsonEdge     `json:"edges"`
	Boundaries []jsonBoundary `json:"boundaries"`
}

// kindOf maps JSON "type" values (and Mermaid macro names) to kind + external.
func kindOf(t string) (Kind, bool, bool) {
	t = strings.ToLower(strings.TrimSpace(t))
	if spec, ok := elementMacros[t]; ok {
		return spec.kind, spec.external, true
	}
	ext := strings.HasSuffix(t, "_ext") || strings.HasSuffix(t, "-ext")
	t = strings.TrimSuffix(strings.TrimSuffix(t, "_ext"), "-ext")
	switch t {
	case "person", "actor", "user":
		return KindPerson, ext, true
	case "system", "software_system":
		return KindSystem, ext, true
	case "container", "service", "app", "application":
		return KindContainer, ext, true
	case "component":
		return KindComponent, ext, true
	case "database", "db", "datastore", "store":
		return KindDatabase, ext, true
	case "queue", "topic", "bus":
		return KindQueue, ext, true
	}
	return "", false, false
}

// ParseJSON parses the simple JSON model format.
func ParseJSON(source string, src []byte) (*Model, error) {
	if len(src) > MaxInputBytes {
		return nil, &ParseError{Source: source, Msg: "input larger than 2 MiB"}
	}
	var jm jsonModel
	dec := json.NewDecoder(bytes.NewReader(src))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&jm); err != nil {
		return nil, &ParseError{Source: source, Msg: "invalid JSON model: " + err.Error()}
	}
	m := &Model{Source: source, Title: jm.Title}
	for i, jn := range jm.Nodes {
		k, ext, ok := kindOf(jn.Type)
		if !ok {
			return nil, &ParseError{Source: source, Msg: fmt.Sprintf("nodes[%d]: unknown type %q", i, sanitizeName(jn.Type))}
		}
		n := &Node{ID: jn.ID, Kind: k, External: ext || jn.External, Name: jn.Name, Technology: jn.Technology,
			Description: jn.Description, Boundary: jn.Boundary, Attrs: map[string]string{}}
		if n.Name == "" {
			n.Name = n.ID
		}
		for _, t := range jn.Tags {
			AddTag(t, &n.Tags, n.Attrs)
		}
		for k, v := range jn.Attrs {
			SetAttr(k, v, n.Attrs)
		}
		m.Nodes = append(m.Nodes, n)
	}
	for _, je := range jm.Edges {
		e := &Edge{From: je.From, To: je.To, Label: je.Label, Technology: je.Technology, Attrs: map[string]string{}}
		for _, t := range je.Tags {
			AddTag(t, &e.Tags, e.Attrs)
		}
		for k, v := range je.Attrs {
			SetAttr(k, v, e.Attrs)
		}
		if je.Protocol != "" {
			SetAttr("protocol", je.Protocol, e.Attrs)
			if e.Technology == "" {
				e.Technology = je.Protocol
			}
		}
		if je.Auth != "" {
			SetAttr("auth", je.Auth, e.Attrs)
		}
		if je.Encryption != "" {
			SetAttr("enc", je.Encryption, e.Attrs)
		}
		m.Edges = append(m.Edges, e)
	}
	for _, jb := range jm.Boundaries {
		b := &Boundary{ID: jb.ID, Name: jb.Name, Type: jb.Type, Parent: jb.Parent}
		if b.Name == "" {
			b.Name = b.ID
		}
		attrs := map[string]string{}
		for _, t := range jb.Tags {
			AddTag(t, &b.Tags, attrs)
		}
		m.Boundaries = append(m.Boundaries, b)
	}
	jsonLines(m, src)
	return m, Finalize(m)
}

// jsonLines gives nodes and edges an approximate source line: the line of
// the node's "id" key and of the n-th "from" key.
func jsonLines(m *Model, src []byte) {
	lineAt := func(off int) int { return bytes.Count(src[:off], []byte("\n")) + 1 }
	for _, n := range m.Nodes {
		re := regexp.MustCompile(`"id"\s*:\s*` + regexp.QuoteMeta(strconv.Quote(n.ID)))
		if loc := re.FindIndex(src); loc != nil {
			n.Line = lineAt(loc[0])
		}
	}
	froms := regexp.MustCompile(`"from"\s*:`).FindAllIndex(src, -1)
	for i, e := range m.Edges {
		if i < len(froms) {
			e.Line = lineAt(froms[i][0])
		}
	}
	for _, b := range m.Boundaries {
		re := regexp.MustCompile(`"id"\s*:\s*` + regexp.QuoteMeta(strconv.Quote(b.ID)))
		if loc := re.FindIndex(src); loc != nil {
			b.Line = lineAt(loc[0])
		}
	}
}

// Parse picks the parser from the file name: .json → JSON model, .md →
// the first ```mermaid block that is a C4 diagram, anything else → Mermaid.
func Parse(source string, src []byte) (*Model, error) {
	lower := strings.ToLower(source)
	switch {
	case strings.HasSuffix(lower, ".json"):
		return ParseJSON(source, src)
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		block, offset, ok := mermaidFromMarkdown(src)
		if !ok {
			return nil, &ParseError{Source: source, Msg: "no ```mermaid C4 block found"}
		}
		m, err := ParseMermaid(source, block)
		if err != nil {
			if pe, ok := err.(*ParseError); ok && pe.Line > 0 {
				pe.Line += offset
			}
			return nil, err
		}
		shiftLines(m, offset)
		return m, nil
	}
	return ParseMermaid(source, src)
}

func shiftLines(m *Model, offset int) {
	for _, n := range m.Nodes {
		n.Line += offset
	}
	for _, e := range m.Edges {
		e.Line += offset
	}
	for _, b := range m.Boundaries {
		b.Line += offset
	}
}

// mermaidFromMarkdown returns the first fenced mermaid block whose first
// statement is a C4 header, and the line offset of its first line.
func mermaidFromMarkdown(src []byte) ([]byte, int, bool) {
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```mermaid" {
			continue
		}
		var body []string
		j := i + 1
		for ; j < len(lines) && strings.TrimSpace(lines[j]) != "```"; j++ {
			body = append(body, lines[j])
		}
		for _, b := range body {
			t := strings.ToLower(strings.TrimSpace(b))
			if t == "" || strings.HasPrefix(t, "%%") {
				continue
			}
			if strings.HasPrefix(t, "c4") {
				return []byte(strings.Join(body, "\n")), i + 1, true
			}
			break
		}
		i = j
	}
	return nil, 0, false
}
