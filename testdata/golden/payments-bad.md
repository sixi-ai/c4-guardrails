### c4-guardrails: **4 finding(s) at or above `high`**

| critical | high | medium | low | info |
|---:|---:|---:|---:|---:|
| 0 | 4 | 4 | 0 | 1 |

#### examples/payments-bad.mmd (Payments platform \(bad twin\))

| Severity | Rule | Where | Finding | Clauses |
|---|---|---|---|---|
| HIGH | CG001 Plaintext transport across a trust boundary | Back-office clerk → Payments ledger (line 19) | &quot;Edits rejected orders&quot; crosses a trust boundary\: SQL declared without TLS. | [GDPR\:Art32\(1\)\(a\)](https://eur-lex.europa.eu/eli/reg/2016/679/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(h\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| HIGH | CG002 External system called without declared authentication | Payment engine → Clearing ICT provider (line 21) | &quot;Sends payment orders&quot;\: no authentication declared \(missing fact\). | [NIS2\:2022/2555\:Art21\(2\)\(j\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj)<br>[GDPR\:Art32](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| HIGH | CG003 Personal-data store without declared encryption at rest | Payments ledger (line 11) | Holds personal data; encryption at rest is not declared \(missing fact\). | [GDPR\:Art32\(1\)\(a\)](https://eur-lex.europa.eu/eli/reg/2016/679/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(h\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| HIGH | CG006 Database reachable directly from a person or the internet | Back-office clerk → Payments ledger (line 19) | &quot;Edits rejected orders&quot; connects Back-office clerk directly to a database. | [NIS2\:2024/2690\:Annex-6.8](https://eur-lex.europa.eu/eli/reg_impl/2024/2690/oj)<br>[GDPR\:Art25](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| MEDIUM | CG005 Critical third-party ICT service without a declared exit or alternative | Clearing ICT provider (line 15) | Tagged critical; no exit strategy or alternative declared. | [DORA\:2022/2554\:Art28\(8\)](https://eur-lex.europa.eu/eli/reg/2022/2554/oj)<br>[DORA\:2022/2554\:Art28](https://eur-lex.europa.eu/eli/reg/2022/2554/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(d\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| MEDIUM | CG007 No logging or monitoring component in a system handling personal data | whole diagram (line 4) | 2 element\(s\) handle personal data; no element tagged logging. | [NIS2\:2024/2690\:Annex-3.2](https://eur-lex.europa.eu/eli/reg_impl/2024/2690/oj)<br>[DORA-RTS\:2024/1774\:Art12](https://eur-lex.europa.eu/eli/reg_del/2024/1774/oj) |
| MEDIUM | CG008 Credential-based connections without a secrets store | Payment engine → Payments ledger (line 20) | &quot;Posts entries&quot; authenticates with password; no element tagged secrets. | [NIS2\:2022/2555\:Art21\(2\)\(i\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj)<br>[GDPR\:Art32](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| MEDIUM | CG010 Personal data sent to an external system with no declared processing region | Payment engine → Clearing ICT provider (line 21) | &quot;Sends payment orders&quot; sends personal data to Clearing ICT provider; region not declared \(missing fact\). | [GDPR\:Art44](https://eur-lex.europa.eu/eli/reg/2016/679/oj)<br>[GDPR\:Art46](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| INFO | CG011 Missing fact | Payment engine → Clearing ICT provider (line 21) | &quot;Sends payment orders&quot; crosses a trust boundary; transport encryption not stated. | _no clause found_ |

<details><summary>Proposed fixes</summary>

- **CG001 Plaintext transport across a trust boundary**: Use TLS \(HTTPS, JDBC/TLS, AMQPS, mTLS ...\) and say so in the technology label, or add $tags=&quot;enc\:tls&quot;.
- **CG002 External system called without declared authentication**: Declare the mechanism, e.g. $tags=&quot;auth\:oauth2&quot;, &quot;auth\:mtls&quot; or &quot;auth\:managed\_identity&quot;.
- **CG003 Personal-data store without declared encryption at rest**: Declare it, e.g. $tags=&quot;pii;at\_rest\:aes256&quot; \(or at\_rest\:cmk for customer-managed keys\).
- **CG006 Database reachable directly from a person or the internet**: Route access through an application or API tier that authenticates and authorises; keep the database in an internal zone.
- **CG005 Critical third-party ICT service without a declared exit or alternative**: Declare the exit path, e.g. $tags=&quot;critical;exit\:secondary-provider&quot;, and draw the alternative if it exists.
- **CG007 No logging or monitoring component in a system handling personal data**: Add the log sink to the diagram and tag it, e.g. Container\(logs, &quot;Central logging&quot;, &quot;OpenTelemetry&quot;, $tags=&quot;logging&quot;\).
- **CG008 Credential-based connections without a secrets store**: Add a secrets store \(Key Vault, Vault, KMS\) tagged secrets, or switch to workload identity \(auth\:managed\_identity, auth\:mtls\).
- **CG010 Personal data sent to an external system with no declared processing region**: Declare the location on the external system, e.g. $tags=&quot;region\:eu-west&quot;, and record the transfer safeguard if it is outside the EU/EEA.
- **CG011 Missing fact**: State the fact in the technology label or with $tags \(see the attribute vocabulary in the README\).

</details>

<sub>c4-guardrails assesses what a design-time diagram declares against deterministic rules. It evidences gaps and proposes fixes; it is not a compliance certification and not legal advice. Clause references: © European Union, https\://eur-lex.europa.eu \(Commission Decision 2011/833/EU\).</sub>
