### c4-guardrails: **3 finding(s) at or above `high`**

| critical | high | medium | low | info |
|---:|---:|---:|---:|---:|
| 0 | 3 | 5 | 0 | 0 |

#### examples/rag-chatbot-bad.mmd (RAG support chatbot \(bad twin\))

| Severity | Rule | Where | Finding | Clauses |
|---|---|---|---|---|
| HIGH | CG001 Plaintext transport across a trust boundary | Customer → Chat widget (line 15) | &quot;Chats with&quot; crosses a trust boundary\: HTTP declared without TLS. | [GDPR\:Art32\(1\)\(a\)](https://eur-lex.europa.eu/eli/reg/2016/679/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(h\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| HIGH | CG003 Personal-data store without declared encryption at rest | Vector store (line 9) | Holds personal data; encryption at rest is not declared \(missing fact\). | [GDPR\:Art32\(1\)\(a\)](https://eur-lex.europa.eu/eli/reg/2016/679/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(h\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| HIGH | CG004 AI endpoint receives personal data without a human-oversight component | Chat API → Hosted LLM API (line 19) | &quot;Prompt with retrieved context&quot; sends personal data to an AI endpoint; no element tagged oversight. | [AIACT\:2024/1689\:Art14](https://eur-lex.europa.eu/eli/reg/2024/1689/oj)<br>[GDPR\:Art25](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| MEDIUM | CG005 Critical third-party ICT service without a declared exit or alternative | Hosted LLM API (line 13) | Tagged critical; no exit strategy or alternative declared. | [DORA\:2022/2554\:Art28\(8\)](https://eur-lex.europa.eu/eli/reg/2022/2554/oj)<br>[DORA\:2022/2554\:Art28](https://eur-lex.europa.eu/eli/reg/2022/2554/oj)<br>[NIS2\:2022/2555\:Art21\(2\)\(d\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj) |
| MEDIUM | CG007 No logging or monitoring component in a system handling personal data | whole diagram (line 4) | 3 element\(s\) handle personal data; no element tagged logging. | [NIS2\:2024/2690\:Annex-3.2](https://eur-lex.europa.eu/eli/reg_impl/2024/2690/oj)<br>[DORA-RTS\:2024/1774\:Art12](https://eur-lex.europa.eu/eli/reg_del/2024/1774/oj) |
| MEDIUM | CG008 Credential-based connections without a secrets store | Chat API → Vector store (line 17) | &quot;Similarity search&quot; authenticates with password; no element tagged secrets. | [NIS2\:2022/2555\:Art21\(2\)\(i\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj)<br>[GDPR\:Art32](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| MEDIUM | CG008 Credential-based connections without a secrets store | Chat API → Hosted LLM API (line 19) | &quot;Prompt with retrieved context&quot; authenticates with api\_key; no element tagged secrets. | [NIS2\:2022/2555\:Art21\(2\)\(i\)](https://eur-lex.europa.eu/eli/dir/2022/2555/oj)<br>[GDPR\:Art32](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |
| MEDIUM | CG010 Personal data sent to an external system with no declared processing region | Chat API → Hosted LLM API (line 19) | &quot;Prompt with retrieved context&quot; sends personal data to Hosted LLM API; region not declared \(missing fact\). | [GDPR\:Art44](https://eur-lex.europa.eu/eli/reg/2016/679/oj)<br>[GDPR\:Art46](https://eur-lex.europa.eu/eli/reg/2016/679/oj) |

<details><summary>Proposed fixes</summary>

- **CG001 Plaintext transport across a trust boundary**: Use TLS \(HTTPS, JDBC/TLS, AMQPS, mTLS ...\) and say so in the technology label, or add $tags=&quot;enc\:tls&quot;.
- **CG003 Personal-data store without declared encryption at rest**: Declare it, e.g. $tags=&quot;pii;at\_rest\:aes256&quot; \(or at\_rest\:cmk for customer-managed keys\).
- **CG004 AI endpoint receives personal data without a human-oversight component**: Add the oversight step to the design and tag it, e.g. Container\(review, &quot;Human review&quot;, ..., $tags=&quot;oversight&quot;\).
- **CG005 Critical third-party ICT service without a declared exit or alternative**: Declare the exit path, e.g. $tags=&quot;critical;exit\:secondary-provider&quot;, and draw the alternative if it exists.
- **CG007 No logging or monitoring component in a system handling personal data**: Add the log sink to the diagram and tag it, e.g. Container\(logs, &quot;Central logging&quot;, &quot;OpenTelemetry&quot;, $tags=&quot;logging&quot;\).
- **CG008 Credential-based connections without a secrets store**: Add a secrets store \(Key Vault, Vault, KMS\) tagged secrets, or switch to workload identity \(auth\:managed\_identity, auth\:mtls\).
- **CG010 Personal data sent to an external system with no declared processing region**: Declare the location on the external system, e.g. $tags=&quot;region\:eu-west&quot;, and record the transfer safeguard if it is outside the EU/EEA.

</details>

<sub>c4-guardrails assesses what a design-time diagram declares against deterministic rules. It evidences gaps and proposes fixes; it is not a compliance certification and not legal advice. Clause references: © European Union, https\://eur-lex.europa.eu \(Commission Decision 2011/833/EU\).</sub>
