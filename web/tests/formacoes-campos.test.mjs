import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer as createHttpServer } from "node:http";
import { fileURLToPath } from "node:url";
import { createServer as createViteServer } from "vite";
import { chromium } from "playwright";

const root = new URL("..", import.meta.url);

test("Gauntlet desenha 4-2-1-3 em desktop e celular e preserva tabela quando a formação é desconhecida", async () => {
  const vite = await createViteServer({ root: fileURLToPath(root), server: { middlewareMode: true, hmr: false }, appType: "spa" });
  const indexHtml = await readFile(new URL("index.html", root), "utf8");
  const http = createHttpServer((req, res) => vite.middlewares(req, res, async () => {
    if ((req.headers.accept ?? "").includes("text/html")) {
      const html = await vite.transformIndexHtml(req.url ?? "/", indexHtml);
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(html);
      return;
    }
    res.statusCode = 404;
    res.end();
  }));
  await new Promise((resolve) => http.listen(0, "127.0.0.1", resolve));
  const address = http.address();
  const port = typeof address === "object" && address ? address.port : 0;
  const browser = await chromium.launch({ headless: true });
  let formationName = "4-2-1-3";
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 720 } });
    await page.route("**/api/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      const json = (value) => route.fulfill({ contentType: "application/json", body: JSON.stringify(value) });
      if (path === "/api/formacoes") return json({ ciclo: "27", value: [{ nome: "4-2-1-3", vagas: [
        { index: 0, posicao: "GK", x: 50, y: 90 },
        ...["RB", "CB", "CB", "LB"].map((posicao, i) => ({ index: i + 1, posicao, x: 88 - i * 76 / 3, y: 69.75 })),
        ...["CDM", "CDM"].map((posicao, i) => ({ index: i + 5, posicao, x: 67 - i * 34, y: 49.5 })),
        { index: 7, posicao: "CAM", x: 50, y: 29.25 },
        ...["RW", "ST", "LW"].map((posicao, i) => ({ index: i + 8, posicao, x: 84 - i * 34, y: 9 })),
      ] }, { nome: "5-4-1", vagas: [
        { index: 0, posicao: "GK", x: 50, y: 90 },
        ...["RB", "CB", "CB", "CB", "LB"].map((posicao, i) => ({ index: i + 1, posicao, x: 88 - i * 19, y: 63 })),
        ...["RM", "CM", "CM", "LM"].map((posicao, i) => ({ index: i + 6, posicao, x: 88 - i * 76 / 3, y: 36 })),
        { index: 10, posicao: "ST", x: 50, y: 9 },
      ] }] });
      if (path === "/api/gauntlet") {
        const positions = formationName === "5-4-1"
          ? ["GK", "RB", "CB", "CB", "CB", "LB", "RM", "CM", "CM", "LM", "ST"]
          : ["GK", "RB", "CB", "CB", "LB", "CDM", "CDM", "CAM", "RW", "ST", "LW"];
        return json({ generated_at: new Date().toISOString(), avaliacao: { fonte: "futgg", ciclo: "27" }, formation: formationName, status: "ok", rules: "Regra de teste", objectives: [], rounds: [{ round: 1, total_rating: 935, average_rating: 85, bench: [], starters: positions.map((position, index) => ({ index, position, player: { id: index + 1, name: `Carta ${index + 1}`, common_name: `Carta ${index + 1}`, position, rating: 85 }, rating: 85 })) }] });
      }
      if (path === "/api/status") return json({ running: false });
      if (path === "/api/resumo") return json({ avisos: [], squad_score: 85 });
      return json({});
    });

    await page.goto(`http://127.0.0.1:${port}/time/gauntlet`);
    await page.waitForSelector(".gauntlet-page .fut-pitch");
    assert.equal(await page.locator(".gauntlet-page .fut-slot").count(), 11);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 1280);

    await page.setViewportSize({ width: 390, height: 844 });
    assert.equal(await page.locator(".gauntlet-page .fut-slot").count(), 11);
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 390);
    await page.getByRole("button", { name: "tabela" }).click();
    assert.equal(await page.locator(".gauntlet-page .fut-pitch").count(), 0);
    assert.equal(await page.locator(".gauntlet-page tbody tr").count(), 11);
    await page.getByRole("button", { name: "campo" }).click();
    assert.equal(await page.locator(".gauntlet-page .fut-slot").count(), 11);

    formationName = "5-4-1";
    await page.reload();
    await page.waitForSelector(".gauntlet-page .fut-pitch.dense");
    const defense = page.locator('.gauntlet-page .fut-slot[style*="top: 63%"]');
    assert.equal(await defense.count(), 5);
    const defenders = await Promise.all(Array.from({ length: 5 }, (_, i) => defense.nth(i).boundingBox()));
    for (let i = 1; i < defenders.length; i++) {
      assert.ok(defenders[i - 1].x + defenders[i - 1].width <= defenders[i].x, "as cinco cartas da defesa não podem se sobrepor");
    }
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), 390);

    formationName = "formação sem catálogo";
    await page.reload();
    await page.getByText(/Formação desconhecida ou sem 11 vagas/).waitFor();
    assert.equal(await page.locator(".gauntlet-page .fut-pitch").count(), 0);
    assert.equal(await page.locator(".gauntlet-page tbody tr").count(), 11);
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
});
