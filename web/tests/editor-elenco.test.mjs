import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createServer as createHttpServer } from "node:http";
import { fileURLToPath } from "node:url";
import { createServer as createViteServer } from "vite";
import { chromium } from "playwright";

const root = new URL("..", import.meta.url);

function jogador(id, nome, posicao) {
  return {
    id, name: nome, common_name: nome, position: posicao, alt_positions: [], rating: 88,
    gg_rating: 80 + id / 10, gg_rating_pos: posicao, gg_ratings: { [posicao]: 80 + id / 10 },
    club_item_id: `item-${id}`,
  };
}

function formacao(nome, posicoes, linhas) {
  const vagas = [{ index: 0, posicao: "GK", x: 50, y: 90 }];
  let index = 1;
  for (let line = 0; line < linhas.length; line++) {
    const size = linhas[line];
    for (let column = 0; column < size; column++) {
      vagas.push({ index, posicao: posicoes[index], x: size === 1 ? 50 : 88 - column * 76 / (size - 1), y: 90 - (line + 1) * 81 / linhas.length });
      index++;
    }
  }
  return { nome, vagas };
}

test("editor troca cartas, preserva a formação e ignora resposta atrasada", async () => {
  const posicoes = ["GK", "RB", "CB", "CB", "LB", "CDM", "CDM", "RM", "CAM", "LM", "ST"];
  const titulares = posicoes.map((position, index) => {
    const player = jogador(index + 1, `Titular ${index + 1}`, position);
    if (position === "RM") player.alt_positions = ["RW"];
    if (position === "LM") player.alt_positions = ["LW"];
    return { index, position, player };
  });
  const david = jogador(20, "J. David", "ST");
  const kolo = jogador(21, "Kolo Muani", "ST");
	const alvo = { ...jogador(30, "Alvo de mercado", "ST"), alvo_plano: true };
  const cartas = [...titulares.map(({ player }) => ({ player })), { player: david }, { player: kolo }];
  let planoSalvo;

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
  try {
    const page = await browser.newPage();
    await page.route("**/api/**", async (route) => {
      const request = route.request();
      const url = new URL(request.url());
      const json = (value, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(value) });
      if (url.pathname === "/api/editor/elenco" && request.method() === "GET") {
				return json({ generated_at: new Date().toISOString(), clube: "Teste", formacao: "4-2-3-1", titulares, cartas, media_atual: 82, alvos: [{ tipo: "mercado", player: alvo, referencia: { origem: "mercado", player_id: alvo.id }, custo: 120000, descricao: "alvo de compra para ST" }], funcoes: [{ nome: "Ponta", posicao: "RM" }, { nome: "Atacante", posicao: "ST" }], quimica_referencia: { total: 30, maximo: 33, jogadores: [], fora_de_posicao: 0, modelo: "teste", verificacao: { status: "confere", observado: 30, calculado: 30, modelo: "teste", jogadores_conferem: 11, jogadores_total: 11, pior_erro: 0 } }, avaliacao: { fonte: "futgg", ciclo: "27" } });
      }
			if (url.pathname === "/api/formacoes") return json({ ciclo: "27", value: [
				formacao("4-2-3-1", posicoes, [4, 2, 3, 1]),
				formacao("4-2-1-3", ["GK", "RB", "CB", "CB", "LB", "CDM", "CDM", "CAM", "RW", "ST", "LW"], [4, 2, 1, 3]),
				formacao("4-4-2", ["GK", "RB", "CB", "CB", "LB", "RM", "CM", "CM", "LM", "ST", "ST"], [4, 4, 2]),
			] });
			if (url.pathname === "/api/resumo") return json({ avisos: [], squad_score: 82, trocas_viaveis: 0, analise_entra_no_xi: 0, catalogo_elegiveis: 0, salvos: 0 });
			if (url.pathname === "/api/status") return json({ running: false });
      if (url.pathname === "/api/editor/elenco/avaliar" && request.method() === "POST") {
        const body = request.postDataJSON();
        const atacante = body.vagas.find((vaga) => vaga.index === 10)?.carta?.player_id;
        if (atacante === david.id) await new Promise((resolve) => setTimeout(resolve, 900));
        if (atacante === kolo.id) await new Promise((resolve) => setTimeout(resolve, 10));
				const media = atacante === david.id ? 70 : atacante === kolo.id ? 90 : atacante === alvo.id ? 91 : 82;
				const jogadores = [...cartas.map(({ player }) => player), alvo];
				return json({ status: "ok", formacao: body.formacao, revisao_plano: `rascunho:${atacante}`, vagas: body.vagas.map((vaga) => ({ index: vaga.index, posicao: vaga.posicao, carta: jogadores.find((player) => player.id === vaga.carta.player_id), nota: media, nota_disponivel: true, fora_de_posicao: false })), media, cobertura: 11, avaliacao: { fonte: "futgg", ciclo: "27" } });
      }
      if (url.pathname === "/api/planos/elenco/salvos" && request.method() === "GET") return json({ value: planoSalvo ? [{ plano: planoSalvo }] : [], "@odata.count": planoSalvo ? 1 : 0 });
      if (url.pathname === "/api/planos/elenco/salvos" && request.method() === "POST") {
        const body = request.postDataJSON();
        planoSalvo = { ...body, id: "plano-1", ciclo: "27", clube: "Teste", revisao: 1, referencia: false, criado_em: new Date().toISOString(), atualizado_em: new Date().toISOString() };
        return json({ plano: planoSalvo });
      }
      return json({});
    });

    await page.goto(`http://127.0.0.1:${port}/time/editor`);
    await page.waitForSelector(".editor-slot");
    assert.equal(await page.locator(".editor-slot").count(), 11);

		await page.getByPlaceholder("Nome, posição ou evolução").fill("David");
    await page.getByRole("button", { name: /J\. David/ }).focus();
    await page.keyboard.press("Enter");
    await page.locator('[data-slot-index="10"]').getByRole("button", { name: /ST:/ }).focus();
    await page.keyboard.press("Enter");
    await page.waitForTimeout(300);

		await page.getByPlaceholder("Nome, posição ou evolução").fill("Kolo");
    await page.getByRole("button", { name: /Kolo Muani/ }).click();
    await page.locator('[data-slot-index="10"]').getByRole("button", { name: /ST:/ }).click();
    await page.waitForFunction(() => document.querySelector(".editor-avg")?.textContent?.includes("90.0"));
    await page.waitForTimeout(700);
    assert.match(await page.locator(".editor-avg").innerText(), /90\.0/);
    // media_atual (82) é a base do "vs jogo": o delta só aparece com o XI inteiro avaliado.
    assert.match(await page.locator(".editor-delta").innerText(), /\+8\.0 vs jogo/);
		await page.getByLabel("Entrosamento").selectOption("Caçador");
		await page.locator('[data-slot-index="7"]').getByRole("button", { name: /RM:/ }).click();
		await page.getByLabel("Função").fill("Ponta");

		await page.locator(".editor-controlbar select").first().selectOption("4-2-1-3");
		assert.match(await page.locator('[data-slot-index="7"]').innerText(), /Titular 9/);
		assert.match(await page.locator('[data-slot-index="8"]').innerText(), /Titular 8/);
		assert.match(await page.locator('[data-slot-index="9"]').innerText(), /Kolo Muani/);
		assert.match(await page.locator('[data-slot-index="10"]').innerText(), /Titular 10/);
		assert.equal(await page.locator(".editor-slot").count(), 11);
		await page.locator('[data-slot-index="9"]').getByRole("button", { name: /ST:/ }).click();
		assert.equal(await page.getByLabel("Entrosamento").inputValue(), "Caçador");
		await page.locator('[data-slot-index="8"]').getByRole("button", { name: /RW:/ }).click();
		assert.equal(await page.getByLabel("Função").inputValue(), "");
		await page.getByRole("button", { name: "desfazer" }).click();
		assert.match(await page.locator('[data-slot-index="10"]').innerText(), /Kolo Muani/);
		await page.getByRole("button", { name: "refazer" }).click();
		assert.match(await page.locator('[data-slot-index="9"]').innerText(), /Kolo Muani/);

    await page.locator(".editor-controlbar select").first().selectOption("4-4-2");
    assert.match(await page.locator('[data-slot-index="9"]').innerText(), /Kolo Muani/);
    assert.match(await page.locator(".editor-controlbar label").first().innerText(), /manual/i);

		await page.getByRole("tab", { name: /alvos/ }).click();
		await page.getByPlaceholder("Nome, posição ou evolução").fill("");
		await page.getByRole("button", { name: /Alvo de mercado/ }).click();
		await page.locator('[data-slot-index="10"]').getByRole("button", { name: /ST:/ }).click();
		await page.waitForFunction(() => document.querySelector(".editor-avg")?.textContent?.includes("91.0"));
		assert.match(await page.locator('[data-slot-index="10"]').innerText(), /alvo mercado/i);

    await page.getByLabel("Nome do plano").fill("Plano teclado");
    await page.getByRole("button", { name: "salvar plano" }).click();
    await page.waitForSelector("text=Plano salvo.");
    assert.equal(planoSalvo.nome, "Plano teclado");
		assert.equal(planoSalvo.vagas[10].carta.player_id, alvo.id);
		assert.equal(planoSalvo.vagas[10].carta.origem, "mercado");
		assert.equal(planoSalvo.vagas[9].estilo_entrosamento, "Caçador");
    assert.equal(planoSalvo.origem_formacao, "manual");
		await page.reload();
		await page.locator(".editor-saved").getByRole("button", { name: "abrir" }).click();
		assert.equal(await page.locator(".editor-controlbar select").first().inputValue(), "4-4-2");
		assert.match(await page.locator('[data-slot-index="10"]').innerText(), /Alvo de mercado/);
  } finally {
    await browser.close();
    await new Promise((resolve) => http.close(resolve));
    await vite.close();
  }
});
