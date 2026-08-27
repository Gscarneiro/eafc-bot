#!/usr/bin/env node
// REPL driver for eafc-bot's React UI: stdin commands -> Playwright actions
// against a headless Chromium page. Mirrors the chromium-cli vocabulary
// (nav/wait-for/screenshot/click/fill/press/console) described in the
// "run" skill's playwright.md example, so that example applies here
// almost verbatim even though chromium-cli itself isn't installable in
// this environment (see .claude/skills/run-web/SKILL.md).
//
// Usage (run from web/): node scripts/browser-driver.mjs
// Point `nav` at whichever server is actually serving the UI — see
// SKILL.md for the two options (embedded `serve -demo` vs Vite dev server).

import { chromium } from "playwright";
import * as readline from "node:readline";
import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const SHOT_DIR = process.env.SCREENSHOT_DIR || path.join(__dirname, "..", "..", ".claude", "skills", "run-web", "screenshots");
fs.mkdirSync(SHOT_DIR, { recursive: true });

let browser = null;
let page = null;
let consoleLog = [];

function trackConsole(target) {
  consoleLog = [];
  target.on("console", (msg) => consoleLog.push({ type: msg.type(), text: msg.text() }));
  target.on("pageerror", (err) => consoleLog.push({ type: "pageerror", text: err.message }));
}

const COMMANDS = {
  async launch() {
    if (browser) return console.log("already launched");
    browser = await chromium.launch({ headless: true });
    page = await browser.newPage();
    trackConsole(page);
    console.log("launched.");
  },

  async nav(url) {
    if (!url) return console.log("ERROR: nav <url>");
    if (!page) await COMMANDS.launch();
    await page.goto(url, { waitUntil: "domcontentloaded", timeout: 30_000 });
    console.log("nav ->", url);
  },

  async "wait-for"(selector) {
    if (!page) return console.log("ERROR: launch/nav first");
    try {
      await page.waitForSelector(selector, { timeout: 10_000 });
      console.log("found:", selector);
    } catch {
      console.log("TIMEOUT:", selector);
    }
  },

  async screenshot(name) {
    if (!page) return console.log("ERROR: launch/nav first");
    const file = path.join(SHOT_DIR, (name || `ss-${Date.now()}`) + ".png");
    await page.screenshot({ path: file, fullPage: true });
    fs.copyFileSync(file, path.join(SHOT_DIR, "screenshot.png"));
    console.log("screenshot:", file);
  },

  async click(selector) {
    if (!page) return console.log("ERROR: launch/nav first");
    await page.click(selector, { timeout: 10_000 });
    console.log("click ->", selector);
  },

  // Selector must be a single whitespace-free token — everything after it
  // is the value to type. Good enough for the CSS/text= selectors this
  // driver actually gets used with; a selector with a literal space in it
  // needs `eval` instead.
  async fill(args) {
    if (!page) return console.log("ERROR: launch/nav first");
    const spaceIdx = args.indexOf(" ");
    const selector = spaceIdx === -1 ? args : args.slice(0, spaceIdx);
    const value = spaceIdx === -1 ? "" : args.slice(spaceIdx + 1);
    await page.fill(selector, value, { timeout: 10_000 });
    console.log("fill", selector, "->", value);
  },

  async press(key) {
    if (!page) return console.log("ERROR: launch/nav first");
    await page.keyboard.press(key);
    console.log("press ->", key);
  },

  async eval(expr) {
    if (!page) return console.log("ERROR: launch/nav first");
    try {
      console.log(JSON.stringify(await page.evaluate(expr)));
    } catch (e) {
      console.log("ERROR:", e.message);
    }
  },

  async text(selector) {
    if (!page) return console.log("ERROR: launch/nav first");
    const value = await page.evaluate(
      (s) => (s ? document.querySelector(s) : document.body)?.innerText ?? "(null)",
      selector || null,
    );
    console.log(value);
  },

  console(flag) {
    const errors = consoleLog.filter((m) => m.type === "error" || m.type === "pageerror");
    const list = flag === "--errors" ? errors : consoleLog;
    if (list.length === 0) {
      console.log(flag === "--errors" ? "no console errors" : "no console output");
      return;
    }
    for (const m of list) console.log(`[${m.type}]`, m.text);
  },

  async quit() {
    if (browser) await browser.close().catch(() => {});
    browser = null;
    page = null;
  },

  help() {
    console.log("commands:", Object.keys(COMMANDS).join(", "));
  },
};

const rl = readline.createInterface({ input: process.stdin, output: process.stdout, prompt: "driver> " });

// Piped stdin (heredoc scripts) delivers every line to readline's "line"
// event essentially at once — an async listener does NOT make Node wait
// for it to resolve before emitting the next line. Without this queue,
// "quit" (or anything later in the script) starts running concurrently
// with "nav" and can close the browser before nav's own await finishes.
// A queue drained one item at a time is what makes the script actually
// run in order.
const queue = [];
let busy = false;

async function handleLine(line) {
  const trimmed = line.trim();
  if (!trimmed) return;
  const spaceIdx = trimmed.indexOf(" ");
  const cmd = spaceIdx === -1 ? trimmed : trimmed.slice(0, spaceIdx);
  const rest = spaceIdx === -1 ? "" : trimmed.slice(spaceIdx + 1);
  const fn = COMMANDS[cmd];
  if (!fn) {
    console.log("unknown:", cmd, "— try: help");
    return;
  }
  try {
    await fn(rest);
  } catch (e) {
    console.log("ERROR:", e.message);
  }
  if (cmd === "quit") {
    rl.close();
    process.exit(0);
  }
}

async function drainQueue() {
  if (busy) return;
  busy = true;
  while (queue.length > 0) {
    await handleLine(queue.shift());
    rl.prompt();
  }
  busy = false;
}

rl.on("line", (line) => {
  queue.push(line);
  void drainQueue();
});
// Piped stdin (heredoc scripts) fires "close" the instant EOF is read —
// essentially immediately, well before an async queue draining a slow
// command like "nav" has finished. Exiting right away here would kill the
// browser mid-navigation. Poll until the queue is actually empty AND
// nothing is still running before quitting for real. (When the script
// ends with an explicit "quit", handleLine's own quit branch has usually
// already called process.exit before this ever runs — this is the
// fallback for scripts that don't.)
rl.on("close", async () => {
  while (busy || queue.length > 0) {
    await new Promise((resolve) => setTimeout(resolve, 20));
  }
  await COMMANDS.quit();
  process.exit(0);
});

console.log('eafc-bot browser driver — "help" for commands, "nav <url>" to start');
rl.prompt();
