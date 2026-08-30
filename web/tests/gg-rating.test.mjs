import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer as createHttpServer } from "node:http";
import { fileURLToPath } from "node:url";
import { createServer as createViteServer } from "vite";
import { chromium } from "playwright";

const root = new URL("..", import.meta.url);
const read = (path) => readFile(new URL(path, root), "utf8");

test("o presenter distingue GG atual e posicional sem fallback silencioso", async () => {
  const source = await read("src/components/GGRating.tsx");
  assert.match(source, /formatGGRating\(current\)/);
  assert.match(source, /GG atual/);
  assert.match(source, /GG posicional/);
  assert.match(source, /formatGGRating\(current\) !== formatGGRating\(positional\)/);
  assert.match(source, /return isKnownGGRating\(value\) \? value\.toFixed\(1\) : "—"/);
});

test("o campo usa a cópia física como chave e mostra os dois contextos", async () => {
  const pitch = await read("src/components/Pitch.tsx");
  const time = await read("src/pages/Time.tsx");
  assert.match(pitch, /card\.player\.club_item_id \|\|/);
  assert.match(time, /p\.club_item_id \|\|/);
  assert.match(pitch, /current=\{player\.gg_rating\}/);
  assert.match(time, /positional=\{positionalGGRating\}/);
});

test("evoluções permanecem explicitamente finais", async () => {
  const detail = await read("src/pages/CardDetail.tsx");
  const gauntlet = await read("src/pages/Gauntlet.tsx");
  assert.match(detail, /GG final/);
  assert.match(gauntlet, /GG final/);
});

test("filtro de posição da análise usa o resultado final de cada path", async () => {
  const analysis = await read("src/pages/AnaliseEvolucoes.tsx");
  assert.match(analysis, /function finalPathPosition/);
  assert.match(analysis, /final\?\.gg_rating_pos \|\| final\?\.position/);
  assert.match(analysis, /some\(path => finalPathPosition\(path\) === position\)/);
  assert.match(analysis, /const visiblePaths = position === "todas" \? paths : paths\.filter\(path => finalPathPosition\(path\) === position\)/);
  assert.match(analysis, /posição final do path/);
});

test("fixture Vite/Playwright mantém cópia atual, posicional e ausente", async () => {
  const vite = await createViteServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true }, appType: "spa" });
  const http = createHttpServer((req, res) => vite.middlewares(req, res, () => { res.statusCode = 404; res.end(); }));
  await new Promise((resolve) => http.listen(0, "127.0.0.1", resolve));
  const address = http.address();
  const port = typeof address === "object" && address ? address.port : 0;
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`http://127.0.0.1:${port}/tests/fixtures/gg-rating.html`);
    await page.waitForSelector("[data-testid=different] .gg-rating");
  assert.match(await page.locator("[data-testid=different]").innerText(), /atual.*88\.0.*pos\..*97\.7/is);
    assert.doesNotMatch(await page.locator("[data-testid=current-higher]").innerText(), /pos\./i);
    assert.doesNotMatch(await page.locator("[data-testid=equal]").innerText(), /pos\./i);
    assert.match(await page.locator("[data-testid=missing]").innerText(), /atual.*—.*pos\..*97\.7/is);
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
});
