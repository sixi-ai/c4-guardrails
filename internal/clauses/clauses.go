// Package clauses embeds the trimmed EU clause subset the rules cite.
package clauses

import (
	_ "embed"
	"encoding/json"
	"sort"
)

//go:embed clauses.json
var raw []byte

// Clause is one citable provision, identified as REGIME:DOC:REF.
type Clause struct {
	ID         string `json:"id"`
	Instrument string `json:"instrument"`
	Ref        string `json:"ref"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	SourceURL  string `json:"source_url"`
}

type file struct {
	About       string   `json:"about"`
	Attribution string   `json:"attribution"`
	Clauses     []Clause `json:"clauses"`
}

var (
	data  file
	index map[string]Clause
)

func init() {
	if err := json.Unmarshal(raw, &data); err != nil {
		panic("clauses.json: " + err.Error())
	}
	index = make(map[string]Clause, len(data.Clauses))
	for _, c := range data.Clauses {
		index[c.ID] = c
	}
}

// NoClause is shown wherever a finding has no clause in the subset.
const NoClause = "no clause found"

// Lookup returns the clause with the given id.
func Lookup(id string) (Clause, bool) { c, ok := index[id]; return c, ok }

// All returns every clause, sorted by id.
func All() []Clause {
	out := append([]Clause(nil), data.Clauses...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Attribution is the EU re-use notice for the clause titles and links.
func Attribution() string { return data.Attribution }
