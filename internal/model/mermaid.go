package model

import (
	"fmt"
	"strings"
)

// MaxInputBytes bounds every input file.
const MaxInputBytes = 2 << 20

// elementSpec describes one Mermaid C4 element macro.
type elementSpec struct {
	kind     Kind
	external bool
	hasTech  bool // Container/Component style: (alias, label, techn, descr)
}

var elementMacros = map[string]elementSpec{
	"person": {KindPerson, false, false}, "person_ext": {KindPerson, true, false},
	"system": {KindSystem, false, false}, "system_ext": {KindSystem, true, false},
	"systemdb": {KindDatabase, false, false}, "systemdb_ext": {KindDatabase, true, false},
	"systemqueue": {KindQueue, false, false}, "systemqueue_ext": {KindQueue, true, false},
	"container": {KindContainer, false, true}, "container_ext": {KindContainer, true, true},
	"containerdb": {KindDatabase, false, true}, "containerdb_ext": {KindDatabase, true, true},
	"containerqueue": {KindQueue, false, true}, "containerqueue_ext": {KindQueue, true, true},
	"component": {KindComponent, false, true}, "component_ext": {KindComponent, true, true},
	"componentdb": {KindDatabase, false, true}, "componentdb_ext": {KindDatabase, true, true},
	"componentqueue": {KindQueue, false, true}, "componentqueue_ext": {KindQueue, true, true},
}

var relMacros = map[string]bool{
	"rel": true, "birel": true, "rel_u": true, "rel_up": true, "rel_d": true, "rel_down": true,
	"rel_l": true, "rel_left": true, "rel_r": true, "rel_right": true, "rel_back": true, "relindex": true,
}

var boundaryMacros = map[string]string{
	"boundary": "", "enterprise_boundary": "enterprise", "system_boundary": "system",
	"container_boundary": "container", "deployment_node": "node", "node": "node",
	"node_l": "node", "node_r": "node",
}

var ignoredMacros = map[string]bool{
	"updateelementstyle": true, "updaterelstyle": true, "updatelayoutconfig": true,
	"updateboundarystyle": true, "addelementtag": true, "addreltag": true,
}

// ParseError is a fatal input error with a line number.
type ParseError struct {
	Source string
	Line   int
	Msg    string
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Source, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.Source, e.Msg)
}

// ParseMermaid parses a Mermaid C4 diagram (C4Context, C4Container,
// C4Component, C4Dynamic, C4Deployment).
func ParseMermaid(source string, src []byte) (*Model, error) {
	if len(src) > MaxInputBytes {
		return nil, &ParseError{Source: source, Msg: "input larger than 2 MiB"}
	}
	m := &Model{Source: source}
	var stack []string // open boundary ids
	seenHeader := false
	lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
	for i, raw := range lines {
		ln := i + 1
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "\ufeff") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		}
		if line == "" || strings.HasPrefix(line, "%%") {
			continue
		}
		// Closing braces, possibly several.
		for strings.HasPrefix(line, "}") {
			if len(stack) == 0 {
				return nil, &ParseError{source, ln, "unbalanced '}'"}
			}
			stack = stack[:len(stack)-1]
			line = strings.TrimSpace(line[1:])
		}
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "c4context"), strings.HasPrefix(lower, "c4container"),
			strings.HasPrefix(lower, "c4component"), strings.HasPrefix(lower, "c4dynamic"),
			strings.HasPrefix(lower, "c4deployment"):
			seenHeader = true
			continue
		case strings.HasPrefix(lower, "title "), lower == "title":
			m.Title = strings.TrimSpace(line[len("title"):])
			continue
		case strings.HasPrefix(lower, "acctitle"), strings.HasPrefix(lower, "accdescr"):
			continue
		}
		name, args, rest, err := splitCall(line)
		if err != nil {
			return nil, &ParseError{source, ln, err.Error()}
		}
		if name == "" {
			m.Warnings = append(m.Warnings, fmt.Sprintf("%s:%d: unrecognised statement ignored", source, ln))
			continue
		}
		key := strings.ToLower(name)
		pos, named := splitNamed(args)
		arg := func(i int) string {
			if i < len(pos) {
				return pos[i]
			}
			return ""
		}
		switch {
		case ignoredMacros[key]:
		case relMacros[key]:
			if key == "relindex" && len(pos) > 0 {
				pos = pos[1:]
			}
			if len(pos) < 2 {
				return nil, &ParseError{source, ln, name + " needs at least (from, to)"}
			}
			e := &Edge{From: arg(0), To: arg(1), Label: arg(2), Technology: arg(3), Attrs: map[string]string{}, Line: ln}
			if key == "rel_back" {
				e.From, e.To = e.To, e.From
			}
			if v, ok := named["techn"]; ok {
				e.Technology = v
			}
			if v, ok := named["label"]; ok {
				e.Label = v
			}
			ApplyTags(named["tags"], &e.Tags, e.Attrs)
			applyInline(arg(4)+" "+named["descr"], &e.Tags, e.Attrs)
			m.Edges = append(m.Edges, e)
			if key == "birel" {
				back := *e
				back.From, back.To = e.To, e.From
				back.Tags = append([]string(nil), e.Tags...)
				back.Attrs = copyMap(e.Attrs)
				m.Edges = append(m.Edges, &back)
			}
		default:
			if spec, ok := elementMacros[key]; ok {
				if len(pos) < 1 || pos[0] == "" {
					return nil, &ParseError{source, ln, name + " needs an alias"}
				}
				n := &Node{ID: arg(0), Kind: spec.kind, External: spec.external, Name: arg(1), Attrs: map[string]string{}, Line: ln}
				if spec.hasTech {
					n.Technology, n.Description = arg(2), arg(3)
				} else {
					n.Description = arg(2)
				}
				if v, ok := named["techn"]; ok {
					n.Technology = v
				}
				if v, ok := named["descr"]; ok {
					n.Description = v
				}
				if n.Name == "" {
					n.Name = n.ID
				}
				if len(stack) > 0 {
					n.Boundary = stack[len(stack)-1]
				}
				ApplyTags(named["tags"], &n.Tags, n.Attrs)
				applyInline(n.Description, &n.Tags, n.Attrs)
				m.Nodes = append(m.Nodes, n)
				break
			}
			if btype, ok := boundaryMacros[key]; ok {
				if len(pos) < 1 || pos[0] == "" {
					return nil, &ParseError{source, ln, name + " needs an alias"}
				}
				b := &Boundary{ID: arg(0), Name: arg(1), Type: btype, Line: ln}
				if btype == "" {
					b.Type = arg(2)
				}
				if b.Name == "" {
					b.Name = b.ID
				}
				if len(stack) > 0 {
					b.Parent = stack[len(stack)-1]
				}
				attrs := map[string]string{}
				ApplyTags(named["tags"], &b.Tags, attrs)
				m.Boundaries = append(m.Boundaries, b)
				if strings.HasPrefix(strings.TrimSpace(rest), "{") {
					stack = append(stack, b.ID)
					rest = strings.TrimSpace(strings.TrimSpace(rest)[1:])
					if strings.HasPrefix(rest, "}") {
						stack = stack[:len(stack)-1]
					}
				}
				break
			}
			m.Warnings = append(m.Warnings, fmt.Sprintf("%s:%d: unknown macro %q ignored", source, ln, sanitizeName(name)))
		}
	}
	if len(stack) != 0 {
		return nil, &ParseError{source, len(lines), "unclosed boundary '{'"}
	}
	if !seenHeader {
		return nil, &ParseError{source, 0, "not a Mermaid C4 diagram (expected C4Context, C4Container, C4Component, C4Dynamic or C4Deployment)"}
	}
	return m, Finalize(m)
}

// applyInline reads "#tag" and "#key:value" words from free text.
func applyInline(text string, tags *[]string, attrs map[string]string) {
	for _, w := range strings.Fields(text) {
		if strings.HasPrefix(w, "#") && len(w) > 1 {
			AddTag(strings.TrimRight(w, ".,;)"), tags, attrs)
		}
	}
}

func copyMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func sanitizeName(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// splitCall splits `Name(arg, "arg", $k="v") rest` into its parts. A line
// that is not a call returns name == "".
func splitCall(line string) (name string, args []string, rest string, err error) {
	open := strings.IndexByte(line, '(')
	if open <= 0 {
		return "", nil, "", nil
	}
	name = strings.TrimSpace(line[:open])
	for _, r := range name {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "", nil, "", nil
		}
	}
	var cur strings.Builder
	inQ, escaped, depth := false, false, 0
	started := false
	for i := open + 1; i < len(line); i++ {
		c := line[i]
		switch {
		case inQ && escaped:
			cur.WriteByte(c)
			escaped = false
		case inQ && c == '\\':
			escaped = true
		case c == '"':
			inQ = !inQ
			started = true
			if inQ {
				cur.WriteByte(0) // marker: quoted content follows
			}
		case inQ:
			cur.WriteByte(c)
		case c == '(':
			depth++
			cur.WriteByte(c)
		case c == ')' && depth > 0:
			depth--
			cur.WriteByte(c)
		case c == ')':
			if started || cur.Len() > 0 || len(args) > 0 {
				args = append(args, cleanArg(cur.String()))
			}
			return name, args, line[i+1:], nil
		case c == ',':
			args = append(args, cleanArg(cur.String()))
			cur.Reset()
			started = false
		default:
			cur.WriteByte(c)
		}
	}
	if inQ {
		return "", nil, "", fmt.Errorf("unterminated string in %s(...)", sanitizeName(name))
	}
	return "", nil, "", fmt.Errorf("missing ')' in %s(...)", sanitizeName(name))
}

// cleanArg trims an argument; quoted parts keep inner whitespace.
func cleanArg(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		prefix := strings.TrimSpace(s[:i])
		return prefix + strings.ReplaceAll(s[i+1:], "\x00", "")
	}
	return strings.TrimSpace(s)
}

// splitNamed separates positional arguments from $name="value" arguments.
func splitNamed(args []string) ([]string, map[string]string) {
	named := map[string]string{}
	var pos []string
	for _, a := range args {
		if strings.HasPrefix(a, "$") {
			if i := strings.IndexByte(a, '='); i > 0 {
				named[strings.ToLower(strings.TrimSpace(a[1:i]))] = strings.TrimSpace(a[i+1:])
				continue
			}
		}
		pos = append(pos, a)
	}
	return pos, named
}

// Finalize applies inferences and validates references. Both parsers call it.
func Finalize(m *Model) error {
	seen := map[string]int{}
	for _, n := range m.Nodes {
		if n.ID == "" {
			return &ParseError{m.Source, n.Line, "element without id"}
		}
		if prev, dup := seen[n.ID]; dup {
			return &ParseError{m.Source, n.Line, fmt.Sprintf("duplicate id %q (first defined on line %d)", sanitizeName(n.ID), prev)}
		}
		seen[n.ID] = n.Line
		if n.Attrs == nil {
			n.Attrs = map[string]string{}
		}
		InferNodeTags(n)
		if d := n.Attr("data"); strings.Contains(d, "personal") || strings.Contains(d, "pii") {
			AddTag("pii", &n.Tags, n.Attrs)
		}
		if n.Is("encrypted") && n.Attr("at_rest") == "" {
			n.Attrs["at_rest"] = "declared"
		}
	}
	bseen := map[string]bool{}
	for _, b := range m.Boundaries {
		if bseen[b.ID] {
			return &ParseError{m.Source, b.Line, fmt.Sprintf("duplicate boundary id %q", sanitizeName(b.ID))}
		}
		bseen[b.ID] = true
	}
	for _, n := range m.Nodes {
		if n.Boundary != "" && !bseen[n.Boundary] {
			return &ParseError{m.Source, n.Line, fmt.Sprintf("element %q refers to unknown boundary %q", sanitizeName(n.ID), sanitizeName(n.Boundary))}
		}
	}
	for _, e := range m.Edges {
		if e.Attrs == nil {
			e.Attrs = map[string]string{}
		}
		for _, id := range []string{e.From, e.To} {
			if _, ok := seen[id]; !ok {
				return &ParseError{m.Source, e.Line, fmt.Sprintf("relationship refers to unknown element %q", sanitizeName(id))}
			}
		}
	}
	m.Index()
	return nil
}
