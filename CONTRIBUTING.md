# Contributing

Thanks for your interest. This is a small proof of concept, so the bar for
changes is: deterministic, tested, and honest in its wording.

## Ground rules

- **Findings come only from rules.** A rule is Go code in `internal/rules/rules.go`
  with an id, severity, title, rationale, remediation and clause ids. No scoring,
  no heuristics that cannot be explained in one sentence, no network calls, no AI.
- **Citations are real or absent.** A rule may cite only ids that exist in
  `internal/clauses/clauses.json`. If no clause fits, cite none: the outputs then
  say "no clause found". A test fails on unknown ids and on unused ones.
- **Wording.** The tool *assesses*, *evidences* and *proposes*. It never
  *certifies*, *confirms* or *guarantees*, and it never calls a design *compliant*.
- **Standard library only.** Please do not add Go dependencies.

## Workflow

```sh
go vet ./...
gofmt -l .           # must print nothing
go test ./...
go test ./cmd/c4g -update   # only when an output change is intended; review the diff
```

Every new rule needs at least one positive and one negative case in
`internal/rules/rules_test.go` (a test enforces this).

## Developer Certificate of Origin

Contributions are accepted under the [Developer Certificate of Origin 1.1](https://developercertificate.org/).
Sign off every commit to certify that you wrote the change or have the right to
submit it under the project licence (Apache-2.0):

```sh
git commit -s -m "feat(rules): add CG012 ..."
```

This adds a `Signed-off-by: Your Name <your address>` trailer. Pull requests
without sign-off cannot be merged.
