# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub:
**Security → Report a vulnerability** on
<https://github.com/rbrus/c4-guardrails/security/advisories/new>
(GitHub private vulnerability reporting).

Please do not open a public issue for a vulnerability. Include the version
(`c4g -version`), an input file that reproduces the problem if you can, and
what you expected to happen.

You can expect an acknowledgement within 5 working days. Fixes are released as
a new tag and credited in the release notes unless you prefer otherwise.

## Scope

In scope: the `c4g` CLI, its parsers, its report writers (for example an escaping
problem in the Markdown, SARIF or HTML output) and the composite GitHub Action in
`action.yml`.

Out of scope: the correctness of the regulatory interpretation. The rules are
design-time heuristics; report disagreements as a normal issue.

## Supported versions

Only the latest tagged release receives fixes.
