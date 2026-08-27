---
name: run-web
description: Build, run, and drive eafc-bot's React web UI in a headless Chromium browser. Use when asked to start the web UI, take a screenshot of a screen, or verify a frontend change actually renders.
---

eafc-bot serves its React UI two ways: embedded in the Go binary
(`./eafcbot serve` / `serve -demo`, no network) or via Vite's dev server
(`npm run dev`, hot-reload, proxies `/api` to a `./eafcbot serve` running
alongside it — see AGENTS.md). For agent verification, prefer `serve
-demo`: one process, fictional data, no fut.gg network calls.

`chromium-cli` (referenced by the general `run` skill) isn't installable
in this environment — it 404s on npm and isn't shipped here. This skill
is the fallback the `run` skill itself recommends: a small Playwright
REPL driver at `web/scripts/browser-driver.mjs`, using the SAME command
vocabulary (`nav`/`wait-for`/`screenshot`/`click`/`fill`/`press`/`console`)
so the general skill's examples still apply almost verbatim.

## Prerequisites

Playwright + a headless Chromium are already installed
(`web/package.json` devDependency, browser cached under
`%LOCALAPPDATA%\ms-playwright`). If `browserType.launch: Executable
doesn't exist`, re-run from `web/`: `npx playwright install chromium`.

## Build + run (agent path)

```bash
cd web && npm install && npm run build && cd ..
go build ./cmd/eafcbot
./eafcbot serve -demo -port 4173 &
timeout 20 bash -c 'until curl -sf http://127.0.0.1:4173/api/status >/dev/null; do sleep 1; done'
```

`serve -demo` picks a random free port unless `-port` is passed
explicitly — always pass it so the driver knows where to `nav`.

## Drive it

```bash
cd web && node scripts/browser-driver.mjs <<'EOF'
nav http://127.0.0.1:4173/time/osimhen-88
wait-for text=Evolução
screenshot card-detail
console --errors
EOF
```

For iterative use, run it under tmux and `send-keys` one command at a
time instead of piping a whole script.

Screenshots land in `.Codex/skills/run-web/screenshots/` (latest also
copied to `screenshot.png` — Windows has no cheap symlink here, so it's a
real file copy, not a link).

### Commands

| command | what it does |
|---|---|
| `nav <url>` | go to a URL (auto-launches the browser first) |
| `wait-for <selector>` | wait for a selector — supports Playwright's `text=`/`:has-text()` |
| `screenshot [name]` | full-page screenshot → `screenshots/<name>.png` |
| `click <selector>` | click an element |
| `fill <selector> <text>` | fill an input (selector must be one token, no spaces) |
| `press <key>` | keyboard press (e.g. `Enter`) |
| `eval <js>` | evaluate JS in the page, print JSON |
| `text [selector]` | print `innerText` of a selector (or `document.body`) |
| `console` / `console --errors` | print captured console messages, optionally errors/pageerrors only |
| `quit` | close the browser, exit |

## Stop the server

```bash
kill %1   # or find the PID holding the port and stop it
```

## Gotchas

- **`serve -demo`'s fixtures are sparse for evolution.** Only Osimhen
  (`osimhen-88`) and Rodri (`rodri-89`) have a hand-built `CardReport`
  with `Best`/`Alternates`; neither has `Graph`/`Evolutions` populated
  (see `docs/planos/copiloto/03-plano-evolucao-e-workbench.md`), so the
  Workbench section on `/time/:slug` renders empty for both today —
  that's correct given the fixture, not a driver bug.
- **RTK's Bash hook can swallow stdout of some commands silently**
  (observed with `npx playwright install`) without the command actually
  failing. If a command seems to produce no output, verify the real side
  effect (file on disk, port open) before assuming it failed —
  `rtk proxy <cmd>` did not help here either; checking the filesystem was
  what actually confirmed success.

## Troubleshooting

- **`browserType.launch: Executable doesn't exist`**: Chromium wasn't
  downloaded for this machine/user. Run `npx playwright install chromium`
  from `web/`.
- **`nav` hangs or times out**: the server isn't up yet. Confirm first
  with `curl http://127.0.0.1:4173/api/status`; the build step above
  already polls for this before returning.
