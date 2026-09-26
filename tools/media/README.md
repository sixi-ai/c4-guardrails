# tools/media

Regenerates every file in `docs/media/` from the examples. It is **not** needed to build or test `c4g`,
and it adds no dependency to the Go module.

Requirements: Go, Node.js 22+, a Chromium binary, and `playwright-core` installed **outside** the repo.

```sh
npm i --prefix /tmp/media-tools playwright-core
NODE_PATH=/tmp/media-tools/node_modules \
CHROME=/path/to/chromium \
node tools/media/generate.cjs
```

What it does:

1. Builds `c4g` into `tools/media/out/` (git-ignored) and runs it on `examples/`.
2. Screenshots the real HTML reports (`report-bad.png`, `report-good.png`, 1600×1000).
3. Renders the real table output in a terminal-style page (`terminal.png`), the real `-md` output in a
   GitHub-like comment (`pr-summary.png`) and the two diagram SVGs side by side (`before-after.png`).
4. Records two 1080×1920 clips with Playwright (`short-1-vertical.webm`, `short-2-vertical.webm`).
5. If an `ffmpeg` with `libx264` and the GIF encoder is on `PATH` (or in `$FFMPEG`), also writes `.mp4`
   versions and a 540×960 `short-1-vertical.gif`. Otherwise it keeps the `.webm` files only.

Nothing is hand-edited: change an example or the renderer, rerun the script.
