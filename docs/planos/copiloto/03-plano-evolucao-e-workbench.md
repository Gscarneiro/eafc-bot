# Fase 03 — plano de Evolução e Workbench

## Estado da implementação

Concluída nesta entrega, em duas partes.

### Parte 1 — grafo, porta e Gate

- `domain.EvolutionGraph`/`EvolutionNode`/`EvolutionTransition`
  (`internal/domain/evolution_graph.go`) modelam a evolução como grafo:
  nós são estados de carta, transições levam de um estado a outro com
  custo, `IsExpired`, `TrainingTime` e metadado `Repeatable`/`Lab`.
  `EvolutionGraph.LinearPaths()` projeta o grafo de volta pra
  `[]domain.EvolutionPath` — a visão linear de hoje continua sendo uma
  PROJEÇÃO, não foi substituída; `cards.bestPaths`, `analyze.FindEvolutions`
  e `/api/evolucoes` não mudaram uma linha nesta entrega.
- Achado que corrigiu o desenho inicial, só descoberto ao rodar os testes
  contra o fixture real (`internal/futgg/evopaths_test.go`): dentro de UM
  caminho de evolução, o `eaId` (`Player.ID`) não muda do início ao fim —
  é a mesma cópia de clube evoluindo, nunca um item novo. Um design
  anterior que usava `Card.ID` como identidade de nó colapsava toda
  transição num self-loop e falhava `Validate()` contra dado real. Por isso
  `EvolutionNode.ID` é sintético (string, atribuído por quem constrói o
  grafo), independente de `Card.ID` — registrado aqui para a parte 2 não
  reintroduzir o mesmo bug.
- `EvolutionGraph.Validate()` é o fail-closed: `RootID` precisa existir,
  toda transição precisa referenciar nó existente, nenhum ciclo é
  alcançável a partir da raiz (self-loop incluso) e nenhum nó pode
  pertencer a um ciclo de jogo (`Player.Cycle`) diferente do grafo.
  `futgg.Client.EvolutionGraph` (`internal/futgg/evolution_graph.go`) chama
  isso na fronteira, depois de montar o grafo — zero decode novo, reaproveita
  `EvolutionPaths` já existente.
- `domain.LinearGraph(current, paths)` converte o retorno confirmado de
  hoje num grafo: como a API só confirma custo/prazo AGREGADO por cadeia
  inteira (nunca por evolução individual dentro dela — confirmado no
  fixture, um `coinsCost`/`readableTrainingTime` só por `path`), cada
  `EvolutionPath` retornado vira UMA transição raiz→final carregando esse
  agregado, com o `EvolutionPath` original preservado em
  `EvolutionTransition.Source` para fidelidade completa — nenhum nó ou
  custo intermediário é inventado (CLAUDE.md: na dúvida, não afirma). A
  raiz do grafo é sempre a carta REAL do elenco (`current`, com GG Rating),
  nunca `path.Steps[0]` — que a API sempre devolve sem nota (mesma
  armadilha que `cards.bestPaths` já contornava).
- `cards.EvolutionGraphSource` (`internal/cards/evolution_source.go`) é a
  porta pequena pedida pelo contrato, no mesmo padrão de
  `internal/discover.Fetcher`: interface dona de quem consome, satisfeita
  estruturalmente por `*futgg.Client` (`var _ EvolutionGraphSource =
  (*futgg.Client)(nil)`). O adapter de fixture fica só em
  `internal/cards/evolution_source_test.go`, não exportado — nada em
  produção consome a porta ainda nesta entrega, então promover o fixture
  pra tipo exportado seria adivinhar uma forma que nem a parte 2 nem
  `serve -demo` definiram.
- Gate coberto por teste: branch e rejoin
  (`TestEvolutionGraphIsBranchQuandoNoTemMaisDeUmaTransicaoDeSaida`,
  `TestEvolutionGraphIsRejoinQuandoDoisCaminhosConvergemNoMesmoNo`), Lab
  (`TestEvolutionGraphTransicaoLabNaoAlteraMecanicaDeTravessia`), repetição
  sem ciclo literal (`TestEvolutionGraphRepeticaoGeraNoNovoNuncaCicloLiteral`),
  expiração propagada pro caminho inteiro
  (`TestLinearPathsSomaCustoEExpiraSeQualquerTrechoDoRamoExpirar`),
  progresso parcial via `RootID` repontado
  (`TestLinearPathsProgressoParcialRepontaRootParaNoIntermediario`) e
  payload desconhecido fail-closed — ciclo, aresta pendurada, raiz
  inexistente e ciclo de jogo misturado, cada um com seu próprio teste em
  `internal/domain/evolution_graph_test.go`. Equivalência comportamental
  com o código de produção existente provada em
  `internal/futgg/evolution_graph_test.go`
  (`TestEvolutionGraphEquivaleAEvolutionPathsParaMesmoPayload`).

### Parte 2 — unificação, progresso, `/plano` e Workbench

- **Decisão de escopo confirmada com o usuário**: a unificação
  estimativa×confirmado fica só no novo `/api/evolucoes/{slug}/plano` (uma
  carta). A lista `/api/evolucoes` continua confirmado-only, exatamente como
  `TestHandleEvolucoesNaoDependeDaEstimativaDoAnalyze` já travava — mudar a
  lista reverteria uma decisão de design deliberada e testada; numa carta só,
  mostrar "elegível pelas regras, sem caminho confirmado" é sinal, não ruído
  espalhado pelo roster inteiro.
- `cards.CardReport.Graph` (`internal/cards/report.go`) guarda o grafo
  confirmado, preenchido durante o job (`BuildReports` agora chama
  `client.EvolutionGraph` em vez de `client.EvolutionPaths` direto — mesma
  UMA requisição de rede por carta, já validada por `Validate()`). Best/
  Alternates continuam a visão filtrada por ganho; Graph é a estrutura
  completa, sem esse filtro — é o que o Workbench lê.
  `TestBuildReportsPreenchaGraphQuandoPathConfirmado` prova o preenchimento;
  `TestBuildReportsPreservaFalhaDePathComoEstado` (já existia) continua
  passando sem mudança — falha de rede OU o fail-closed de `Validate()` viram
  o mesmo `EvolutionFetchError` de sempre.
- `internal/api/evolution_plan.go` (`GET /api/evolucoes/{slug}/plano`) reusa
  `cards.BuildCatalog`/`FindCatalog` (o mesmo lookup de `/api/time/{slug}`)
  e compõe `EvolutionPlanResponse{Status, Graph, EstimatedOnly, Completed}`.
  A composição (que precisa de `analyze.Eligible`/`EvolutionAcquisition`)
  mora em `internal/api`, não em `internal/cards`: `internal/analyze` já
  importa `internal/cards` (`sell.go`), então o inverso seria um ciclo de
  import — achado só durante esta entrega, documentado aqui para não
  ninguém tentar mover essa lógica pra dentro de `cards` de novo.
  `Status` reusa `cards.EvolutionStatus` — é literalmente o vocabulário que
  o contrato desta fase pede ("confirmado, sem caminho, inelegível, erro de
  coleta, não verificado"), sem precisar de um enum paralelo.
- Progresso/decisão manual: `config.Config.Serve.EvolutionProgress
  map[string][]string` (por slug de carta, nomes de evolução marcados como
  concluídos) + `Config.SaveEvolutionProgress` — mesma classe de dado que
  `EvolutionFavorites` (preferência durável fora de `store.Snapshot`), mas
  fora de `UISettingsServe`/`Editable`/`ApplyEditable` de propósito: não é
  um bloco de formulário, é um mapa que cresce um slug de cada vez. Rota
  `PUT /api/evolucoes/{slug}/progresso`, guardada pelo `guardLocalWrite` já
  existente. **Não reposiciona `EvolutionGraph.RootID` automaticamente** —
  IDs de nó são sintéticos e recalculados a cada `LinearGraph`, não são
  estáveis entre coletas; `Completed` fica só como anotação que o Workbench
  cruza por NOME de evolução, nunca aplicado na conta EA.
- Workbench: `EvolutionWorkbench`/`TransitionCard`
  (`web/src/pages/CardDetail.tsx`), dentro da seção "Evolução" já existente,
  abaixo do bloco Best/Alternates (que continua a visão "recomendado"
  inalterada). Mostra cada transição do grafo com badges de
  ramificação/reencontro/Lab/repetível/expirada (calculados no cliente, o
  JSON só traz nós+arestas crus), custo, prazo e um checkbox "já concluí"
  ligado ao progresso; lista separada para `estimated_only`, nunca misturada
  com o grafo confirmado.
- **Verificação de UI**: sem `chromium-cli`/Playwright disponíveis neste
  ambiente (repo não tem harness de e2e — confirmado antes de começar).
  Verificado em vez disso contra o binário real (`go build` +
  `./eafcbot serve -demo`): `tsc`/`vite build` sem erro, o bundle servido
  contém o texto novo ("todos os ramos"), `GET/PUT /api/evolucoes/{slug}/plano`
  e `.../progresso` respondem e persistem de verdade via HTTP contra o
  servidor rodando. **Gap honesto**: os `CardReport` do modo demo
  (`cmd/eafcbot/demo.go`, Osimhen/Rodri) são montados à mão e não têm
  `Graph` nem `snap.Evolutions` preenchidos — então o Workbench renderiza
  vazio em `serve -demo` hoje (comportamento correto dado o fixture, não um
  bug: confirmado via `/api/evolucoes/osimhen-88/plano` devolvendo
  `{"status":""}` sem grafo). Estender o fixture de demo pra exercitar
  branch/rejoin visualmente fica pra quando alguém precisar disso de
  verdade — não fazia parte desta entrega.

## Contratos

Modelar a evolução como grafo versionado por ciclo: nós, transições, custos, prazos, branch, rejoin, repetição, Lab, progresso e estado da cópia. A visão linear atual continua como projeção compatível.

Estados de domínio são separados da procedência: confirmado, sem caminho, inelegível, erro de coleta e não verificado.

## Implementação

- Criar uma porta pequena para a fonte de paths, com adapter de produção FUT.GG e adapter de fixture; o planner recebe dados normalizados e não faz HTTP.
- Unificar elegibilidade estimada e path confirmado, mostrando qual fonte sustentou cada afirmação.
- Persistir somente progresso/decisões manuais; não aplicar evolução na conta EA.
- Entregar `/api/evolucoes/{slug}/plano` e Workbench com comparação de ramos, consequências, custos e prazos.
- Modelar fixtures linear e ramificada antes de aceitar payload FC 27 em produção.

## Gate

Falha de path nunca vira venda; fixtures cobrem branch/rejoin/Lab, repetição, expiração, progresso parcial e payload desconhecido fail-closed.

