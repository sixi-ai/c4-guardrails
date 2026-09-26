#!/usr/bin/env node
// Regenerates docs/media/* from the examples. Not part of the Go build.
//
//   npm i --prefix /tmp/media-tools playwright-core
//   NODE_PATH=/tmp/media-tools/node_modules CHROME=/path/to/chrome node tools/media/generate.cjs
//
// Requires Go (to build c4g) and a Chromium binary. Everything it renders is
// produced by c4g from the files in examples/; nothing is hand-edited.
'use strict';

const { execFileSync, spawnSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('playwright-core');

const ROOT = path.resolve(__dirname, '..', '..');
const OUT = path.join(ROOT, 'tools', 'media', 'out');
const MEDIA = path.join(ROOT, 'docs', 'media');
const CHROME = process.env.CHROME || undefined;
const VERSION = '0.1.0';

fs.mkdirSync(OUT, { recursive: true });
fs.mkdirSync(MEDIA, { recursive: true });

// ---------------------------------------------------------------- c4g runs
const bin = path.join(OUT, 'c4g');
execFileSync('go', ['build', '-trimpath', '-ldflags', `-X main.version=${VERSION}`, '-o', bin, './cmd/c4g'], { cwd: ROOT, stdio: 'inherit' });

function c4g(args) {
  const r = spawnSync(bin, args, { cwd: ROOT, encoding: 'utf8' });
  if (r.status === 2) throw new Error(r.stderr);
  return { out: r.stdout, code: r.status };
}

const ex = (n) => `examples/${n}.mmd`;
const o = (f) => path.join(OUT, f);
const ragBad = c4g([ex('rag-chatbot-bad'), '-html', o('rag-bad.html'), '-md', o('rag-bad.md'), '-sarif', o('rag-bad.sarif')]);
const ragGood = c4g([ex('rag-chatbot-good'), '-html', o('rag-good.html')]);
const payBad = c4g([ex('payments-bad'), '-sarif', o('pay-bad.sarif')]);
const payGood = c4g([ex('payments-good')]);
const ragBadJSON = JSON.parse(c4g(['-json', ex('rag-chatbot-bad')]).out);
const payBadJSON = JSON.parse(c4g(['-json', ex('payments-bad')]).out);

const read = (f) => fs.readFileSync(o(f), 'utf8');
const svgOf = (html) => html.match(/<svg[\s\S]*?<\/svg>/)[0];
const badSVG = svgOf(read('rag-bad.html'));
const goodSVG = svgOf(read('rag-good.html'));

const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

// ---------------------------------------------------------------- shared css
const FONT = `font-family: Inter, "Segoe UI", system-ui, -apple-system, Helvetica, Arial, sans-serif;`;
const MONO = `font-family: "JetBrains Mono", "DejaVu Sans Mono", ui-monospace, Menlo, Consolas, monospace;`;

// Colourise the plain-text table for the terminal mock.
function colourTerminal(text) {
  return esc(text).split('\n').map((line) => line
    .replace(/^(\s+)(HIGH|CRITICAL)(\s)/, '$1<b style="color:#ff7b72">$2</b>$3')
    .replace(/^(\s+)(MEDIUM)(\s)/, '$1<b style="color:#ffa657">$2</b>$3')
    .replace(/^(\s+)(LOW)(\s)/, '$1<b style="color:#e3b341">$2</b>$3')
    .replace(/^(\s+)(INFO)(\s)/, '$1<b style="color:#79c0ff">$2</b>$3')
    .replace(/\b(CG\d{3})\b/g, '<span style="color:#d2a8ff">$1</span>')
    .replace(/((?:GDPR|NIS2|DORA|DORA-RTS|AIACT):[^\s,]+)/g, '<span style="color:#56d4dd">$1</span>')
    .replace(/(fail: .*)$/, '<b style="color:#ff7b72">$1</b>')
    .replace(/(-&gt; pass)$/, '<b style="color:#3fb950">$1</b>')
  ).join('\n');
}

function terminalPage(cmd, text, code) {
  return `<!doctype html><meta charset="utf-8"><style>
  body{margin:0;background:#1b1f24;display:flex;align-items:center;justify-content:center;height:100vh;${FONT}}
  .win{width:1540px;background:#0d1117;border-radius:14px;box-shadow:0 30px 80px rgba(0,0,0,.55);overflow:hidden;border:1px solid #30363d}
  .bar{height:40px;background:#161b22;display:flex;align-items:center;gap:8px;padding:0 16px;border-bottom:1px solid #30363d}
  .d{width:13px;height:13px;border-radius:50%}.t{color:#8b949e;font-size:13px;margin-left:12px}
  pre{margin:0;padding:22px 26px 26px;color:#c9d1d9;font-size:14.5px;line-height:1.5;${MONO};white-space:pre}
  .p{color:#3fb950}.c{color:#f0f6fc;font-weight:700}</style>
  <div class="win"><div class="bar"><span class="d" style="background:#ff5f57"></span><span class="d" style="background:#febc2e"></span><span class="d" style="background:#28c840"></span><span class="t">c4-guardrails — bash</span></div>
  <pre><span class="p">$</span> <span class="c">${esc(cmd)}</span>\n${colourTerminal(text)}<span class="p">$</span> echo $?\n${code}</pre></div>`;
}

// Minimal renderer for the Markdown subset c4g writes (headings, tables,
// links, bold, details, sub, backslash escapes, entities).
function renderMD(md) {
  const inline = (s) => {
    const parts = [];
    s = s.replace(/\\(.)/g, (_, c) => { parts.push(esc(c)); return `\u0000${parts.length - 1}\u0000`; });
    s = s.replace(/\[([^\]]+)\]\((https:\/\/[^)\s]+)\)/g, (_, t, u) => `<a href="${u}">${t}</a>`);
    s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
    s = s.replace(/`([^`]+)`/g, '<code>$1</code>');
    s = s.replace(/(^|[\s(])_([^_]+)_/g, '$1<em>$2</em>');
    return s.replace(/\u0000(\d+)\u0000/g, (_, i) => parts[+i]);
  };
  const lines = md.split('\n');
  let html = '';
  for (let i = 0; i < lines.length; i++) {
    const l = lines[i];
    if (l.startsWith('#### ')) html += `<h4>${inline(l.slice(5))}</h4>`;
    else if (l.startsWith('### ')) html += `<h3>${inline(l.slice(4))}</h3>`;
    else if (l.startsWith('|')) {
      const rows = [];
      while (i < lines.length && lines[i].startsWith('|')) rows.push(lines[i++]);
      i--;
      const cells = (r) => r.slice(1, -1).split('|').map((c) => c.trim());
      const head = cells(rows[0]);
      const align = cells(rows[1]).map((c) => (c.endsWith(':') ? 'right' : 'left'));
      html += '<table><thead><tr>' + head.map((h, k) => `<th style="text-align:${align[k]}">${inline(h)}</th>`).join('') + '</tr></thead><tbody>';
      for (const r of rows.slice(2)) html += '<tr>' + cells(r).map((c, k) => `<td style="text-align:${align[k]}">${inline(c)}</td>`).join('') + '</tr>';
      html += '</tbody></table>';
    } else if (l.startsWith('<details>')) html += '<details open><summary>Proposed fixes</summary><ul>';
    else if (l.startsWith('</details>')) html += '</ul></details>';
    else if (l.startsWith('- ')) html += `<li>${inline(l.slice(2))}</li>`;
    else if (l.startsWith('<sub>')) html += `<p class="sub">${inline(l.replace(/<\/?sub>/g, ''))}</p>`;
    else if (l.trim()) html += `<p>${inline(l)}</p>`;
  }
  return html;
}

function prPage(md) {
  return `<!doctype html><meta charset="utf-8"><style>
  body{margin:0;background:#ffffff;${FONT};color:#1f2328;font-size:14px}
  .wrap{width:1400px;margin:28px auto;display:flex;gap:16px}
  .av{width:40px;height:40px;border-radius:50%;background:#1f2328;color:#fff;display:flex;align-items:center;justify-content:center;font-weight:700;flex:none}
  .box{flex:1;border:1px solid #d1d9e0;border-radius:6px;overflow:hidden}
  .hd{background:#f6f8fa;border-bottom:1px solid #d1d9e0;padding:10px 16px;color:#59636e}
  .hd b{color:#1f2328}.tag{border:1px solid #d1d9e0;border-radius:12px;padding:0 7px;font-size:12px;margin-left:6px}
  .bd{padding:16px 20px}
  h3{font-size:20px;margin:4px 0 14px;padding-bottom:8px;border-bottom:1px solid #d1d9e0}h4{font-size:16px;margin:18px 0 8px}
  table{border-collapse:collapse;margin:8px 0 12px;font-size:13px;width:100%}th,td{border:1px solid #d1d9e0;padding:6px 10px;vertical-align:top}th{background:#f6f8fa}
  tr:nth-child(2n) td{background:#f6f8fa}a{color:#0969da;text-decoration:none}code{background:#eff1f3;padding:1px 5px;border-radius:5px;font-size:12px}
  details summary{cursor:pointer;font-weight:600;margin:6px 0}li{margin:3px 0}.sub{color:#59636e;font-size:11.5px}
  .title{width:1400px;margin:24px auto 0;font-size:26px}.title .num{color:#59636e;font-weight:300}
  .state{display:inline-block;background:#1f883d;color:#fff;border-radius:16px;padding:3px 12px;font-size:14px;margin:8px 0 0}
  </style>
  <div class="title">Add RAG support chatbot architecture <span class="num">#42</span><div><span class="state">Open</span></div></div>
  <div class="wrap"><div class="av">C4</div><div class="box"><div class="hd"><b>github-actions</b> <span class="tag">bot</span> commented — c4-guardrails job summary</div>
  <div class="bd">${renderMD(md)}</div></div></div>`;
}

function beforeAfterPage() {
  const sum = (j) => `${j.summary.high} high · ${j.summary.medium} medium`;
  return `<!doctype html><meta charset="utf-8"><style>
  body{margin:0;background:#f6f8fa;${FONT};color:#1f2328}
  h1{text-align:center;font-size:30px;margin:26px 0 4px}p.s{text-align:center;color:#59636e;margin:0 0 18px;font-size:16px}
  .row{display:flex;gap:20px;padding:0 24px 24px}
  .col{flex:1;background:#fff;border:1px solid #d1d9e0;border-radius:14px;overflow:hidden}
  .hd{padding:12px 18px;font-weight:700;font-size:18px;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #d1d9e0}
  .bad .hd{background:#ffebe9;color:#82071e}.good .hd{background:#dafbe1;color:#116329}
  .col svg{display:block;width:100%;height:auto;max-height:700px}
  .pill{font-size:13px;padding:3px 10px;border-radius:999px;color:#fff}.bad .pill{background:#cf222e}.good .pill{background:#1a7f37}
  </style>
  <h1>Same system, two designs</h1><p class="s">c4-guardrails on examples/rag-chatbot-bad.mmd and examples/rag-chatbot-good.mmd</p>
  <div class="row"><div class="col bad"><div class="hd">Before (bad twin)<span class="pill">${sum(ragBadJSON)}</span></div>${badSVG}</div>
  <div class="col good"><div class="hd">After (good twin)<span class="pill">no findings</span></div>${goodSVG}</div></div>`;
}

// ---------------------------------------------------------------- videos
function findingCards(j, limit) {
  return j.findings.slice(0, limit).map((f, i) => `<div class="fc sev-${f.severity}" style="--i:${i}">
    <div class="fh"><span class="pill">${esc(f.severity)}</span><b>${esc(f.rule_id)}</b> ${esc(f.title)}</div>
    <div class="fs">${esc(f.subject)}</div>
    <div class="chips">${(f.clauses.length ? f.clauses : ['no clause found']).map((c) => `<span class="chip">${esc(c)}</span>`).join('')}</div></div>`).join('');
}

const VIDEO_CSS = `
  html,body{margin:0;width:1080px;height:1920px;overflow:hidden;background:#0d1117;${FONT};color:#f0f6fc}
  .scene{position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center;opacity:0;transition:opacity .6s ease;padding:0 70px;box-sizing:border-box}
  .scene.on{opacity:1}
  .kicker{font-size:30px;letter-spacing:.14em;text-transform:uppercase;color:#8b949e;margin-bottom:28px}
  .big{font-size:92px;font-weight:800;line-height:1.08;text-align:center}
  .mid{font-size:44px;font-weight:700;text-align:center;margin:0 0 30px}
  .sub{font-size:34px;color:#8b949e;text-align:center;margin-top:30px}
  .card{background:#ffffff;border-radius:28px;padding:26px;width:100%;box-sizing:border-box}
  .card svg{width:100%;height:auto;display:block}
  .card.pan{height:1060px;overflow:hidden;position:relative;padding:0}
  .panner{position:absolute;left:0;top:30px;width:2050px;transition:transform 4.8s ease-in-out}
  .panner svg{width:2050px;height:auto}
  .flagged,.badge{transition:opacity .5s ease}
  .hide-flags .badge{opacity:0}
  .hide-flags .flagged path,.hide-flags path.flagged,.hide-flags .flagged rect{stroke:#57606a !important;stroke-width:1.6 !important}
  .fc{background:#161b22;border:1px solid #30363d;border-left:10px solid var(--c);border-radius:18px;padding:22px 26px;margin:0 0 20px;width:100%;box-sizing:border-box;opacity:0;transform:translateY(30px);transition:all .5s ease}
  .fc.on{opacity:1;transform:none}
  .fh{font-size:31px;line-height:1.3}.fh b{color:#d2a8ff;margin:0 8px 0 12px}
  .fs{font-size:26px;color:#8b949e;margin:8px 0 12px}
  .pill{display:inline-block;background:var(--c);color:#fff;border-radius:999px;padding:3px 16px;font-size:22px;font-weight:800;text-transform:uppercase;vertical-align:4px}
  .chip{display:inline-block;margin:0 10px 8px 0;padding:5px 14px;border-radius:10px;background:#0c2d6b;color:#a5d6ff;border:1px solid #1f6feb;font-size:23px;${MONO}}
  .sev-critical,.sev-high{--c:#f85149}.sev-medium{--c:#f0883e}.sev-low{--c:#d29922}.sev-info{--c:#58a6ff}
  .banner{font-size:46px;font-weight:800;padding:18px 36px;border-radius:20px;margin:30px 0 0}
  .red{background:#da3633}.green{background:#238636}
  .term{background:#010409;border:1px solid #30363d;border-radius:22px;width:100%;box-sizing:border-box;padding:28px 30px;${MONO};font-size:25px;line-height:1.45;white-space:pre;overflow:hidden;color:#c9d1d9;min-height:1000px}
  .term .p{color:#3fb950}
  .gh{background:#ffffff;color:#1f2328;border-radius:24px;width:100%;box-sizing:border-box;padding:30px;font-size:30px}
  .gh .row{display:flex;align-items:center;gap:18px;padding:18px 0;border-bottom:1px solid #d1d9e0}
  .gh .row:last-child{border-bottom:0}
  .x{width:44px;height:44px;border-radius:50%;background:#cf222e;color:#fff;display:flex;align-items:center;justify-content:center;font-weight:800;flex:none}
  .ok{background:#1a7f37}
  .gh small{display:block;color:#59636e;font-size:24px}
  .alert{border:1px solid #d1d9e0;border-radius:18px;padding:24px;margin-top:24px}
  .alert h3{margin:0 0 10px;font-size:32px}
  .alert .chip{background:#ddf4ff;color:#0550ae;border-color:#54aeff}
  .end{font-size:60px;font-weight:800;text-align:center;line-height:1.25}
  .url{font-size:40px;color:#58a6ff;margin-top:26px;${MONO}}
`;

function short1Page() {
  const cards = findingCards(ragBadJSON, 6);
  return `<!doctype html><meta charset="utf-8"><style>${VIDEO_CSS}</style>
  <div class="scene" id="s0"><div class="kicker">c4-guardrails</div><div class="big">Your architecture diagram, reviewed like code</div><div class="sub">Deterministic rules · EU clause citations · no AI</div></div>
  <div class="scene" id="s1"><div class="mid">RAG chatbot, bad twin</div><div class="card pan hide-flags" id="bad"><div class="panner" id="panner">${badSVG}</div></div><div class="banner red" id="b1" style="opacity:0">${ragBadJSON.summary.high} high · ${ragBadJSON.summary.medium} medium</div></div>
  <div class="scene" id="s2" style="justify-content:flex-start;padding-top:120px"><div class="mid">Every finding cites a clause</div>${cards}</div>
  <div class="scene" id="s3"><div class="mid">Good twin</div><div class="card">${goodSVG}</div><div class="banner green">0 findings · exit 0</div></div>
  <div class="scene" id="s4"><div class="end">c4-guardrails<br><span style="color:#8b949e;font-weight:600">open source</span></div><div class="url">github.com/rbrus/c4-guardrails</div><div class="sub">Assesses design-time facts. Not a compliance certification.</div></div>
  <script>
  const on = (id) => { document.querySelectorAll('.scene').forEach(s => s.classList.toggle('on', s.id === id)); };
  const at = (ms, fn) => setTimeout(fn, ms);
  at(200, () => on('s0'));
  at(3800, () => on('s1'));
  at(4300, () => document.getElementById('panner').style.transform = 'translateX(-1110px)');
  at(5200, () => document.getElementById('bad').classList.remove('hide-flags'));
  at(7400, () => document.getElementById('b1').style.opacity = 1);
  at(9800, () => { on('s2'); document.querySelectorAll('#s2 .fc').forEach((c, i) => setTimeout(() => c.classList.add('on'), 400 + i * 650)); });
  at(16800, () => on('s3'));
  at(21300, () => on('s4'));
  </script>`;
}

function short2Page() {
  const cut = (t, n) => (t.length > n ? t.slice(0, n - 1) + '…' : t);
  const lines = [`c4-guardrails ${VERSION}`, `${payBadJSON.inputs[0].source}: ${payBadJSON.findings.length} findings`, ''];
  for (const f of payBadJSON.findings) {
    lines.push(`${f.severity.toUpperCase().padEnd(7)}${f.rule_id}  ${cut(f.subject, 40)}`);
    lines.push(`       ${cut(f.clauses.length ? f.clauses.join(', ') : 'no clause found', 50)}`);
  }
  const sm = payBadJSON.summary;
  lines.push('', `${sm.high} high · ${sm.medium} medium · ${sm.info} info`, 'SARIF written: c4g.sarif');
  const first = payBadJSON.findings[0];
  return `<!doctype html><meta charset="utf-8"><style>${VIDEO_CSS}</style>
  <div class="scene" id="s0"><div class="kicker">c4-guardrails</div><div class="big">From terminal to pull request</div><div class="sub">Mermaid C4 in · table, SARIF, Markdown out</div></div>
  <div class="scene" id="s1" style="justify-content:flex-start;padding-top:110px"><div class="mid">Payments platform (DORA third-party risk)</div><div class="term" id="term"></div></div>
  <div class="scene" id="s2"><div class="mid">SARIF lands in the PR</div><div class="gh">
    <div class="row"><div class="x">✕</div><div><b>ci / guardrails</b><small>Failing: ${payBadJSON.summary.high} findings at or above high</small></div></div>
    <div class="alert"><div style="color:#cf222e;font-weight:700;font-size:26px">Code scanning · error</div><h3>${esc(first.rule_id)} ${esc(first.title)}</h3>
    <div style="color:#59636e;font-size:26px;margin-bottom:14px">examples/payments-bad.mmd line ${first.line} · ${esc(first.subject)}</div>
    ${first.clauses.map((c) => `<span class="chip">${esc(c)}</span>`).join('')}</div></div></div>
  <div class="scene" id="s3"><div class="mid">Fix the design, push again</div><div class="gh">
    <div class="row"><div class="x ok">✓</div><div><b>ci / guardrails</b><small>examples/payments-good.mmd · no findings</small></div></div>
    <div class="row"><div class="x ok">✓</div><div><b>ci / test</b><small>go vet · go test</small></div></div></div></div>
  <div class="scene" id="s4"><div class="end">c4-guardrails<br><span style="color:#8b949e;font-weight:600">open source</span></div><div class="url">github.com/rbrus/c4-guardrails</div><div class="sub">Runs locally and in CI. No account.</div></div>
  <script>
  const LINES = ${JSON.stringify(lines)};
  const CMD = 'c4g examples/payments-bad.mmd -sarif c4g.sarif';
  const on = (id) => { document.querySelectorAll('.scene').forEach(s => s.classList.toggle('on', s.id === id)); };
  const at = (ms, fn) => setTimeout(fn, ms);
  const term = document.getElementById('term');
  const esc = (s) => s.replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));
  const colour = (l) => esc(l).replace(/^(HIGH|CRITICAL)\\b/, '<b style="color:#ff7b72">$1</b>').replace(/^(MEDIUM)\\b/, '<b style="color:#ffa657">$1</b>').replace(/^(INFO)\\b/, '<b style="color:#79c0ff">$1</b>').replace(/\\b(CG\\d{3})\\b/g, '<span style="color:#d2a8ff">$1</span>').replace(/((?:GDPR|NIS2|DORA-RTS|DORA|AIACT):[^\\s,…]+)/g, '<span style="color:#56d4dd">$1</span>');
  at(200, () => on('s0'));
  at(3000, () => { on('s1'); term.innerHTML = '<span class="p">$</span> '; let i = 0;
    const t = setInterval(() => { term.innerHTML = '<span class="p">$</span> ' + esc(CMD.slice(0, ++i)); if (i >= CMD.length) { clearInterval(t);
      let k = 0; const u = setInterval(() => { if (k >= LINES.length) { clearInterval(u); term.innerHTML += '\\n<span class="p">$</span> echo $?\\n<b style="color:#ff7b72">1</b>'; return; }
        term.innerHTML += '\\n' + colour(LINES[k++]); }, 160); } }, 45); });
  at(12500, () => on('s2'));
  at(17000, () => on('s3'));
  at(20500, () => on('s4'));
  </script>`;
}

// ---------------------------------------------------------------- render
(async () => {
  const browser = await chromium.launch({ executablePath: CHROME });
  const shot = async (html, file, w, h, opts = {}) => {
    const page = await browser.newPage({ viewport: { width: w, height: h } });
    const tmp = o(path.basename(file, '.png') + '.page.html');
    fs.writeFileSync(tmp, html);
    await page.goto('file://' + tmp);
    await page.waitForTimeout(150);
    await page.screenshot({ path: path.join(MEDIA, file), fullPage: !!opts.fullPage });
    await page.close();
  };
  const shotFile = async (file, out, w, h) => {
    const page = await browser.newPage({ viewport: { width: w, height: h } });
    await page.goto('file://' + o(file));
    await page.screenshot({ path: path.join(MEDIA, out) });
    await page.close();
  };

  await shotFile('rag-bad.html', 'report-bad.png', 1600, 1000);
  await shotFile('rag-good.html', 'report-good.png', 1600, 1000);
  await shot(terminalPage('c4g examples/rag-chatbot-bad.mmd', ragBad.out, ragBad.code), 'terminal.png', 1600, 900);
  await shot(prPage(read('rag-bad.md')), 'pr-summary.png', 1460, 1000, { fullPage: true });
  await shot(beforeAfterPage(), 'before-after.png', 1800, 900, { fullPage: true });

  const record = async (html, name, ms) => {
    const dir = path.join(OUT, 'video-' + name);
    fs.rmSync(dir, { recursive: true, force: true });
    const ctx = await browser.newContext({ viewport: { width: 1080, height: 1920 }, recordVideo: { dir, size: { width: 1080, height: 1920 } } });
    const page = await ctx.newPage();
    const tmp = o(name + '.page.html');
    fs.writeFileSync(tmp, html);
    await page.goto('file://' + tmp);
    await page.waitForTimeout(ms);
    const video = page.video();
    await ctx.close();
    fs.copyFileSync(await video.path(), path.join(MEDIA, name + '.webm'));
  };
  await record(short1Page(), 'short-1-vertical', 25500);
  await record(short2Page(), 'short-2-vertical', 24000);
  await browser.close();

  // Optional: MP4/GIF when an ffmpeg with libx264 and the gif encoder exists.
  const ff = process.env.FFMPEG || 'ffmpeg';
  const probe = spawnSync(ff, ['-hide_banner', '-encoders'], { encoding: 'utf8' });
  if (probe.status === 0 && /libx264/.test(probe.stdout) && /\bgif\b/.test(probe.stdout)) {
    for (const n of ['short-1-vertical', 'short-2-vertical']) {
      execFileSync(ff, ['-y', '-i', path.join(MEDIA, n + '.webm'), '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-crf', '26', '-movflags', '+faststart', path.join(MEDIA, n + '.mp4')]);
    }
    execFileSync(ff, ['-y', '-i', path.join(MEDIA, 'short-1-vertical.webm'), '-vf', 'fps=10,scale=540:960:flags=lanczos,split[a][b];[a]palettegen=max_colors=96[p];[b][p]paletteuse', path.join(MEDIA, 'short-1-vertical.gif')]);
  } else {
    console.log('no ffmpeg with libx264 + gif found: kept .webm only');
  }
  console.log('media written to', path.relative(ROOT, MEDIA));
  void payGood; void ragGood;
})().catch((e) => { console.error(e); process.exit(1); });
