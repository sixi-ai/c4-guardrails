package clauses_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/rbrus/c4-guardrails/internal/clauses"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

// Every clause a rule cites must exist in the shipped subset. Unknown ids
// are a hard failure: a citation is never invented.
func TestRulesCiteOnlyKnownClauses(t *testing.T) {
	for _, r := range rules.All() {
		for _, id := range r.Clauses {
			if _, ok := clauses.Lookup(id); !ok {
				t.Errorf("rule %s cites %q, which is not in clauses.json", r.ID, id)
			}
		}
	}
}

// The subset stays trimmed: every clause in it is cited by some rule.
func TestSubsetHasNoUnusedClauses(t *testing.T) {
	used := map[string]bool{}
	for _, r := range rules.All() {
		for _, id := range r.Clauses {
			used[id] = true
		}
	}
	for _, c := range clauses.All() {
		if !used[c.ID] {
			t.Errorf("clause %s is not cited by any rule; remove it from the subset", c.ID)
		}
	}
}

func TestClauseRecords(t *testing.T) {
	idRe := regexp.MustCompile(`^[A-Z0-9-]+:([0-9]{4}/[0-9]+:)?[A-Za-z0-9().-]+$`)
	if !strings.Contains(clauses.Attribution(), "eur-lex.europa.eu") || !strings.Contains(clauses.Attribution(), "2011/833/EU") {
		t.Errorf("attribution = %q", clauses.Attribution())
	}
	for _, c := range clauses.All() {
		if !idRe.MatchString(c.ID) {
			t.Errorf("id %q is not REGIME:DOC:REF", c.ID)
		}
		if c.Title == "" || c.Summary == "" || c.Instrument == "" || c.Ref == "" {
			t.Errorf("%s: incomplete record", c.ID)
		}
		if !strings.HasPrefix(c.SourceURL, "https://eur-lex.europa.eu/eli/") {
			t.Errorf("%s: source_url %q is not an EUR-Lex ELI link", c.ID, c.SourceURL)
		}
	}
	if _, ok := clauses.Lookup("GDPR:Art999"); ok {
		t.Error("lookup of an unknown id must fail")
	}
}
