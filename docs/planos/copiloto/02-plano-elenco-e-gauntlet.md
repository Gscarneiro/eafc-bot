# Fase 02 — plano de elenco e Gauntlet

## Estado da implementação

Concluída nesta entrega, com uma lacuna documentada (empréstimos, abaixo).

### Gauntlet generalizado

- `analyze.GauntletRules`/`GauntletRequest` (`internal/analyze/gauntlet_rules.go`)
  generalizam o Gauntlet para regras versionadas: 3 a 5 rodadas (era
  hardcoded em 4), quatro estratégias nomeadas (crescente, mais forte
  primeiro, equilibrada, valor total), locks e exclusões. Nenhuma reescreve
  `matchGauntletRound` — as quatro estratégias só decidem QUAL RODADA
  escolhe primeiro no pool que sobra; equilibrada/valor_total decidem isso
  por simulação a cada passo (O(rodadas²) chamadas do matching existente,
  barato para 3–5 rodadas).
- `BuildGauntletPlan`/`BuildGauntletPlanWithOptions` continuam existindo,
  agora como atalhos que montam `DefaultGauntletRequest` — nenhum chamador
  precisou mudar, e `TestBuildGauntletPlanWithOptionsBateComRequestPadrao`
  prova a equivalência campo a campo com o motor geral.
- Locks: com `ClubItemID` confirmado, prendem a cópia física exata num
  slot; sem ele, degradam para "quantidade preservada" (garante o jogador
  na rodada, sem prometer slot) — mesma distinção de procedência da fase 01.
  Resolvidos TODOS de uma vez, antes de qualquer rodada ser processada, para
  uma rodada sem lock não roubar sem querer a cópia que outra reservou.
- Química entra na função objetivo como uma segunda fase de busca local
  (`chemistrySwapRound`, sobre `chemistry.Contador`) DEPOIS do matching por
  GG Rating — nunca dentro dele. Peso 0 nunca chama essa fase, preservando o
  comportamento histórico por construção, não por coincidência numérica.
- Formação aceita "observada" (do clube) e "manual" (slots explícitos);
  "preset comprovado" fica de fora — nenhum catálogo de formações do FC 27
  foi confirmado em lugar nenhum, e inventar um violaria CLAUDE.md.
- `GET /api/gauntlet` ganhou `?strategy=`, `?rodadas=` e
  `?chemistry_weight=` opcionais, que forçam recompute contra o motor
  geral; sem eles, continua servindo o plano persistido no snapshot, sem
  tocar rede.

### Squad Planner

- `analyze.SquadPlanRequest`/`BuildSquadPlan` (`internal/analyze/squad_planner.go`)
  reaproveitam o mesmo `squadMatch` que `OptimizeSquad` sempre usou — extraído
  de `squad_optimizer.go` sem mudar comportamento nenhum
  (`TestBuildSquadScenarioPesoZeroBateComOptimizeSquad` prova a equivalência).
  Locks e formação seguem exatamente a mesma distinção do Gauntlet
  (`FormationSource` é literalmente o mesmo tipo, compartilhado entre os
  dois via `resolveFormationSource`).
- A fronteira Pareto nota×química é uma aproximação por SOMA PONDERADA:
  varre um conjunto fixo de pesos de química (`squadPlanWeightSweep`),
  aplica `chemistrySwapSquad` (equivalente de `chemistrySwapRound` para um
  XI só, sem rodadas) em cada um, e mantém os cenários não dominados, até
  `MaxScenarios` (3 a 5). É uma técnica conhecida, mas só alcança a parte
  CONVEXA da fronteira real — pontos não-convexos podem ficar de fora
  (documentado no código; CLAUDE.md: não afirmar o que não é provado).
- "Necessidades de mercado" (`SquadPlanNeed`) aponta posição sem alternativa
  no banco ou notavelmente abaixo da média do time — nunca escolhe qual
  carta comprar; isso continua sendo `analyze.FindUpgrades`, do lado do
  mercado.
- Orçamento não entra no motor: o Squad Planner nunca escolhe compra
  nenhuma, então dinheiro não influencia quem entra no XI. `POST
  /api/planos/elenco` devolve `domain.Capital` como contexto de exibição,
  computado do mesmo jeito que toda outra rota já faz (`s.EvolutionExtraBudget`/
  `s.MarketReserve`), sem precisar de campo nenhum no corpo da requisição.
- `POST /api/planos/elenco` (`internal/api/squad_plan.go`) aceita objetivo,
  formação, locks, exclusões e máximo de cenários no corpo JSON — tudo
  opcional, caindo no padrão quando omitido. `GET /api/gauntlet` não muda:
  continua a única rota do Gauntlet, sem depender do Squad Planner.
- Tela de comparação em `/time/planos` (`web/src/pages/PlanoElenco.tsx`):
  abas por cenário (rótulo + nota média), campo tático via o componente
  `Pitch` já existente, banner de necessidades e lista de movimentos em
  relação ao XI atual — mesmo padrão visual do Gauntlet
  (`web/src/pages/Gauntlet.tsx`), verificado sem erro de console em
  `/time`, `/time/gauntlet` e `/time/planos` via Playwright.

### Lacuna documentada: empréstimos

`domain.ClubPlayer` não tem campo nenhum marcando uma carta como
empréstimo, e não há confirmação de que o envelope do GG Club exponha esse
status por carta — por isso "empréstimos" (o único item do contrato de
entrada que faltou) não entra em `SquadPlanRequest`/`GauntletRequest`.
Adicionar um campo que não pode filtrar nada de verdade seria uma
funcionalidade fingida (CLAUDE.md: na dúvida, não afirma). Decisão
confirmada com o usuário nesta entrega; ver `docs/pesquisa-futgenie.md`,
seção 6, para o precedente do FutGenie (`PermitirLoans`).

## Contratos

Criar entrada de planejamento com objetivo, formação, química, orçamento, locks, exclusões, empréstimos e máximo de alternativas. A saída contém três a cinco cenários Pareto, necessidades de mercado, movimentos, explicações e diagnósticos de inviabilidade.

Generalizar Gauntlet para regras versionadas: 3–5 rodadas, titulares, banco, restrições e estratégias equilibrada, crescente, mais forte primeiro e valor total. O default reproduz o comportamento atual de quatro rodadas.

## Implementação

- Evoluir o optimizer atual sem reescrever o matching; química passa a participar da função objetivo, com peso zero mantendo compatibilidade.
- Impedir duas versões do mesmo `PlayerKey` no XI e impedir reutilização da mesma cópia física entre rodadas. Lock individual só é permitido com `ClubItemID` confirmado; caso contrário usar quantidade preservada.
- Aceitar formação observada, preset comprovado ou formação manual; não inventar formações do FC 27.
- Emitir necessidades para o mercado, sem escolher compras dentro do Squad Planner.
- Adicionar `POST /api/planos/elenco`, manter `/api/gauntlet` como wrapper compatível e criar tela de comparação.

## Gate

Fixtures comprovam pesos de química, locks, duplicatas, estratégias distintas, 3/4/5 rodadas e explicação de falta por posição ou requisito.

