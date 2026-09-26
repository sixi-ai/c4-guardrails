package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/rbrus/c4-guardrails/internal/clauses"
	"github.com/rbrus/c4-guardrails/internal/rules"
)

// SARIF 2.1.0 subset.
type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"`
	Rules          []sarifRule `json:"rules"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifHelp struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

type sarifRule struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	ShortDescription     sarifText      `json:"shortDescription"`
	FullDescription      sarifText      `json:"fullDescription"`
	Help                 sarifHelp      `json:"help"`
	HelpURI              string         `json:"helpUri"`
	DefaultConfiguration map[string]any `json:"defaultConfiguration"`
	Properties           map[string]any `json:"properties"`
}

type sarifResult struct {
	RuleID              string            `json:"ruleId"`
	RuleIndex           int               `json:"ruleIndex"`
	Level               string            `json:"level"`
	Message             sarifText         `json:"message"`
	Locations           []sarifLocation   `json:"locations"`
	PartialFingerprints map[string]string `json:"partialFingerprints"`
	Properties          map[string]any    `json:"properties"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysical `json:"physicalLocation"`
}

type sarifPhysical struct {
	ArtifactLocation sarifArtifact `json:"artifactLocation"`
	Region           sarifRegion   `json:"region"`
}

type sarifArtifact struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine int `json:"startLine"`
}

// SARIFLevel maps a severity to a SARIF level.
func SARIFLevel(s rules.Severity) string {
	switch {
	case s >= rules.High:
		return "error"
	case s == rules.Medium:
		return "warning"
	default:
		return "note"
	}
}

func securitySeverity(s rules.Severity) string {
	return [...]string{"0.0", "3.0", "5.5", "8.0", "9.5"}[s]
}

// WriteSARIF writes a SARIF 2.1.0 log with one run.
func WriteSARIF(w io.Writer, r *Result) error {
	all := rules.All()
	idx := map[string]int{}
	var srules []sarifRule
	for i, ru := range all {
		idx[ru.ID] = i
		tags := []string{"security", "design-time", "c4"}
		tags = append(tags, ru.Clauses...)
		srules = append(srules, sarifRule{
			ID: ru.ID, Name: ru.Name,
			ShortDescription:     sarifText{ru.Title},
			FullDescription:      sarifText{ru.Rationale},
			Help:                 sarifHelp{Text: ruleHelpText(ru), Markdown: ruleHelpMarkdown(ru)},
			HelpURI:              ToolURI + "#rules",
			DefaultConfiguration: map[string]any{"level": SARIFLevel(ru.Severity)},
			Properties: map[string]any{
				"tags":              tags,
				"clauses":           append([]string{}, ru.Clauses...),
				"security-severity": securitySeverity(ru.Severity),
				"precision":         "high",
			},
		})
	}
	results := []sarifResult{}
	for _, f := range r.Findings {
		line := f.Line
		if line < 1 {
			line = 1
		}
		sum := sha256.Sum256([]byte(f.RuleID + "\x00" + f.Source + "\x00" + f.Subject + "\x00" + f.Message))
		results = append(results, sarifResult{
			RuleID: f.RuleID, RuleIndex: idx[f.RuleID], Level: SARIFLevel(f.Severity),
			Message: sarifText{OneLine(fmt.Sprintf("%s: %s (%s) Clauses: %s", f.Title, f.Message, f.Subject, ClauseLabel(f.Clauses)), 1000)},
			Locations: []sarifLocation{{PhysicalLocation: sarifPhysical{
				ArtifactLocation: sarifArtifact{URI: sarifURI(f.Source)},
				Region:           sarifRegion{StartLine: line},
			}}},
			PartialFingerprints: map[string]string{"c4guardrails/v1": hex.EncodeToString(sum[:16])},
			Properties:          map[string]any{"severity": f.Severity.String(), "clauses": f.Clauses},
		})
	}
	log := sarifLog{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool:    sarifTool{Driver: sarifDriver{Name: ToolName, Version: r.Version, InformationURI: ToolURI, Rules: srules}},
			Results: results,
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func sarifURI(p string) string {
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	return p
}

func ruleHelpText(ru rules.Rule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nWhy: %s\n\nFix: %s\n\nClauses: ", ru.Title, ru.Rationale, ru.Remediation)
	if len(ru.Clauses) == 0 {
		b.WriteString(clauses.NoClause)
	}
	for i, id := range ru.Clauses {
		if i > 0 {
			b.WriteString("; ")
		}
		c, _ := clauses.Lookup(id)
		fmt.Fprintf(&b, "%s (%s) %s", id, c.Title, c.SourceURL)
	}
	b.WriteString("\n\n" + Disclaimer)
	return b.String()
}

func ruleHelpMarkdown(ru rules.Rule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s**\n\n%s\n\n**Proposed fix:** %s\n\n**Clauses**\n\n", MD(ru.Title), MD(ru.Rationale), MD(ru.Remediation))
	if len(ru.Clauses) == 0 {
		b.WriteString("- " + clauses.NoClause + "\n")
	}
	for _, id := range ru.Clauses {
		c, _ := clauses.Lookup(id)
		fmt.Fprintf(&b, "- [%s](%s): %s. %s\n", MD(id), c.SourceURL, MD(c.Title), MD(c.Summary))
	}
	b.WriteString("\n_" + MD(Disclaimer) + "_\n")
	return b.String()
}
