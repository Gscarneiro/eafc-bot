# Listagens uniformes: OData (subconjunto) em Go + React

## Contexto

Hoje cada tela inventou o próprio jeito de listar. O levantamento encontrou:

| Tela | Forma da lista | Filtro | Ordenação | Paginação | Onde executa |
|---|---|---|---|---|---|
| `/` Status | `.list-row` (4 feeds sem teto) | — | — | — | — |
| `/time` | `<table>` (`RosterTable`) | 3 campos, em `useState` (fora da URL) | fixa no servidor, sem UI | sim (impl. local) | servidor |
| `/mercado` | `.rank-row` | 2 selects, na URL | `<select>`, 3 opções | **não** | **cliente** |
| `/evolucoes` | `.rank-row` | 6 controles, na URL | cabeçalhos multi-critério | sim (2ª impl.) | servidor |
| `/capital/*` ×3 | `.rank-row` | 1 select, na URL | `<select>`, 2 opções | **não** | **cliente** |

Ou seja: 3 renderizações de lista diferentes, 2 implementações de paginação (e 4 telas sem
nenhuma), 3 UIs de ordenação distintas, estado de filtro dividido entre `useState` e
`useSearchParams`, e metade das telas filtrando no cliente — o que as impede de paginar.

Há também defeitos reais que caem junto nesse arrasto:

- `Investimentos.tsx:87-88` — as três seções compartilham `?filter=` e `?sort=`, então uma
  URL de `/capital/vendas` colada em `/capital/sbcs` produz lista vazia com "limpar filtros" aceso.
- `Evolucoes.tsx:149` e `Time.tsx:92` — busca sem debounce; em `/evolucoes` o `setParams`
  usa *push*, então cada tecla digitada vira uma entrada no histórico do navegador.
- `Mercado.css:1` — `.tx-card.unaffordable` ficou órfão quando a classe migrou para
  `.rank-row` (`Mercado.tsx:117`); linhas fora do orçamento **não são mais esmaecidas**, e o
  único sinal restante é um chip `.optional-metric`, escondido abaixo de 900px.
- `shared.css:296-346` — bloco `.tx-card`/`.tx-head`/`.tx-row` apontando para
  `components/TransactionCard.tsx`, arquivo que não existe mais.
- `main.tsx` — sem rota `path="*"`; URL desconhecida renderiza página em branco, sem nem o shell.
- `AGENTS.md:112` e `CLAUDE.md` citam `api.SquadCard`, que **não existe** (só
  `report.SquadCard`, sem tags).

O resultado pretendido: **uma gramática de consulta só**, falada igual pelo Go e pelo React,
com filtro/ordenação/paginação sempre no servidor e sempre refletidos na URL.

## Decisões

**1. Subconjunto de OData escrito à mão, biblioteca padrão apenas.** `go.mod` não tem um
único `require` e isso é decisão de projeto (`CLAUDE.md`). Toda lib OData de Go quebraria
isso, então o parser é nosso. Go 1.24 dá genéricos, `slices` e `cmp` — dá para fazer sem
reflexão em produção.

**2. Uma coleção = uma rota.** É o modelo de *entity set* do OData e resolve o problema das
rotas compostas (`/api/investimentos` devolve 3 listas, `/api/status` devolve 6): sem isso
seria preciso inventar namespace de parâmetro (`bench_*`, como hoje) ou implementar
`$expand` com opções aninhadas. Também casa com as telas — `/capital/investimentos`,
`/capital/vendas` e `/capital/sbcs` já são três telas que hoje baixam as três listas.

**3. Filtro/ordenação sempre no servidor, estado sempre na URL.** É o que torna a paginação
possível em `/mercado` e `/capital/*`, e o que torna qualquer tela um link compartilhável.

**4. Cache de snapshot no `api.Server`.** Consequência direta da decisão 2, e não opcional:
`Server.load` (`internal/api/api.go:182`) chama `Store.LatestSnapshot` a cada request, e as
duas implementações **re-leem e re-decodificam o arquivo inteiro do dia** — `os.ReadFile` +
`json.Unmarshal` sob mutex global no JSON (`store/json.go:356-381`), `SELECT payload` +
`Unmarshal` no Postgres (`store/postgres.go:315-330`). Os snapshots reais nesta máquina têm
**30-96 MB**. Quebrar a tela Hoje em 5 coleções multiplicaria isso por 5.

## Escopo

Uniformizar **todas as coleções** de todas as telas, inclusive os feeds do Status (que hoje
são `.map()` sem teto). Blocos escalares — KPIs, nota do elenco, `top_move`, os dois
`TrendChart`, o desenho do campo, o formulário de Configurações — ficam como estão: não são
coleção, e forçá-los a virar lista deixaria o contrato pior.

---

## Fase 1 — `internal/query` (pacote novo, stdlib only)

```
internal/query/
  query.go    Options, Order, Parse(url.Values) (Options, error)
  lex.go      tokenizador do $filter
  parse.go    descida recursiva -> Expr
  expr.go     nós da AST + Eval(row) bool
  value.go    Value (união tagueada) + Kind + fold de acentos
  schema.go   Schema[T], Field[T], NewSchema
  apply.go    Apply[T](Schema[T], []T, Options) (Page[T], error)
  errors.go   *query.Error -> 400 que ensina o próximo passo
```

### Gramática suportada

```
$filter   := or
or        := and ( "or" and )*
and       := unary ( "and" unary )*
unary     := "not" unary | primary
primary   := "(" or ")" | fn | compare
compare   := ident op literal          -- só campo-vs-literal
op        := eq | ne | gt | ge | lt | le | in
fn        := ( contains | startswith | endswith ) "(" ident "," literal ")"
ident     := nome ( "/" nome )*        -- ex.: candidate/rating
literal   := 'texto' | número | true | false | null | 2026-08-24T00:00:00Z

$orderby  := ident [asc|desc] ( "," ident [asc|desc] )*
$top $skip $count $search
```

Deliberadamente **fora**: campo-vs-campo, aritmética, lambdas `any`/`all`, `$select`,
`$expand`, `$apply`. Cada um desses produz erro nomeando o que não é suportado — o repo já
segue "mensagem de erro ensina o próximo passo", e "na dúvida, não afirma" vale aqui:
melhor recusar do que fingir que entendeu.

### Schema — registro explícito, sem reflexão

Reflexão sobre tags `json` resolveria tudo genericamente, mas embutir structs inteiras
(`analyze.Upgrade` carrega um `domain.Player` e um `domain.ClubPlayer` completos) tornaria o
custo por linha alto e — pior — exporia campo que não deveria ser consultável. Registro
explícito dá whitelist, tipo em tempo de compilação e a lista de campos válidos para a
mensagem de erro:

```go
type Field[T any] struct {
	Name   string       // nome no $filter/$orderby — igual à tag json da linha
	Kind   Kind         // String, Int, Float, Bool, Time
	Get    func(T) Value
	Facet  bool         // entra em @eafc.facets
	Search bool         // entra no $search (só Kind String)
	Desc   bool         // direção padrão quando o $orderby não diz
}

type Schema[T any] struct {
	Name    string      // "mercado", "evolucoes" — só para mensagem de erro
	Default string      // $orderby aplicado quando o cliente não manda
	MaxTop  int
	// ...
}
func NewSchema[T any](name, def string, maxTop int, fields ...Field[T]) Schema[T]
```

`Value` é união tagueada (`Kind` + `S`/`N`/`B`/`T`), não `any` — sem boxing por comparação.

### Pipeline de `Apply`

1. Valida os campos citados em `$filter`/`$orderby` contra o schema → `*query.Error`
   listando os nomes válidos.
2. **Facetas sobre o conjunto NÃO filtrado** — preserva a semântica travada por
   `TestHandleEvolucoesBuscaSemResultadoPreservaFacetas` (`api_test.go:414`): o select de
   posição não pode esvaziar quando a busca não acha nada.
3. `$search` (OR sobre os campos `Search:true`) e depois `$filter`.
4. `Count` = tamanho do conjunto filtrado — **antes** de `$top`/`$skip`. Resumos por rota
   também são calculados aqui, não sobre a página visível (semântica atual, `api.go:885-901`).
5. `slices.SortStableFunc` com a cadeia de `$orderby`, desempate pelo `Default` do schema.
6. Clamp de `$skip`/`$top` e fatia.

Comparação de texto é **insensível a caixa e a acento** (`query.fold`, ~15 linhas cobrindo
`áàâãéêíóôõúüç`): sem `golang.org/x/text`, e "evolucao" precisa achar "evolução".

### Envelope

```go
type Page[T any] struct {
	Value  []T                `json:"value"`
	Count  int                `json:"@odata.count"`
	Skip   int                `json:"@eafc.skip"`
	Top    int                `json:"@eafc.top"`
	Facets map[string][]Facet `json:"@eafc.facets,omitempty"`
}
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}
```

`value` e `@odata.count` são OData de verdade; `@eafc.*` usa a convenção OData de anotação
customizada. Cada rota embute `query.Page[T]` e acrescenta as suas anotações (funil, resumo,
série de preço).

**Reaproveitar em vez de reescrever:** `parseEvoSort` (`api.go:695-730`) é o protótipo do
parser de `$orderby` — alias, direção padrão por campo, dedupe, teto de 4 critérios. Os
comparadores `compareFloat`/`compareInt`/`compareBool` (`api.go:763-793`) são
type-agnósticos e viram os comparadores de `Kind`, ou dão lugar a `cmp.Compare`. ⚠️
`internal/api/api.go:733` e `:878` usam `cmp` como **nome de variável local** — importar o
pacote `cmp` ali exige renomear.

### Testes (nomes em português, descrevendo o invariante)

`TestFiltroCombinaAndOrComParenteses`, `TestCampoDesconhecidoListaOsValidos`,
`TestBuscaIgnoraAcentoECaixa`, `TestFacetasNaoSomemQuandoOFiltroZeraALista`,
`TestOrderByMultiplosCriteriosRespeitaAPrioridade`, `TestTopAcimaDoTetoEhCortado`,
`TestSkipAlemDoFimDevolveListaVaziaNaoErro`, `TestContainsNaoAceitaCampoNoSegundoArgumento`.

---

## Fase 2 — `internal/api`: schemas, rotas, cache

### `internal/api/listing.go`

```go
// serveList aplica a consulta e responde 400 com a lista de campos válidos
// quando o cliente pede o que não existe. Devolve a página para o handler
// pendurar as anotações da rota.
func serveList[T any](w http.ResponseWriter, r *http.Request, sc query.Schema[T], items []T) (query.Page[T], bool)
```

Absorve a matemática de clamp de página hoje **duplicada literalmente** em
`api.go:385-398` e `api.go:902-919`.

### `internal/api/schemas.go` — um schema por coleção

Nomes de campo iguais às tags `json` da linha, com `/` para caminho aninhado
(`candidate/rating`, `player/common_name`) — assim `$orderby=efficiency desc` casa com o
campo que a linha renderiza, sem tabela de tradução.

### `internal/api/cache.go`

```go
// snapCache evita re-decodificar o snapshot do dia (30-96 MB nos arquivos
// reais) a cada request. Invalida por TTL curto e quando Status().LastSuccess
// muda — o job acabou de gravar um snapshot novo.
```

Sem plumbing novo: `Server.Status func() JobStatus` (`api.go:66`) já entrega `LastSuccess`.
Em `serve -demo` o `Status` devolve zero (`serve.go:203`) e o cache nunca invalida, que é o
comportamento certo para dado estático. `CacheTTL` zero desliga o cache (testes).

### Tabela de rotas final

**Coleções** (OData completo em todas):

| Rota | Elemento | Origem | Anotações |
|---|---|---|---|
| `GET /api/mercado` | `analyze.Upgrade` | `snap.Upgrades` | `@eafc.funnel`, `@eafc.price_series` |
| `GET /api/evolucoes` | `EvoMatchView` | `confirmedEvoViews` | `@eafc.summary` |
| `GET /api/elenco/titulares` | `StarterCard` | `report.MainSquad` | `@eafc.formation` |
| `GET /api/elenco/reservas` | `RosterCard` | banco | — |
| `GET /api/capital/investimentos` | `analyze.Investment` | `FindInvestments` | `@eafc.funnel` |
| `GET /api/capital/vendas` | `analyze.SellCandidate` | `FindSellCandidates` | `@eafc.funnel` |
| `GET /api/capital/sbcs` | `analyze.FodderSignal` | `FindFodderDemand` | — |
| `GET /api/hoje/novidades` | `domain.Player` | `snap.NewCards` | — |
| `GET /api/hoje/noticias` | `domain.NewsItem` | `snap.FreshNews` | — |
| `GET /api/hoje/sbcs` | `domain.SBC` | `snap.SBCs` (via `report.RankChallenges`) | — |
| `GET /api/hoje/objetivos` | `domain.Objective` | `snap.Objectives` | — |
| `GET /api/hoje/movimentacao` | `MovimentoCard` (novo) | `snap.Diff.Added/Removed` | — |
| `GET /api/historico` | `store.SnapshotSummary` | `SnapshotHistory` | — |

`MovimentoCard` é `RosterCard` + `Movimento string` (`"entrou"`/`"saiu"`) — unifica as duas
listas de `ClubDiff` numa coleção filtrável, e de quebra dá ordem determinística ao que hoje
sai em ordem de iteração de mapa.

**Escalares** (sem coleção): `GET /api/status` (KPIs, `top_move`, `errors`), `GET /api/time`
(formação + `optimization`), `GET /api/time/{slug}`, `/api/job`, `/api/config`,
`/api/evolucoes/favoritos`.

⚠️ `/api/hoje/sbcs` (SBCs que valem/expiram) e `/api/capital/sbcs` (demanda de fodder) são
coleções diferentes com nome parecido — espelham as duas telas, mas o comentário de cabeçalho
de cada schema precisa dizer isso.

Conflito de rota: Go 1.22+ prefere padrão literal a wildcard, então `/api/evolucoes/favoritos`
continua ganhando de `/api/evolucoes`. `report.RankChallenges` (`report/report.go:245`)
**muta a fatia `objs` no lugar** — ao reusá-la sob cache de snapshot, copiar antes.

### Testes

Um por coleção, no padrão de `api_test.go` (servidor real via `store.NewJSON(t.TempDir())`,
sempre através de `srv.Handler()`). Mais um teste de contrato barato: um teste que usa
reflexão **só no teste** para provar que todo `Field.Name` de cada schema corresponde a uma
tag `json` real do tipo da linha — é o que pega a deriva quando uma tag mudar.

---

## Fase 3 — `web/src/odata.ts` + `useCollection`

`odata.ts` espelha a gramática do Go: `Filter` como união discriminada, `formatFilter`,
`parseFilter`, `toSearchParams`, `fromSearchParams`.

`useCollection<T>(path, { defaultOrderBy, pageSize })`:
- amarra a consulta a `useSearchParams` — `replace` ao digitar, `push` nas demais mudanças
  (corrige o spam de histórico de `Evolucoes.tsx:149`);
- debounce de 300 ms no `$search` (hoje **cada tecla dispara request** em `/time` e `/evolucoes`);
- mantém os dados anteriores enquanto recarrega — hoje `useData.ts:25` liga `loading` a cada
  mudança de dependência e a tela inteira pisca esqueleto ao trocar de página;
- devolve `{ rows, count, page, pages, facets, query, setFilter, setOrder, setPage, clear }`.

`useData` (`web/src/useData.ts`) continua para as rotas escalares. `api.ts` ganha um
`fetchCollection<T>(path, params)` genérico; `fetchEvolucoes(query: string)` (`api.ts:46`,
que hoje recebe query string crua) some.

`types.ts` ganha `ODataPage<T>` e as anotações por rota. O arquivo é espelho manual das tags
Go (dito no próprio cabeçalho, `types.ts:1-9`) — a convenção `T[] | null` para fatia sem
`omitempty` continua valendo.

---

## Fase 4 — componentes compartilhados

| Componente | Substitui |
|---|---|
| `DataList.tsx` (colunas declarativas + cabeçalho ordenável + expansão) | `RosterTable` (`Time.tsx:116`) e as 5 cópias de `.rank-row` (`Mercado.tsx:117`, `Evolucoes.tsx:87`, `Investimentos.tsx:133/156/179`) |
| `Pagination.tsx` | `Time.tsx:104` e `Evolucoes.tsx:166` (duas marcações e dois CSS: `.pagination` e `.evo-pagination`) |
| `SearchInput.tsx` (debounced) | `Time.tsx:92` e `Evolucoes.tsx:149` |
| `SortHeader` (dentro de `DataList`) | `SortColumn` local de `Evolucoes.tsx:58` — hoje a **única** tela com ordenação multi-critério |
| `RankingControls` estendido | recebe as facetas do servidor e mostra a contagem por opção; mantém o nome e `.ranking-controls` para não mexer no CSS |

`DataList` preserva o que já funciona: `.optional-metric` nos breakpoints 900/620px
(`shared.css`), `role="columnheader"` + `aria-sort` + rótulo de prioridade do `SortColumn`,
e o padrão `?open=` de expansão.

---

## Fase 5 — telas

| Tela | Mudança |
|---|---|
| `/` Status | KPIs e os 2 `TrendChart` intactos. Os 4 feeds viram `useCollection(..., {top:5})` com "+N mais" — hoje são `.map()` sem teto |
| `/time` | `titulares` e `reservas` viram duas coleções; vista "tabela" ganha cabeçalho ordenável; filtros migram de `useState` para a URL; vista de campo e toggle `localStorage` intactos |
| `/mercado` | `useCollection` — **ganha paginação**; some o `useMemo` de filtro+ordenação (`Mercado.tsx:78-85`); mantém o `MercadoEmpty` que narra o funil |
| `/evolucoes` | Maior simplificação: `parseSortSpec`/`toggleSortCriterion`/`SortColumn`/paginação inline saem para o código compartilhado; pílulas de obtenção viram facetas do servidor |
| `/capital/*` ×3 | Uma coleção por seção — **corrige** o `?filter=`/`?sort=` compartilhado (`Investimentos.tsx:87-88`); ganham paginação |
| `/time/:slug`, `/configuracoes` | Só os tipos |

Junto, porque cai no mesmo arrasto: rota `path="*"` em `main.tsx` (hoje URL desconhecida =
página branca), `.unaffordable` reapontado para `.rank-row` em `Mercado.css`, e remoção do
bloco morto `.tx-card` (`shared.css:296-346`) e dos seletores órfãos (`.spark`,
`.evo-objectives`, `.swap .sub`, `.req-line`).

---

## Fase 6 — documentação

`CLAUDE.md` e `AGENTS.md`: seção sobre `internal/query` como camada única de listagem, a
tabela de rotas nova, e a correção da deriva `api.SquadCard` (não existe; são
`api.RosterCard`/`api.StarterCard`). `web/AGENTS.md`: usar `DataList`/`Pagination`/
`useCollection` antes de criar componente ou CSS local.

---

## Verificação

```bash
gofmt -l ./cmd ./internal; go vet ./...; go test ./... -count=1
cd web && npm install && npm run build && npm test && cd ..
go build ./cmd/eafcbot
```

**Frontend hoje não tem teste nenhum** — sem Vitest, Jest, ESLint ou qualquer `*.test.*`.
Adicionar Vitest (devDependency de `web/`, que não é afetado pela regra stdlib-only do
`go.mod`) e cobrir `odata.ts`: ida e volta `formatFilter`/`parseFilter`, e um teste que
monta a mesma consulta nos dois lados e confere que o Go aceita a string que o TS gera — é
onde a gramática duplicada pode divergir em silêncio.

Manual, com o vetor que o repo já oferece:

```bash
./eafcbot serve -demo    # as 7 telas sem rede
```

Percorrer cada tela em largura de desktop e de celular, conferindo: filtro reflete na URL,
F5 restaura o estado, ordenação multi-critério mostra prioridade, paginação bate com
`@odata.count`, facetas não somem quando a busca zera a lista, e `$filter` inválido devolve
400 com os campos válidos. ⚠️ Verificar antes se o snapshot de demo tem linhas suficientes
para passar de uma página — se não tiver, ampliar `demoSnapshot` em `cmd/eafcbot`.

## Riscos

- **Quebra de contrato deliberada.** As rotas compostas somem e os envelopes mudam. É
  seguro porque a UI é embutida no binário via `//go:embed` — as duas pontas versionam juntas
  e não há cliente externo. Mas `types.ts` e as 7 telas têm que virar no mesmo commit.
- **Gramática escrita duas vezes** (Go e TypeScript). É o custo de não ter codegen; mitigado
  pelo teste de ida e volta acima.
- **Cache de snapshot** introduz janela de dado velho de alguns segundos após o job. O
  gatilho por `Status().LastSuccess` fecha o caso comum; o TTL cobre o resto.
- Volume: `internal/query` é a maior peça nova (~800-1200 linhas com testes). As fases 1 e 2
  são verificáveis isoladamente antes de qualquer tela mudar.
