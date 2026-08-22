# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

O código, os comentários, os nomes de teste e a saída da CLI são em
**português**. Escreva na mesma língua.

## Comandos

```bash
# A UI React de web/ é embutida no binário via //go:embed. O build TEM que
# existir antes: sem ele, `go build` falha (não é só o comando `serve` que
# quebra — é a compilação inteira).
cd web && npm install && npm run build && cd ..

go build ./cmd/eafcbot        # sem -o: no Windows o Go já nomeia eafcbot.exe

go test ./...
go test ./internal/analyze -run TestZagueiroLentoPerdeParaRapido -v   # um teste só

# O ciclo de verificação usado neste repo:
gofmt -l ./cmd ./internal; go vet ./...; go test ./... -count=1
```

```bash
./eafcbot demo -out briefing-demo.html   # pipeline inteiro, sem rede, dados fictícios
./eafcbot autoconfig -dry-run            # o que a descoberta acharia, sem gravar
./eafcbot discover players               # resposta crua de um endpoint configurado
./eafcbot run                            # o job uma vez: coleta + análise + grava snapshot/HTML
./eafcbot serve                          # API + UI em :4173, com scheduler diário embutido
./eafcbot serve -demo                    # as 5 telas com dado fictício, sem rede
```

`demo` é a forma mais rápida de exercitar motor de análise e layout do
relatório sem endpoint calibrado nem rede — use antes de sair batendo no site.
`serve -demo` é o equivalente para a UI React.

Interface com hot-reload: `./eafcbot serve` numa aba (API real na 4173) e
`cd web && npm run dev` noutra (Vite na 5173, com proxy de `/api`).

Postgres é opcional e fica atrás de build tag:

```bash
go get github.com/jackc/pgx/v5
go build -tags postgres ./cmd/eafcbot
psql "$EAFC_DSN" -f migrations/001_init.sql
```

## Restrições que valem mais que conveniência

**Nada de dependência externa no build padrão.** `go.mod` não tem um único
`require` — só biblioteca padrão. O driver do Postgres entra apenas sob a tag
`postgres`, por blank import em `cmd/eafcbot/driver_pgx.go`, para que
`internal/store/postgres.go` continue usando só `database/sql`. Antes de
propor qualquer biblioteca, considere que isso é uma decisão de projeto.

**O bot nunca toca na conta EA.** Sem login, sem automatizar o Web App, sem
enviar nada para a EA — automatizar o Web App é o que a EA bane (deleção de
clube e ban da conta). Toda leitura de elenco passa pela rota pública do
fut.gg. Não sugira caminho que contrarie isso.

**robots.txt é obedecido por padrão** na *descoberta* de rotas novas
(`autoconfig`): o do fut.gg tem `Disallow: /api/*` e `Disallow: /gg-club/`,
justamente onde estão os dados; o bot pula essas rotas e diz quantas pulou,
e ler assim mesmo é escolha explícita do usuário (`-ignore-robots`). Os
endpoints de PRODUÇÃO (já aprendidos, usados por `Collect`) não passam por
esse gate — travar isso quebraria a coleta diária inteira, já que são
justamente rotas `/api/*`. Em vez de gate, `futgg.checkRobots`
(`internal/futgg/robots.go`) conta quantas das rotas configuradas o
robots.txt bloquearia e expõe em `Stats.RobotsBypassed`, que `run`/`serve`
logam — a escolha de ler essas rotas já foi feita (ao aprender os endpoints
via `autoconfig -ignore-robots`), isto só garante que ela não fique
disfarçada no dia a dia. Não troque o User-Agent por um de navegador para
passar despercebido — ver o comentário de cabeçalho em
`internal/discover/robots.go`.

**Na dúvida, não afirma.** O `autoconfig` não grava mapeamento de campo
ambíguo (cai nos aliases do código e avisa no relatório); evolução com
requisito que o parser não entendeu é descartada de propósito. Um palpite
silencioso aqui vira recomendação errada sem rastro.

## Arquitetura

Fluxo: `config.Load` → `futgg.Client.Collect` → `cards.BuildReports` (evolução
carta a carta) → `analyze.*` → `report.Build`/`Render`, com `store.Store`
guardando histórico e o snapshot do dia no meio
(`cmd/eafcbot/main.go:runJob` é o job inteiro; `analyzeAndBuild` é o miolo
compartilhado com `demo`, que chama com `dryRun=true` e sem `cards`).
`serve` só troca QUEM decide a hora de chamar `runJob`: `run` chama uma vez
na mão, `internal/scheduler.Scheduler` chama sozinho todo dia (mais na
subida, se o último snapshot estiver velho — ver `ShouldRunNow`).

### `store.Snapshot` é plano de propósito

`internal/report` importa `internal/store` (por causa de `PriceTrend` em
`MarketRow`), então `internal/store` **não pode** importar `internal/report`
— faria um ciclo de import. Por isso `store.Snapshot` não embute
`report.Data`: ele guarda os mesmos ingredientes crus (`futgg.Snapshot` +
`analyze.Upgrade`/`EvoMatch`/`SquadSwap` + `cards.CardReport`) que
`report.Build` já recebe via `report.Input`. Quem quiser a visão "pronta pro
briefing" (nota do elenco, XI titular, tabelas cortadas) chama
`report.Build` — ou as peças exportadas dele (`report.SquadSummary`,
`report.MainSquad`, `report.RankChallenges`, `report.MarketRows`) — em vez
deste tipo reaprender a calcular isso. `internal/api` é quem faz isso hoje,
montando os envelopes JSON de cada rota por cima.

`report.Data` (e `report.SquadCard`) **não têm tag JSON** — são structs de
`html/template`, não contrato HTTP. `internal/api` define os próprios tipos
tagueados (`api.SquadCard`, `api.RosterCard`...) em volta de
`domain.*`/`analyze.*`/`cards.CardReport`, que já são tagueados.

Um snapshot com clube vazio **nunca** sobrescreve um snapshot bom — a trava
fica em `cmd/eafcbot/main.go:analyzeAndBuild` (`len(snap.Club.Players) == 0`
pula `st.SaveSnapshot` e vira aviso em `data.Errors`), não em `store`, que
grava o que recebe sem julgar.

### Endpoints são configuração, não código

`internal/discover` minera as rotas do próprio JavaScript do fut.gg e
classifica cada resposta pelo **formato do JSON**, nunca pelo nome da rota —
é o que faz a descoberta sobreviver à virada FC 26 → FC 27. O resultado é
gravado em `.eafc-bot/config.json` como `endpoints`, `field_maps` e
`wrappers`.

Na leitura, o tipo `lens` (`internal/futgg/client.go`) resolve cada campo
lógico: **nome aprendido primeiro, aliases do código depois**. Os aliases
vivem em `internal/futgg/map.go` — acrescentar um nome ali é a forma de
consertar o parser quando o site muda, mas rodar `autoconfig` de novo costuma
resolver antes.

O ciclo do jogo (`futgg.cycle`, `"26"`/`"27"`) é configuração e particiona os
dados no store. Não hardcode ciclo em lugar nenhum.

### Coleta tolera falha parcial

`futgg.Collect` busca todas as fontes em paralelo e acumula erros em
`Snapshot.Errors` em vez de abortar: um briefing diário que não entrega nada
porque uma rota mudou é pior que um que entrega 80%. O clube é a exceção que
merece destaque no console (ver `clubErrorMessage` em `main.go`).

Fallbacks, do mais barato ao mais caro: API → payload do Next.js embutido no
HTML (`internal/futgg/embedded.go`, `self.__next_f.push`) → páginas de detalhe
enumeradas pelos sitemaps (`internal/futgg/sitemap.go`, comando `pages` —
uma requisição por carta e **sem preço**).

### Duas notas, dois domínios de uso — não misture

| nota | onde vive | quando usar |
|---|---|---|
| `analyze.Score()` | `internal/analyze/roles.go` | decidir **compra no mercado** (`FindUpgrades`, `FindEvolutions`) |
| GG Rating do fut.gg | vem na carta | comparar **dentro do seu elenco** (`FindSquadSwaps`, elo mais fraco, `internal/cards`) |
| overall da EA | — | nunca, para ranquear |

`Score()` é a opinião *deste* bot: pesos por função, bônus de PlayStyle
(PlayStyle+ vale dobrado, teto de 12), corte de ritmo, penalidade por jogar
fora de posição. Faz sentido no mercado, onde não existe número oficial para
comparar. Dentro do clube o fut.gg já publicou uma nota por carta — usar a
dele é mais direto e é a que o usuário confere no site. `roleTable` em
`roles.go` é o ponto de calibração; mexer ali muda todo o ranking, e os
testes de `internal/analyze` existem para travar esses invariantes.

Sugestões saem ordenadas por **ganho por moeda gasta**, não por ganho bruto.

### Camadas

- `internal/domain` — modelo puro, sem saber de fut.gg, banco ou relatório.
  Virar o ciclo do jogo não deve exigir mudança aqui.
- `internal/futgg` — cliente HTTP, cache, rate limit, decode e mapeamento.
- `internal/analyze` — motor de decisão.
- `internal/cards` — a análise "atual x potencial" carta a carta (evolução).
- `internal/report` — briefing HTML autocontido (`report.gohtml`, `//go:embed`);
  também dono das funções de resumo (`SquadSummary`, `MainSquad`,
  `RankChallenges`, `MarketRows`) que `internal/api` reusa.
- `internal/store` — interface `Store` (preços, clube, notícia vista, e o
  `Snapshot` do dia) com duas implementações (JSON e Postgres); trocar uma
  pela outra não muda uma linha do motor nem do relatório.
- `internal/scheduler` — só decide QUANDO rodar (`Next`, `ShouldRunNow`,
  puros e testáveis) e o laço que espera (`Scheduler.Run`); zero saber de
  fut.gg ou de análise.
- `internal/api` — a API JSON que a UI consome, um envelope por tela; lê
  `store.Store`, nunca a rede.
- `internal/webui` + `web/` — a UI React embutida no binário.

`web/vite.config.ts` grava o build **direto** em `internal/webui/dist`
(`build.outDir`) — não existe passo de cópia, e `dist/` não é versionado.

### Estado fica em `.eafc-bot/`, no diretório atual

Config, cache, histórico JSON, snapshots (`.eafc-bot/snapshots/<cycle>/`,
um arquivo por dia + `history.json` com o resumo leve, 30 dias de retenção
por padrão — `JSONStore.pruneSnapshots`) e relatórios, todos sob
`.eafc-bot/` no CWD — não em `$HOME`, não em `$TEMP` (ver o comentário de
`baseDir` em `internal/config/config.go`). Já está no `.gitignore`. É por
isso que o volume do Docker monta em cima desse caminho relativo em vez de
usar `EAFC_DATA_DIR` sozinho — essa variável só move o histórico, não o
cache nem o `config.json` (ver `Dockerfile`).

`config.Load` nunca falha por arquivo ausente: aplica os padrões, sobrepõe as
variáveis de ambiente (`EAFC_GAMERTAG`, `EAFC_DSN`, `EAFC_BUDGET`,
`EAFC_FUTGG_COOKIE`, `EAFC_CONFIG`, `EAFC_CYCLE`, `EAFC_DATA_DIR`) e valida.

Dois dados que a Community API da EA não expõe e por isso não chegam sozinhos:
**saldo de moedas** (preencha `market.extra_budget`) e a **escalação titular**
(deduzida do `isInActiveSquad` / rota `active-squad`).

## Convenções

**Comentário explica o porquê, não o quê** — e frequentemente registra o que
foi verificado ao vivo contra o site ("testado ao vivo, a resposta vem do
mesmo tamanho com ou sem o parâmetro"). Esse é o padrão do repo; mantenha-o
ao mexer em decode/mapeamento, porque é o que impede a próxima pessoa de
"consertar" um comportamento que já foi investigado.

**Nome de teste descreve o invariante, em português**:
`TestZagueiroLentoPerdeParaRapidoComOverallMenor`,
`TestPlayStylePlusDesempata`. Os testes de `internal/discover` sobem um site
falso com rotas de nomes inventados (`/v3/catalogue/item-index/`) e campos
abreviados (`ovr`, `pos`, `bin`) para provar que a classificação não depende
de pista de nome — que é exatamente a situação do FC 27.

**Mensagem de erro ensina o próximo passo**, com o comando literal a rodar.

## Armadilhas conhecidas

- **`SquadSlot` é lugar físico, não posição lógica.** Uma formação repete
  posição (dois CB, dois CM); iterar `Squad.Starters` compara os dois,
  `Club.Starter(pos)` devolve só o de menor overall — de propósito, para a
  pergunta "a evolução supera o titular?".
- **`eaId` ≠ `id` interno.** Dentro de `path`, nos caminhos de evolução, `id`
  é o id interno da carta e `eaId` é a identidade que o resto do bot usa;
  `evopaths.go` lê `eaId` direto, sem passar pelo alias genérico.
- **Posição chega em duas línguas**: rótulo (`"CM"`) na listagem de mercado,
  id numérico (`14`) no elenco do GG Club. `domain.ParsePosition` aceita as
  duas; a tabela `positionByID` tem uma irregularidade real (LWB é 8, não 6).
- **Goleiro reaproveita os seis campos de atributo de linha**
  (`pac`=DIV, `sho`=HAN, `pas`=KIC, `dri`=REF, `def`=SPD, `phy`=POS).
- **PlayStyles vêm como número no elenco** (`playstyles:[0,34,5]`); sem a
  tabela `eaId → nome` carregada antes da coleta paralela
  (`ensurePlayStyles`), o motor de pontuação, que compara por nome, não acha
  nada.
- **`spaHandler` serve `index.html` cru de propósito** — delegar ao
  `http.FileServer` devolve 301 para `./` e destrói a rota do React Router.
- **`domain.Player.slug` ≠ `cards.CardReport.Slug`.** O primeiro é o slug
  bruto do fut.gg (`FutGGSlug`, pode vir vazio no elenco); o segundo passa
  por `assignSlugs` e desambigua duas cartas com o mesmo eaId (ver o
  comentário lá). A rota `/api/time/{slug}` busca por `CardReport.Slug` —
  `internal/api.handleTime` cruza `snap.Cards` contra o elenco para expor o
  slug CERTO em `RosterCard.CardSlug` (vazio quando a carta está abaixo do
  `cards_min_rating` e não tem `CardReport`). Linkar direto com
  `player.slug` produz link quebrado ou pra carta errada.

## Agent skills

### Issue tracker

GitHub Issues in `Gscarneiro/eafc-bot`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at the repo root, created lazily. See `docs/agents/domain.md`.
