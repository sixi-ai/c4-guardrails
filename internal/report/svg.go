package report

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"

	"github.com/rbrus/c4-guardrails/internal/model"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

// Layout constants (SVG user units).
const (
	nodeW   = 210.0
	nodeH   = 92.0
	colW    = 360.0
	rowH    = 124.0
	laneGap = 58.0
	margin  = 36.0
)

type box struct{ x, y, w, h float64 }

type laidOut struct {
	pos    map[string]box
	rank   map[string]int
	lanes  []laneBox
	bounds []boundBox
	w, h   float64
}

type laneBox struct {
	key string
	box box
}

type boundBox struct {
	b     *model.Boundary
	box   box
	depth int
}

// rank assigns each node a column: longest path from a source after
// dropping back-edges found by depth-first search (declaration order).
func rank(m *model.Model) map[string]int {
	adj := map[string][]string{}
	for _, e := range m.Edges {
		if e.From != e.To {
			adj[e.From] = append(adj[e.From], e.To)
		}
	}
	state := map[string]int{} // 0 new, 1 on stack, 2 done
	forward := map[string][]string{}
	var dfs func(string)
	dfs = func(u string) {
		state[u] = 1
		for _, v := range adj[u] {
			switch state[v] {
			case 0:
				forward[u] = append(forward[u], v)
				dfs(v)
			case 2:
				forward[u] = append(forward[u], v)
			} // state 1: back-edge, dropped
		}
		state[u] = 2
	}
	// Persons first so they become sources.
	order := append([]*model.Node(nil), m.Nodes...)
	sort.SliceStable(order, func(i, j int) bool { return order[i].IsPerson() && !order[j].IsPerson() })
	for _, n := range order {
		if state[n.ID] == 0 {
			dfs(n.ID)
		}
	}
	r := map[string]int{}
	// Kahn over forward edges, longest path.
	indeg := map[string]int{}
	for _, vs := range forward {
		for _, v := range vs {
			indeg[v]++
		}
	}
	var queue []string
	for _, n := range order {
		if indeg[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range forward[u] {
			if r[u]+1 > r[v] {
				r[v] = r[u] + 1
			}
			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	return r
}

// layout places nodes in columns (rank) and horizontal lanes (boundary).
func layout(m *model.Model) laidOut {
	r := rank(m)
	// Lane order: persons without boundary, boundaries in tree order, rest.
	var laneKeys []string
	children := map[string][]*model.Boundary{}
	for _, b := range m.Boundaries {
		p := b.Parent
		if m.Boundary(p) == nil || p == b.ID {
			p = ""
		}
		children[p] = append(children[p], b)
	}
	depth := map[string]int{}
	visited := map[string]bool{}
	var walk func(parent string, d int)
	walk = func(parent string, d int) {
		for _, b := range children[parent] {
			if visited[b.ID] {
				continue
			}
			visited[b.ID] = true
			depth[b.ID] = d
			laneKeys = append(laneKeys, "b:"+b.ID)
			walk(b.ID, d+1)
		}
	}
	laneKeys = append(laneKeys, "persons")
	walk("", 0)
	laneKeys = append(laneKeys, "outside")
	laneOf := func(n *model.Node) string {
		if n.Boundary != "" && visited[n.Boundary] {
			return "b:" + n.Boundary
		}
		if n.IsPerson() {
			return "persons"
		}
		return "outside"
	}
	// cells[lane][col] = ordered node ids
	cells := map[string]map[int][]string{}
	maxCol := 0
	for _, n := range m.Nodes {
		l := laneOf(n)
		if cells[l] == nil {
			cells[l] = map[int][]string{}
		}
		cells[l][r[n.ID]] = append(cells[l][r[n.ID]], n.ID)
		if r[n.ID] > maxCol {
			maxCol = r[n.ID]
		}
	}
	// Barycenter ordering inside each cell (two sweeps).
	nbr := map[string][]string{}
	for _, e := range m.Edges {
		nbr[e.From] = append(nbr[e.From], e.To)
		nbr[e.To] = append(nbr[e.To], e.From)
	}
	slot := map[string]float64{}
	assign := func() {
		for li, l := range laneKeys {
			for _, ids := range cells[l] {
				for i, id := range ids {
					slot[id] = float64(li)*100 + float64(i)
				}
			}
		}
	}
	assign()
	for sweep := 0; sweep < 2; sweep++ {
		for _, l := range laneKeys {
			for c, ids := range cells[l] {
				bc := map[string]float64{}
				for _, id := range ids {
					sum, k := 0.0, 0.0
					for _, v := range nbr[id] {
						sum += slot[v]
						k++
					}
					if k == 0 {
						bc[id] = slot[id]
					} else {
						bc[id] = sum / k
					}
				}
				sort.SliceStable(ids, func(i, j int) bool { return bc[ids[i]] < bc[ids[j]] })
				cells[l][c] = ids
			}
		}
		assign()
	}
	// Geometry.
	out := laidOut{pos: map[string]box{}, rank: r}
	y := margin + 8
	laneRect := map[string]box{}
	prevDepth := -1
	for _, l := range laneKeys {
		cs := cells[l]
		if len(cs) == 0 {
			continue
		}
		d := 0
		if strings.HasPrefix(l, "b:") {
			d = depth[l[2:]] + 1
		}
		if d > prevDepth && prevDepth >= 0 {
			y += 26 * float64(d-prevDepth)
		}
		prevDepth = d
		rows := 0
		minC, maxC := math.MaxInt, -1
		for c, ids := range cs {
			if len(ids) > rows {
				rows = len(ids)
			}
			if c < minC {
				minC = c
			}
			if c > maxC {
				maxC = c
			}
		}
		top := y + 30
		for c, ids := range cs {
			for i, id := range ids {
				// centre shorter stacks vertically
				off := float64(rows-len(ids)) * rowH / 2
				out.pos[id] = box{margin + 20 + float64(c)*colW, top + off + float64(i)*rowH, nodeW, nodeH}
			}
		}
		h := 30 + float64(rows-1)*rowH + nodeH + 18
		lr := box{margin + float64(minC)*colW, y, float64(maxC-minC)*colW + nodeW + 40, h}
		laneRect[l] = lr
		out.lanes = append(out.lanes, laneBox{l, lr})
		y += h + laneGap
	}
	// Boundary rectangles: own lane plus descendants, padded by depth.
	var rectOf func(b *model.Boundary) (box, bool)
	memo := map[string]box{}
	rectOf = func(b *model.Boundary) (box, bool) {
		if v, ok := memo[b.ID]; ok {
			return v, true
		}
		var acc box
		have := false
		if lr, ok := laneRect["b:"+b.ID]; ok {
			acc, have = lr, true
		}
		for _, ch := range children[b.ID] {
			if cr, ok := rectOf(ch); ok {
				cr = box{cr.x - 14, cr.y - 26, cr.w + 28, cr.h + 40}
				if !have {
					acc, have = cr, true
				} else {
					acc = union(acc, cr)
				}
			}
		}
		if have {
			memo[b.ID] = acc
		}
		return acc, have
	}
	for _, b := range m.Boundaries {
		if !visited[b.ID] {
			continue
		}
		if bb, ok := rectOf(b); ok {
			out.bounds = append(out.bounds, boundBox{b, bb, depth[b.ID]})
		}
	}
	sort.SliceStable(out.bounds, func(i, j int) bool { return out.bounds[i].depth < out.bounds[j].depth })
	maxX, maxY := 0.0, 0.0
	for _, p := range out.pos {
		maxX = math.Max(maxX, p.x+p.w)
		maxY = math.Max(maxY, p.y+p.h)
	}
	for _, bb := range out.bounds {
		maxX = math.Max(maxX, bb.box.x+bb.box.w)
		maxY = math.Max(maxY, bb.box.y+bb.box.h)
	}
	out.w = maxX + margin
	out.h = maxY + margin
	return out
}

func union(a, b box) box {
	x1, y1 := math.Min(a.x, b.x), math.Min(a.y, b.y)
	x2, y2 := math.Max(a.x+a.w, b.x+b.w), math.Max(a.y+a.h, b.y+b.h)
	return box{x1, y1, x2 - x1, y2 - y1}
}

func esc(s string, max int) string { return html.EscapeString(OneLine(s, max)) }

// Highlights collects which elements and relationships carry findings.
type highlights struct {
	nodes map[string][]string // id → rule ids
	edges map[int][]string
	info  map[string]bool // rule ids that are info only
}

func collect(fs []rules.Finding) highlights {
	h := highlights{nodes: map[string][]string{}, edges: map[int][]string{}, info: map[string]bool{}}
	add := func(list []string, id string) []string {
		for _, x := range list {
			if x == id {
				return list
			}
		}
		return append(list, id)
	}
	for _, f := range fs {
		if f.Severity == rules.Info {
			h.info[f.RuleID] = true
		}
		if len(f.Edges) > 0 {
			for _, e := range f.Edges {
				h.edges[e] = add(h.edges[e], f.RuleID)
			}
			continue
		}
		if len(f.Nodes) == 1 {
			h.nodes[f.Nodes[0]] = add(h.nodes[f.Nodes[0]], f.RuleID)
		}
	}
	return h
}

func (h highlights) serious(ids []string) bool {
	for _, id := range ids {
		if !h.info[id] {
			return true
		}
	}
	return false
}

// DiagramSVG renders the model as an inline SVG with offending elements and
// relationships outlined in red.
func DiagramSVG(m *model.Model, fs []rules.Finding) string {
	lo := layout(m)
	hl := collect(fs)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.0f %.0f" width="%.0f" height="%.0f" role="img" aria-label="%s" font-family="Inter, Segoe UI, Helvetica, Arial, sans-serif">`,
		lo.w, lo.h, lo.w, lo.h, esc("Diagram of "+firstNonEmpty(m.Title, m.Source), 120))
	b.WriteString(`<defs>` +
		`<marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#57606a"/></marker>` +
		`<marker id="ahr" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#cf222e"/></marker>` +
		`<marker id="aha" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0,0 L10,5 L0,10 z" fill="#9a6700"/></marker>` +
		`<filter id="sh" x="-10%" y="-10%" width="120%" height="130%"><feDropShadow dx="0" dy="2" stdDeviation="2.5" flood-opacity="0.18"/></filter>` +
		`</defs>`)
	// Boundaries.
	for _, bb := range lo.bounds {
		label := bb.b.Name
		if bb.b.Type != "" {
			label += " [" + bb.b.Type + "]"
		}
		fmt.Fprintf(&b, `<g class="boundary"><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="10" fill="none" stroke="#8c959f" stroke-width="1.4" stroke-dasharray="7 5"/><text x="%.1f" y="%.1f" font-size="13" font-weight="600" fill="#57606a">%s</text></g>`,
			bb.box.x, bb.box.y, bb.box.w, bb.box.h, bb.box.x+12, bb.box.y+19, esc(label, 60))
	}
	// Edges below nodes; labels drawn afterwards so they sit on top.
	type lbl struct {
		x, y   float64
		text   string
		tech   string
		badges []string
		color  string
	}
	var labels []lbl
	for i, e := range m.Edges {
		s, okS := lo.pos[e.From]
		t, okT := lo.pos[e.To]
		if !okS || !okT {
			continue
		}
		rs, rt := lo.rank[e.From], lo.rank[e.To]
		var x1, y1, x2, y2, c1x, c1y, c2x, c2y float64
		switch {
		case e.From == e.To:
			x1, y1 = s.x+s.w*0.7, s.y
			x2, y2 = s.x+s.w, s.y+s.h*0.3
			c1x, c1y, c2x, c2y = x1, y1-50, x2+50, y2
		case rt > rs:
			x1, y1 = s.x+s.w, s.y+s.h/2
			x2, y2 = t.x, t.y+t.h/2
			dx := (x2 - x1) * 0.45
			c1x, c1y, c2x, c2y = x1+dx, y1, x2-dx, y2
		case rt < rs:
			x1, y1 = s.x, s.y+s.h/2
			x2, y2 = t.x+t.w, t.y+t.h/2
			dx := math.Max((x1-x2)*0.45, 60)
			c1x, c1y, c2x, c2y = x1-dx, y1, x2+dx, y2
		default: // same column
			if t.y > s.y {
				x1, y1 = s.x+s.w/2, s.y+s.h
				x2, y2 = t.x+t.w/2, t.y
			} else {
				x1, y1 = s.x+s.w/2, s.y
				x2, y2 = t.x+t.w/2, t.y+t.h
			}
			c1x, c1y, c2x, c2y = x1+40, (y1+y2)/2, x2+40, (y1+y2)/2
		}
		stroke, width, marker, dash := "#57606a", 1.6, "ah", ""
		color := ""
		if ids := hl.edges[i]; len(ids) > 0 {
			if hl.serious(ids) {
				stroke, width, marker, color = "#cf222e", 3.2, "ahr", "#cf222e"
			} else {
				stroke, width, marker, dash, color = "#9a6700", 2.2, "aha", ` stroke-dasharray="6 4"`, "#9a6700"
			}
		}
		cls := "edge"
		if color != "" {
			cls = "edge flagged"
		}
		fmt.Fprintf(&b, `<path class="%s" d="M%.1f,%.1f C%.1f,%.1f %.1f,%.1f %.1f,%.1f" fill="none" stroke="%s" stroke-width="%.1f"%s marker-end="url(#%s)"/>`,
			cls, x1, y1, c1x, c1y, c2x, c2y, x2, y2, stroke, width, dash, marker)
		// Label position on the curve; forward edges put it nearer the
		// target so labels of a fan-out spread out vertically.
		tt := 0.5
		if rt > rs {
			tt = 0.64
		}
		mx := bezier(x1, c1x, c2x, x2, tt)
		my := bezier(y1, c1y, c2y, y2, tt)
		labels = append(labels, lbl{mx, my, e.Label, e.Technology, hl.edges[i], color})
	}
	// Nodes.
	for _, n := range m.Nodes {
		p, ok := lo.pos[n.ID]
		if !ok {
			continue
		}
		fill := "#1168bd"
		switch {
		case n.External:
			fill = "#6e7781"
		case n.IsPerson():
			fill = "#08427b"
		case n.Kind == model.KindContainer || n.Kind == model.KindDatabase || n.Kind == model.KindQueue:
			fill = "#2f7fd0"
		case n.Kind == model.KindComponent:
			fill = "#6aa9e9"
		}
		stroke, sw := "none", 0.0
		ids := hl.nodes[n.ID]
		if len(ids) > 0 {
			if hl.serious(ids) {
				stroke, sw = "#cf222e", 4
			} else {
				stroke, sw = "#bf8700", 3
			}
		}
		cls := "node"
		if len(ids) > 0 {
			cls = "node flagged"
		}
		fmt.Fprintf(&b, `<g class="%s" data-id="%s">`, cls, esc(n.ID, 80))
		switch {
		case n.IsDatabase():
			ry := 11.0
			fmt.Fprintf(&b, `<path d="M%.1f,%.1f v%.1f a%.1f,%.1f 0 0 0 %.1f,0 v%.1f a%.1f,%.1f 0 0 0 %.1f,0 z" fill="%s" stroke="%s" stroke-width="%.1f" filter="url(#sh)"/>`,
				p.x, p.y+ry, p.h-2*ry, p.w/2, ry, p.w, -(p.h - 2*ry), p.w/2, ry, -p.w, fill, stroke, sw)
			fmt.Fprintf(&b, `<path d="M%.1f,%.1f a%.1f,%.1f 0 0 0 %.1f,0" fill="none" stroke="#ffffff" stroke-opacity="0.55" stroke-width="1.3"/>`,
				p.x, p.y+ry, p.w/2, ry, p.w)
		case n.IsPerson():
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="26" fill="%s" stroke="%s" stroke-width="%.1f" filter="url(#sh)"/>`, p.x, p.y+10, p.w, p.h-10, fill, stroke, sw)
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="15" fill="%s" stroke="#ffffff" stroke-width="2"/>`, p.x+p.w/2, p.y+6, fill)
		default:
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="9" fill="%s" stroke="%s" stroke-width="%.1f" filter="url(#sh)"/>`, p.x, p.y, p.w, p.h, fill, stroke, sw)
		}
		ty := p.y + 34
		if n.IsPerson() {
			ty = p.y + 44
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="14.5" font-weight="700" fill="#ffffff">%s</text>`, p.x+p.w/2, ty, esc(n.Name, 26))
		kind := string(n.Kind)
		if n.External {
			kind += ", external"
		}
		if n.Technology != "" {
			kind += ": " + n.Technology
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="11.5" fill="#ffffff" fill-opacity="0.85">[%s]</text>`, p.x+p.w/2, ty+18, esc(kind, 32))
		if tags := visibleTags(n); tags != "" {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="10.5" fill="#ffffff" fill-opacity="0.75">%s</text>`, p.x+p.w/2, ty+34, esc(tags, 34))
		}
		for bi, id := range ids {
			bx := p.x + p.w - 8 - float64(len(ids)-bi)*54 + 8
			col := "#cf222e"
			if hl.info[id] {
				col = "#9a6700"
			}
			fmt.Fprintf(&b, `<g class="badge"><rect x="%.1f" y="%.1f" width="50" height="20" rx="10" fill="%s" stroke="#ffffff" stroke-width="1.5"/><text x="%.1f" y="%.1f" text-anchor="middle" font-size="11" font-weight="700" fill="#ffffff">%s</text></g>`,
				bx, p.y-10, col, bx+25, p.y+4, esc(id, 8))
		}
		b.WriteString(`</g>`)
	}
	// Edge labels.
	for _, l := range labels {
		text := OneLine(l.text, 30)
		tech := OneLine(l.tech, 30)
		wc := math.Max(float64(len([]rune(text))), float64(len([]rune(tech))+2))*6.6 + 16
		h := 34.0
		if tech == "" {
			h = 20
		}
		strokeL := "#d0d7de"
		if l.color != "" {
			strokeL = l.color
		}
		fmt.Fprintf(&b, `<g class="elabel"><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="6" fill="#ffffff" fill-opacity="0.94" stroke="%s"/>`, l.x-wc/2, l.y-h/2, wc, h, strokeL)
		tc := "#24292f"
		if l.color != "" {
			tc = l.color
		}
		ty := l.y - 2
		if tech == "" {
			ty = l.y + 4
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="11.5" font-weight="600" fill="%s">%s</text>`, l.x, ty, tc, html.EscapeString(text))
		if tech != "" {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" text-anchor="middle" font-size="10.5" fill="#57606a">[%s]</text>`, l.x, l.y+12, html.EscapeString(tech))
		}
		for bi, id := range l.badges {
			col := "#cf222e"
			if hl.info[id] {
				col = "#9a6700"
			}
			bx := l.x - float64(len(l.badges))*27 + float64(bi)*54
			fmt.Fprintf(&b, `<g class="badge"><rect x="%.1f" y="%.1f" width="50" height="18" rx="9" fill="%s"/><text x="%.1f" y="%.1f" text-anchor="middle" font-size="10.5" font-weight="700" fill="#ffffff">%s</text></g>`,
				bx+2, l.y-h/2-20, col, bx+27, l.y-h/2-7, esc(id, 8))
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`</svg>`)
	return b.String()
}

func bezier(p0, p1, p2, p3, t float64) float64 {
	u := 1 - t
	return u*u*u*p0 + 3*u*u*t*p1 + 3*u*t*t*p2 + t*t*t*p3
}

func firstNonEmpty(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

// visibleTags lists the canonical tags and a few attributes worth showing.
func visibleTags(n *model.Node) string {
	var parts []string
	for _, t := range n.Tags {
		parts = append(parts, "#"+t)
	}
	for _, k := range []string{"at_rest", "region", "exit"} {
		if v := n.Attr(k); v != "" {
			parts = append(parts, k+":"+v)
		}
	}
	return strings.Join(parts, " ")
}
