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

test("Planejador explica somente a cobertura que bloqueou o plano", async () => {
  const vite = await createViteServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true, hmr: false }, appType: "spa" });
  const http = createHttpServer((req, res) => vite.middlewares(req, res, () => { res.statusCode = 404; res.end(); }));
  await new Promise((resolve) => http.listen(0, "127.0.0.1", resolve));
  const address = http.address();
  const port = typeof address === "object" && address ? address.port : 0;
  const unavailable = {
    generated_at: "2026-09-18T12:00:00Z", status: "unavailable", formation: "4-4-1-1", scenarios: [], needs: [], warnings: [],
    reason: "RM Lewis Miley: GG Rating ausente para esta vaga",
    avaliacao: { fonte: "futgg" },
  };
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.route("**/api/planos/elenco", async (route) => {
      await route.fulfill({ contentType: "application/json", body: JSON.stringify(unavailable) });
    });
    await page.goto(`http://127.0.0.1:${port}/tests/fixtures/plano-elenco-unavailable.html`);
    await page.waitForSelector("text=RM Lewis Miley", { timeout: 5000 });
    const body = await page.locator("body").innerText();
    assert.match(body, /RM Lewis Miley: GG Rating ausente para esta vaga/);
    assert.doesNotMatch(body, /escalação titular sincronizada/);
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
});

// O chip do campo recebe GG geral e GG da vaga. Quando ele pinta o elo mais
// fraco, precisa exibir a segunda medida: usar o GG geral faz uma carta 99,0
// parecer acima de uma vaga 98,8 mesmo que ela esteja em 98,3 naquela vaga.
test("o campo exibe o GG da vaga que decide o elo mais fraco", async () => {
  const source = await read("src/components/GGRating.tsx");
  assert.match(source, /const visibleRating = variant === "pitch" && isKnownGGRating\(positional\) \? positional : current/);
});

test("o campo usa a cópia física como chave e mostra os dois contextos", async () => {
  const pitch = await read("src/components/Pitch.tsx");
  const time = await read("src/pages/Time.tsx");
  assert.match(pitch, /card\.player\.club_item_id \|\|/);
  assert.match(time, /p\.club_item_id \|\|/);
  assert.match(pitch, /current=\{player\.gg_rating\}/);
  assert.match(time, /positional=\{positionalGGRating\}/);
});

// A tela recebe tanto o GG publicado quanto a nota da régua ativa. Se a
// preferência seleciona o perfil local, chamar esta última de "GG" faz os
// dois números da mesma carta parecerem contraditórios.
test("Meu time identifica a fonte da nota usada na vaga", async () => {
  const time = await read("src/pages/Time.tsx");
  assert.match(time, /evaluationSourceLabel\(data\.avaliacao\?\.fonte\)/);
  assert.match(time, /sourceLabel/);
  assert.doesNotMatch(time, /GG na vaga/);
  assert.doesNotMatch(time, /GG posicional/);
});

test("Meu time mostra a régua ativa, em vez de chamar a nota do bot de GG", async () => {
  const vite = await createViteServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true, hmr: false }, appType: "spa" });
  const http = createHttpServer((req, res) => vite.middlewares(req, res, () => { res.statusCode = 404; res.end(); }));
  await new Promise((resolve) => http.listen(0, "127.0.0.1", resolve));
  const address = http.address();
  const port = typeof address === "object" && address ? address.port : 0;
  const player = { id: 1, name: "Ezri Konsa", common_name: "Ezri Konsa", position: "CB", alt_positions: [], rating: 84, gg_rating: 80.1, gg_rating_pos: "CB" };
  const time = { avaliacao: { fonte: "bot" }, formation: "", starters: [], bench: [], bench_page: 1, bench_page_size: 24, bench_total: 1, optimization: {}, position_map: [], regua: 0, slot_outlook: [], price_series: {}, price_history_status: {} };
  const bench = { value: [{ player, leitura: { kind: "promover", promocao: { slot_index: 3, position: "CB", starter_name: "Gerard Martín", starter_rating: 78.0, candidate_rating: 84.2, gain: 6.2 } } }], "@odata.count": 1, "@eafc.price_series": {}, "@eafc.price_history_status": {} };
  const starters = { value: [{ index: 3, position: "CB", player, position_gg_rating: 84.2 }] };
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.route("**/api/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      const body = path === "/api/time" ? time : path === "/api/elenco/titulares" ? starters : bench;
      await route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    });
    await page.goto(`http://127.0.0.1:${port}/tests/fixtures/time-evaluation-source.html`);
    await page.waitForSelector("text=nota do bot na vaga");
    assert.match(await page.locator("body").innerText(), /84\.2 vs 78\.0 \+6\.2 nota do bot na vaga/);
    assert.doesNotMatch(await page.locator("body").innerText(), /84\.2 vs 78\.0 \+6\.2 GG na vaga/);
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
});

test("Meu time descarta promoção legada baseada em metarank", async () => {
  const vite = await createViteServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true, hmr: false }, appType: "spa" });
  const http = createHttpServer((req, res) => vite.middlewares(req, res, () => { res.statusCode = 404; res.end(); }));
  await new Promise((resolve) => http.listen(0, "127.0.0.1", resolve));
  const address = http.address();
  const port = typeof address === "object" && address ? address.port : 0;
  const dybala = { id: 211110, name: "Paulo Dybala", common_name: "Paulo Dybala", position: "CAM", alt_positions: ["ST"], rating: 85, gg_rating: 83.95, gg_rating_pos: "CAM" };
  const time = { avaliacao: { fonte: "futgg" }, formation: "", starters: [], bench: [], bench_page: 1, bench_page_size: 24, bench_total: 1, optimization: {}, position_map: [], regua: 0, slot_outlook: [], price_series: {}, price_history_status: {} };
  const bench = { value: [{ player: dybala, leitura: { kind: "promover", promocao: { slot_index: 10, position: "ST", starter_name: "Kerolin Nicoli", starter_rating: 82.79, candidate_rating: 86.28, gain: 3.49, metric: "metarank" } } }], "@odata.count": 1, "@eafc.price_series": {}, "@eafc.price_history_status": {} };
  const starters = { value: [] };
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.route("**/api/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      const body = path === "/api/time" ? time : path === "/api/elenco/titulares" ? starters : bench;
      await route.fulfill({ contentType: "application/json", body: JSON.stringify(body) });
    });
    await page.goto(`http://127.0.0.1:${port}/tests/fixtures/time-evaluation-source.html`);
    await page.waitForSelector("text=Sem GG Rating confirmado para comparar nesta vaga", { timeout: 5000 });
    const body = await page.locator("body").innerText();
    assert.doesNotMatch(body, /86\.3|82\.8|\+3\.5|Metarank|Escalar na ST/);
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
});

test("evoluções permanecem explicitamente finais", async () => {
  const detail = await read("src/pages/CardDetail.tsx");
  const gauntlet = await read("src/pages/Gauntlet.tsx");
  assert.match(detail, /GG final/);
  assert.match(gauntlet, /GG final/);
});

test("detalhe ignora metarank antigo e mantém sem nota as demais posições", async () => {
  const vite = await createViteServer({ root: fileURLToPath(root), server: { middlewareMode: true, hmr: false }, appType: "spa" });
  const http = createHttpServer((req, res) => vite.middlewares(req, res, () => { res.statusCode = 404; res.end(); }));
  await new Promise((resolve) => http.listen(0, "127.0.0.1", resolve));
  const port = http.address().port;
  const browser = await chromium.launch({ headless: true });
  const player = {
    id: 211110, name: "Paulo Dybala", position: "CAM", alt_positions: ["ST"], rating: 85,
    gg_rating: 83.95, gg_rating_pos: "CAM", gg_ratings: { CAM: 91.5, ST: 86.28, LM: 98 },
    attributes: { pace: 78, shooting: 84, passing: 85, dribbling: 88, defending: 40, physical: 60 },
  };
  try {
    for (const width of [1440, 390]) {
      const page = await browser.newPage({ viewport: { width, height: 900 } });
      const errors = [];
      page.on("pageerror", (error) => errors.push(error.message));
      await page.route("**/api/**", (route) => route.fulfill({ contentType: "application/json", body: JSON.stringify({ player, generated_at: "2026-09-19T12:00:00Z", slug: "dybala" }) }));
      await page.goto(`http://127.0.0.1:${port}/tests/fixtures/card-detail-gg-rating.html`);
      const positions = page.getByRole("tablist");
      await positions.waitFor();
      assert.match(await positions.innerText(), /84\.0\s*CAM/);
      assert.match(await positions.innerText(), /—\s*ST/);
      assert.doesNotMatch(await positions.innerText(), /91\.5|86\.3|98\.0|LM/);
      await positions.getByRole("tab", { name: /ST/ }).click();
      assert.equal(await positions.getByRole("tab", { name: /ST/ }).getAttribute("aria-selected"), "true");
      assert.deepEqual(errors, []);
      await page.close();
    }
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
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
  const vite = await createViteServer({ root: fileURLToPath(new URL("..", import.meta.url)), server: { middlewareMode: true, hmr: false }, appType: "spa" });
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
