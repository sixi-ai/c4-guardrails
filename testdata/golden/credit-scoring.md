### c4-guardrails: **2 finding(s) at or above `high`**

| critical | high | medium | low | info |
|---:|---:|---:|---:|---:|
| 0 | 2 | 2 | 0 | 0 |

#### examples/credit-scoring.c4.json (Credit scoring service \(JSON input example\))

| Severity | Rule | Where | Finding | Clauses |
|---|---|---|---|---|
| HIGH | CG002 External system called without declared authentication | Credit scoring model → Credit bureau (line 19) | &quot;Fetches credit history&quot;\: authentication declared as &quot;none&quot;. | [NIS2\:2022/2555\:Art21\(2\)\(j\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj)<br>[GDPR\:Art32](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| HIGH | CG009 High-risk AI system without a record-keeping log store | Credit scoring model (line 9) | Tagged ai \+ high-risk; no relationship to an element tagged records. | [AIACT\:2024/1689\:Art12](https://eur-lex.europa.eu/eli/reg/2024/1689/oj)<br>[AIACT\:2024/1689\:Art26\(6\)](https://eur-lex.europa.eu/eli/reg/2024/1689/oj) |
| MEDIUM | CG005 Critical third-party ICT service without a declared exit or alternative | Credit bureau (line 13) | Tagged critical; no exit strategy or alternative declared. | [DORA\:2022/2554\:Art28\(8\)](https://eur-lex.europa.eu/eli/reg/2022/2554/oj)<br>[DORA\:2022/2554\:Art28](https://eur-lex.europa.eu/eli/reg/2022/2554/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(d\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| MEDIUM | CG007 No logging or monitoring component in a system handling personal data | whole diagram (line 7) | 2 element\(s\) handle personal data; no element tagged logging. | [NIS2\:2024/2690\:Annex-3.2](https://eur-lex.europa.eu/eli/reg_impl/2024/2690/oj)<br>[DORA-RTS\:2024/1774\:Art12](https://eur-lex.europa.eu/eli/reg_del/2024/1774/oj) |

<details><summary>Proposed fixes</summary>

- **CG002 External system called without declared authentication**: Declare the mechanism, e.g. $tags=&quot;auth\:oauth2&quot;, &quot;auth\:mtls&quot; or &quot;auth\:managed\_identity&quot;.
- **CG009 High-risk AI system without a record-keeping log store**: Add an append-only log store tagged records and draw the relationship from the AI system to it.
- **CG005 Critical third-party ICT service without a declared exit or alternative**: Declare the exit path, e.g. $tags=&quot;critical;exit\:secondary-provider&quot;, and draw the alternative if it exists.
- **CG007 No logging or monitoring component in a system handling personal data**: Add the log sink to the diagram and tag it, e.g. Container\(logs, &quot;Central logging&quot;, &quot;OpenTelemetry&quot;, $tags=&quot;logging&quot;\).

</details>

<sub>c4-guardrails assesses what a design-time diagram declares against deterministic rules. It evidences gaps and proposes fixes; it is not a compliance certification and not legal advice. Clause references: © European Union, https\://eur-lex.europa.eu \(Commission Decision 2011/833/EU\).</sub>
