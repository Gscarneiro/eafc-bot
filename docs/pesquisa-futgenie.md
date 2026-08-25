# FutGenie por dentro: pesquisa técnica e roadmap seguro para o eafc-bot

**Data de corte:** 25 de agosto de 2026  
**Escopo:** produto FutGenie, notas, solver de SBC, extensão Chrome, conexão com a EA, Gauntlet, recursos adjacentes e oportunidades para o `eafc-bot`  
**Objetivo:** estudo pessoal e implementação clean-room, sem automação da conta EA nem vantagem operacional indevida

## Conclusão executiva

Vale implementar algumas ideias do FutGenie, mas não sua camada de execução dentro da conta EA.

O produto combina três coisas diferentes:

1. **Dados e inteligência:** catálogo, preços, meta rating, análise do clube, valor de fodder e evolução.
2. **Planejamento:** solução de SBC, montagem de elenco, Gauntlet, preview e proteção de cartas.
3. **Execução no Web App:** submeter SBC, comprar, dar lance, listar, abrir packs, fazer quicksell e aplicar elencos/evoluções.

As duas primeiras camadas são bons alvos para o `eafc-bot`. A terceira deve continuar fora do projeto. O motivo não é comercial: mesmo em estudo pessoal, a [EA proíbe processos automatizados, scraping e modificações não autorizadas de seus serviços](https://www.ea.com/commitments/positive-play/charter), suas [regras específicas do FC orientam a evitar extensões e vedam compradores automáticos](https://help.ea.com/pt-BR/articles/ea-sports-fc/fc-rules/), e o próprio [FutGenie reconhece que seu uso pode contrariar os termos da EA e causar perda da conta ou de ativos](https://www.futgenie.gg/terms-of-service).

O melhor caminho é construir equivalência **funcional e explicável**, não copiar o produto:

- parametrizar e aprofundar o Gauntlet que já existe;
- criar um solver local e consultivo de SBC, com preview e prova de requisitos;
- tornar a nota própria do bot explicável, versionada e separada do GG Rating;
- adicionar Club Insights, Fodder Value e histórico de coleção a partir dos snapshots;
- se for necessária outra fonte de clube, usar importação manual ou uma interface documentada de parceiro aprovado, nunca senha, cookie ou sessão EA.

## Como a pesquisa foi conduzida

Foram cruzadas páginas e políticas do FutGenie, a Chrome Web Store, documentação oficial do Chrome, termos e páginas de produto da EA, documentação do GG Rating do FUT.GG e o código atual do repositório. As páginas voláteis foram verificadas na data de corte.

Também foi feita uma inspeção estática limitada do [pacote oficial da extensão v3.0.1](https://chromewebstore.google.com/detail/futgenie-extension/olhalnjomgocehnhjpdemckdmeccnnfj) para confirmar apenas a arquitetura de integração. O artefato examinado tinha SHA-256 `5CF53CDBA8E447C13E32D1122C9D58129B698C01BB4091A76FB5A2813C877BF6`. Não houve login EA, execução autenticada, captura dinâmica de rede, acesso ao backend nem reprodução de código/algoritmo proprietário. Como os [termos do FutGenie proíbem engenharia reversa e cópia do serviço](https://www.futgenie.gg/terms-of-service), o roadmap não depende dessa inspeção e propõe somente uma implementação clean-room derivada do comportamento público.

O texto usa três níveis de certeza:

- **Comprovado:** aparece em fonte primária, metadado público ou código local.
- **Inferência:** é a explicação técnica mais compatível com os fatos, mas não foi publicada pelo FutGenie.
- **Desconhecido:** não há informação pública suficiente; qualquer resposta mais específica seria palpite.

## 1. O que é o FutGenie no seu conjunto

O [site oficial](https://www.futgenie.gg/) apresenta o FutGenie como companion não oficial de Ultimate Team. O produto possui:

- um site público com jogadores, preços, SBCs, objetivos e packs;
- uma extensão que adiciona controles ao EA FC Web App;
- apps iOS e Android;
- conta própria, limites gratuitos e assinatura;
- serviços remotos para identidade FutGenie, preferências, preços, solver e telemetria.

A extensão Chrome estava na versão 3.0.1, atualizada em 11 de agosto de 2026, e a loja informava 200 mil usuários na data de corte. A descrição promete auto-complete de SBC, overlays de preço e proteções contra consumir itens valiosos. Esses números e textos são declarações da [Chrome Web Store](https://chromewebstore.google.com/detail/futgenie-extension/olhalnjomgocehnhjpdemckdmeccnnfj), não auditoria independente de qualidade ou segurança.

### Modelo funcional

| Camada | Responsabilidade observada | Estado da conta EA |
|---|---|---|
| Catálogo público | jogadores, preços, histórico, SBCs, objetivos, packs | não precisa alterar a conta |
| Inteligência FutGenie | Genie Grade, solver, regras, sugestões, previews | pode trabalhar sobre uma cópia do estado |
| Interface injetada | botões, overlays, seleção e atalhos dentro do Web App | lê a tela/estado autenticado |
| Executor | submeter, comprar, listar, abrir, descartar, criar/aplicar | altera a conta e os ativos |
| Conta/backend FutGenie | assinatura, persona, preferências, limites e telemetria | associa uso à persona |

Esse recorte é importante: a maior parte do valor percebido vem da inteligência e do preview, mas a maior parte do risco vem do executor.

## 2. Como a extensão provavelmente funciona

O tutorial oficial instrui o usuário a instalar a extensão, abrir o **Web App oficial da EA**, autenticar-se normalmente na EA e depois entrar separadamente no FutGenie dentro da interface adicionada. Portanto, não há evidência de que a extensão receba a senha EA. A [política de privacidade do FutGenie](https://www.futgenie.gg/privacy-policy) afirma expressamente que não coleta nem armazena senha ou credenciais de login EA, embora processe persona ID e telemetria de SBC, packs e mercado.

O manifest do pacote examinado, também refletido pelo [Chrome-Stats](https://chrome-stats.com/d/olhalnjomgocehnhjpdemckdmeccnnfj), é Manifest V3 e declara `activeTab`, `scripting`, `storage`, acesso a hosts EA/FutGenie e content scripts em `ea.com`. A permissão `cookies` não aparece. O código empacotado injeta um script no mundo `MAIN`, envolve `fetch`/XHR para observar o tráfego do próprio Web App e acessa serviços internos expostos pela aplicação. Isso confirma o acoplamento à sessão/página; não valida o comportamento do backend nem prova que todo caminho encontrado está ativo em produção.

A própria plataforma Chrome explica que [content scripts podem ler e modificar o DOM](https://developer.chrome.com/docs/extensions/reference/manifest/content-scripts), que a [Scripting API injeta código](https://developer.chrome.com/docs/extensions/reference/api/scripting) e que a [Storage API persiste estado da extensão](https://developer.chrome.com/docs/extensions/reference/api/storage). A inspeção mostrou tokens **do FutGenie** e configurações em `chrome.storage.local`; isso não demonstra armazenamento de token EA. O Chrome recomenda `storage.session` para dados sensíveis e informa que `storage.local` persiste em disco e é acessível a content scripts por padrão, salvo mudança explícita de nível de acesso.

Com isso, o desenho mais provável é:

```text
                         autenticação normal
Usuário ───────────────────────────────────────► accounts.ea.com
                                                        │
                                                        ▼
                                              EA FC Web App autenticado
                                                        ▲
                                                        │ UI/lógica injetada
                                                        ▼
Usuário ◄──────────────► extensão FutGenie ◄────────────┘
                              │
                              ├── conta/licença/preferências FutGenie
                              ├── catálogo, preços e Genie Grade
                              └── geração de soluções e telemetria
                                      serviços FutGenie
```

### O que é fato e o que é inferência

**Comprovado:**

- o login EA acontece no domínio oficial;
- existe um login FutGenie separado;
- a extensão aparece dentro do Web App;
- o pacote injeta no mundo principal, observa `fetch`/XHR e acessa serviços internos da aplicação;
- ela consegue preencher/submeter SBCs e executar ações de mercado/packs;
- a política diz guardar persona ID e atividade iniciada pelo serviço;
- a loja declara que a extensão trata PII, informações de autenticação, localização, atividade e conteúdo de site.

**Inferência de alta confiança:**

- a extensão usa o Web App já autenticado como ambiente de execução;
- preços, solver, assinatura e algumas preferências vêm de serviços do FutGenie;
- ações mutáveis são disparadas no contexto da sessão já aberta no Web App.

**Desconhecido:**

- o comportamento efetivo de cada caminho numa sessão real;
- o formato das mensagens internas;
- quais dados autenticados chegam ao backend e por quanto tempo são retidos além do declarado na política;
- quanto do solver roda localmente ou no servidor;
- a arquitetura exata do app móvel.

Não há base para afirmar que o FutGenie “rouba senha”; a inspeção não encontrou permissão `cookies`, leitura explícita de `document.cookie` ou nomes conhecidos de headers de sessão EA. Isso é evidência limitada ao cliente examinado, não prova absoluta sobre todo o sistema. Também não há base para dizer que toda a integração é uma API pública: a declaração da loja parece plausível para catálogo e preço, mas é ampla demais para explicar ações autenticadas. A própria política reconhece processamento de persona e atividade da conta.

## 3. A conexão oficial da EA é outra coisa

Em 27 de julho de 2026, a EA anunciou a **Community API**. No lançamento, somente **FUT.GG, FUTBIN e FUTWIZ** foram autorizados. O fluxo usa login e consentimento hospedados pela EA; o parceiro recebe permissões específicas, não a senha, e pode recuperar dados como atletas, formações e táticas. A EA orienta a não confiar em outro site que alegue oferecer o mesmo fluxo sem aprovação. [Anúncio da Community API](https://www.ea.com/pt-br/games/ea-sports-fc/fc-26/news/pitch-notes-fc26-community-api-update) e [artigo da EA Help](https://help.ea.com/pt-br/articles/ea-sports-fc/community-api/).

FutGenie não estava entre os parceiros aprovados na data de corte. Logo:

- não há evidência de que a extensão use a Community API oficial;
- entrar no Web App oficial e atuar sobre sua sessão não é o mesmo que uma autorização OAuth de parceiro;
- a Community API anunciada descreve **recuperação de dados**, não compra, venda ou submissão automatizada;
- o `eafc-bot` não deve implementar um login EA próprio.

O fato de FUT.GG ser aprovado também não autoriza automaticamente o `eafc-bot` a consumir rotas privadas ou não documentadas do FUT.GG. A integração correta, se vier a existir, seria uma exportação/API documentada e consentida pelo usuário. Enquanto isso, permanecem válidos o comportamento conservador do projeto, o respeito a `robots.txt` durante descoberta e a importação manual como alternativa.

## 4. As quatro notas que não devem ser confundidas

| Nota | Dono | O que representa | Transparência | Uso recomendado no eafc-bot |
|---|---|---|---|---|
| OVR | EA | classificação oficial agregada | fórmula não é o foco; não mede o meta | nunca usar isoladamente para ranquear |
| Genie Grade | FutGenie | “meta rating” proprietário, posicional | fórmula não publicada | referência de produto, não copiar |
| GG Rating | FUT.GG | desempenho por Role e meta | metodologia geral publicada | comparar cartas dentro do elenco |
| `analyze.Score()` | eafc-bot | opinião do bot por posição | código e pesos locais | avaliar compras/upgrades no mercado |

### Genie Grade

As [release notes do FutGenie](https://www.futgenie.gg/release-notes) chamam a Genie Grade de “meta rating”. A nota surgiu junto ao preço, foi usada no antigo Gauntlet Builder, passou a variar por posição nas views de squad e recebeu correções para cartas evoluídas. A listagem pública mostra valores decimais próximos de uma escala 0–100, distintos do OVR inteiro.

Isso é o máximo que a fonte primária sustenta. O FutGenie não publicou:

- fórmula ou pesos;
- fatores completos;
- conjunto de treino;
- método de calibração;
- validação contra partidas;
- prova de que “AI” significa ML ou LLM;
- política formal de atualização com a power curve.

Assim, “a nota é uma rede neural” ou “usa exatamente PlayStyles, body type e AcceleRATE com tais pesos” seriam invenções. É possível que seja uma heurística bem calibrada, um modelo estatístico ou uma combinação; não sabemos.

### GG Rating, que já existe no projeto

O FUT.GG publica uma metodologia mais clara: GG Rating vai de 1 a 99,9, é específico por **Role**, decompondo desempenho em 41 ações ponderadas. Entre os fatores declarados estão atributos, altura, peso, body type, running style, AcceleRATE, PlayStyles, skill moves, weak foot, familiaridade de Role e pé dominante. Os pesos variam por Role e são ajustados com meta, testes e feedback. [GG Rating Explained](https://www.fut.gg/news/gg-rating-explained-how-players-really-play/).

O `eafc-bot` já respeita essa separação:

- `internal/domain/player.go` guarda GG Rating geral e por posição;
- `internal/futgg/collect.go` enriquece posições do XI;
- `internal/analyze/upgrade.go` e `internal/report/report.go` usam GG Rating no contexto do clube;
- `internal/analyze/roles.go` mantém o `Score()` local para o mercado.

### Melhoria recomendada para a nota local

Não tentar adivinhar a Genie Grade. Criar uma nota própria explicável:

```go
type RatingBreakdown struct {
    Perfil       string
    Versao       string
    Posicao      domain.Position
    BaseAtributos float64
    PlayStyles   float64
    SkillsWeakFoot float64
    Ritmo        float64
    ForaDePosicao float64
    Total        float64
    Confianca    string
    DadosAusentes []string
}
```

O desenho final pode variar, mas deve preservar estes invariantes:

- tipo ou wrapper distinto para `BotScore` e `GGRating`;
- perfil versionado por ciclo/patch;
- breakdown reproduzível;
- nenhuma compensação silenciosa para dado ausente;
- comparação somente entre posições/funções compatíveis;
- testes de regressão com cartas sintéticas;
- suporte futuro a Role tática sem acoplar o domínio ao fornecedor FUT.GG.

O modelo atual possui seis face stats, PlayStyles, WF/SM, altura e roles, mas não todos os subatributos, body type ou AcceleRATE. A regra correta continua sendo: sem fonte confiável, não inventar precisão.

## 5. Como funciona o solver de SBC

### Comportamento observado

A página oficial do [SBC Solver](https://www.futgenie.gg/sbc-solver-extension) declara que ele:

- lê os requisitos do desafio;
- busca uma combinação válida e barata;
- usa duplicatas não negociáveis e itens não utilizados antes de comprar;
- completa pelo mercado apenas quando necessário;
- respeita cartas bloqueadas;
- preenche a equipe e pode submetê-la.

Nos tutoriais de [desafio individual](https://www.futgenie.gg/posts/auto-complete-sbc) e [set inteiro](https://www.futgenie.gg/posts/auto-complete-set), o usuário escolhe opções, gera soluções, visualiza o que será consumido e seleciona squads. O produto também oferece objetivo por preço ou rating, faixa de rating, repetições, prioridade para duplicatas, exclusões, armazenamento de SBC, concepts e ordem dos desafios. A submissão automática e a compra de concepts pertencem à camada de execução, não ao solver em si.

A inspeção estática confirma a fronteira cliente/servidor em alto nível: a extensão prepara requisitos, inventário serializado, managers, perfis de química, opções e persona para o serviço de solução; recebe elencos calculados e oferece caminhos posteriores para copiar/salvar/submeter pelo Web App. Isso indica que o cálculo principal é pelo menos parcialmente remoto. Não revela a técnica matemática do solver nem demonstra quais campos o backend retém.

A EA confirma que um SBC mistura requisitos de rating, química, qualidade, liga, nação, clube e outros; a submissão é final e remove os itens usados. [EA Help sobre SBCs](https://help.ea.com/en/articles/ea-sports-fc/squad-building-challenges/). Portanto, locks, preview e prova de validade são requisitos de segurança, não apenas conveniência.

### O algoritmo real é desconhecido

Nada público mostra se o FutGenie usa programação inteira, CP-SAT, branch-and-bound, busca local ou heurística. Também não publica garantia de ótimo, qualidade/frescor dos preços ou como valoriza um fodder já pertencente ao clube. “Cheapest valid squad” deve ser entendido como objetivo declarado, não prova matemática.

O problema observável, porém, é claro:

```text
Escolher 11 cartas únicas
  sujeito a rating, química e contagens por liga/nação/clube/raridade/tipo
  respeitando locks, exclusões e elegibilidade
  minimizando compra + custo de oportunidade + desperdício + risco
```

### Solver clean-room recomendado

O `eafc-bot` hoje guarda `SBCChallenge.RequirementsText`, custo da solução externa e tendência histórica. `internal/analyze/fodder.go` reconhece poucos padrões e serve para demanda agregada, não para validar uma equipe completa. O solver novo deve separar quatro módulos:

1. **Normalizador de requisitos**
   - transforma texto/JSON em requisitos estruturados;
   - guarda origem e versão do parser;
   - bloqueia solução quando um requisito não é reconhecido;
   - nunca interpreta silenciosamente texto ambíguo.

2. **Pool e proteção de ativos**
   - inclui cartas do clube e, opcionalmente, concepts de mercado;
   - exclui locks, empréstimos e cartas inelegíveis;
   - penaliza titular, favorito, evoluído/evolvível, carta especial e item negociável valioso;
   - favorece duplicata não negociável e fodder de baixo custo de oportunidade.

3. **Busca determinística**
   - reduz candidatos dominados por buckets relevantes;
   - começa pelas restrições mais escassas;
   - usa limites inferiores de custo e superiores de rating para podar;
   - reaproveita o matching/fluxo existentes quando houver slots/posições;
   - usa branch-and-bound em Go padrão para preservar a política sem dependências;
   - aceita orçamento de tempo e devolve “melhor encontrada”, sem chamar de ótima quando não houver prova.

4. **Validador e explicador independente**
   - recalcula todos os requisitos a partir da solução;
   - mostra contribuição de cada carta;
   - informa custo em moedas, custo de oportunidade e rating desperdiçado;
   - devolve funil de inviabilidade quando não houver solução;
   - gera somente preview/exportação manual.

A função objetivo deve ser lexicográfica, não uma soma opaca. Uma ordem inicial razoável:

1. nunca violar requisito ou lock;
2. minimizar compras necessárias;
3. preservar ativos protegidos;
4. minimizar custo de oportunidade;
5. minimizar rating acima do necessário;
6. preferir duplicatas não negociáveis e itens de menor utilidade.

Uma soma ponderada pode existir internamente, mas o usuário deve enxergar essas parcelas. O endpoint sugerido é somente leitura, por exemplo `GET /api/sbcs/{id}/solucoes`, calculado sobre snapshot local; jamais deve submeter o desafio.

## 6. Como funciona o Gauntlet Builder

### O builder legado documentado

O tutorial do [Gauntlet Genie](https://www.futgenie.gg/posts/gauntlet-genie) descreve o fluxo antigo:

- selecionar um squad-base para copiar formação e táticas;
- excluir empréstimos opcionalmente;
- usar reservas de rating baixo;
- escolher entre três elencos equilibrados ou concentrar os melhores no primeiro;
- gerar o plano e criar squads com nomes únicos ao aplicar.

As release notes dizem que o builder era baseado na meta rating. Isso sugere uma alocação global por valor posicional, não apenas três chamadas independentes ao melhor XI, porque as cartas não podem se repetir. O algoritmo exato, contudo, nunca foi publicado.

Em junho de 2026 surgiu um squad builder para objetivos/torneios/Gauntlet e, em julho, o builder legado foi removido em favor do **Genie Squad Builder**. As [release notes](https://www.futgenie.gg/release-notes) não documentam suficientemente o algoritmo atual. O tutorial antigo serve para entender requisitos e UX, não para afirmar como a versão atual decide.

### Regras mudam ao longo do ciclo

A EA descreveu eventos de lançamento com três squads únicos, 11 titulares e 7 reservas por rodada, junto ao trade-off entre equilibrar força e concentrar a melhor equipe. Outras páginas do ciclo descrevem formatos diferentes. [FC 26 Launch Update](https://www.ea.com/games/ea-sports-fc/fc-26/news/pitch-notes-fc26-launch-update) e [Cornerstones](https://www.ea.com/games/ea-sports-fc/fc-26/news/fc-26-cornerstones).

Consequência: `rounds`, banco, restrições e estratégia precisam entrar como dados versionados do evento. Não se deve hardcode “FutGenie usa três” nem “Gauntlet sempre usa quatro”.

### O eafc-bot já está adiantado

`internal/analyze/gauntlet.go` já implementa um plano completo:

- quatro rodadas hardcoded;
- 11 titulares e 7 reservas por rodada;
- 72 cartas únicas;
- matching global de 44 titulares por fluxo de custo mínimo;
- GG Rating por posição;
- distribuição crescente de força;
- reservas mais fracas entre as sobras;
- química e warnings.

O plano já chega à API e à tela React, com testes de unicidade, posição, estratégia, banco e erro de inventário. Em termos de motor, o projeto não precisa “criar um Gauntlet Builder”; precisa transformá-lo em produto configurável.

### Evolução recomendada

Substituir constantes por um request explícito:

```go
type GauntletRequest struct {
    Regras       GauntletRules
    Formacao     Formation
    Estrategia   DistributionStrategy
    Bloqueados   []int64
    Excluidos    []int64
    PermitirLoans bool
    PesoQuimica  float64
}

type GauntletRules struct {
    Evento       string
    Ciclo        string
    Rodadas      int
    Titulares    int
    Reservas     int
    Restricoes   []SquadRequirement
    Fonte        string
    VerificadoEm time.Time
}
```

Estratégias úteis:

- **equilibrada/minimax:** maximiza a força da pior rodada;
- **crescente:** guarda os melhores para a última rodada, como hoje;
- **mais forte primeiro:** reproduz a opção legada documentada;
- **valor total:** maximiza a soma respeitando todas as rodadas;
- **química-aware:** aceita pequena perda de nota por ganho relevante de química.

Também faltam locks/exclusões, escolha de formação, banco com cobertura tática, regras do objetivo, relatório de inviabilidade por posição e uso real de `chemistry.weight`. O motor `internal/chemistry` já possui contador incremental e pode entrar numa segunda fase de busca local, sem contaminar a interface pública com detalhes do grafo.

O resultado deve continuar sendo plano, campo visual, checklist e exportação. “Aplicar no Web App” permanece fora de escopo.

## 7. Outros recursos do FutGenie

| Recurso | Como funciona no produto | O que aproveitar | O que não copiar |
|---|---|---|---|
| Bulk Pack Opener | seleciona packs e aplica regras em ordem para itens, duplicatas e destinos | editor/simulador de ruleset sobre dados fictícios ou importados | abrir, mover, trocar ou descartar na conta |
| Mercado | overlay de preço, primeiras páginas, mass bid/buy, listagem, snipe e lucro | preço, break-even, taxa, spread, checklist e alertas manuais | lance, compra, listagem e sniper automáticos |
| Club Insights | resume composição e valor do clube/storage | métricas sobre snapshot local | leitura da sessão EA |
| Fodder Value | separa valor de fodder por rating e local | buckets de rating, raridade, negociável e tendência | consumir automaticamente |
| Sticker Book | acompanha cartas obtidas por packs/objetivos/picks/SBCs | diferenças entre snapshots e coleção local | afirmar origem quando ela não foi coletada |
| Evo Builder | paths, sugestões, filtros, playstyles e custo | aprofundar `cards.CardReport` e comparar paths confirmados | aplicar evolução ou comprar automaticamente |
| Locks/favoritos | impede consumo indevido e guia solver | lock genérico local, com motivo e escopo | sincronização por sessão EA não autorizada |
| Objetivos/torneios | monta squad a partir de regras | reutilizar `SquadRequirement` e assignment engine | criar/aplicar squad no jogo |
| Atalhos | acelera ações no Web App | atalhos apenas na UI própria | atalhos que disparam ações EA |

O tutorial de [Bulk Pack](https://www.futgenie.gg/posts/bulk-pack-opener) revela uma boa ideia de produto: regras ordenadas, filtros cumulativos, destino explícito e preview. Essa mesma linguagem pode servir para proteção de ativos e SBC sem nunca abrir um pack.

## 8. O que já existe no eafc-bot

| Capacidade | Estado atual | Lacuna que mais importa |
|---|---|---|
| Notas | `Score()` próprio + GG Rating geral/posicional | breakdown, perfil versionado, Role tática e tipos distintos |
| Melhor XI | matching global por fluxo de custo mínimo | formação, locks, exclusões e química no objetivo |
| Química | motor isolado, explicável e incremental | hoje entra depois da seleção; peso configurado não é usado |
| Gauntlet | motor, API, UI e testes completos | regras dinâmicas, estratégias e controles do usuário |
| SBC | catálogo, requisitos textuais, custo, valor e tendência | nenhum solver do clube nem validação simultânea |
| Evoluções | catálogo, elegibilidade, paths confirmados e relatório atual→potencial | unificar visão confirmada e estimativa, ampliar cache sob demanda |
| Mercado | upgrades, eficiência, momentum, venda/segurar e histórico | liquidez/spread; execução deve permanecer manual |
| Clube/histórico | snapshots, retenção e store JSON/Postgres | insights, coleção e locks genéricos |
| Extensão/importação | nenhum adapter; `Club.Source` prevê `csv` ou `chrome` | fonte neutra e segurança local antes de qualquer integração |

Dois achados técnicos merecem correção antes de ampliar o produto:

1. `config.Chemistry.Weight` existe, mas os analisadores não o usam na escolha.
2. `SellCandidate.NetSellValue` desconta 5%, enquanto `Upgrade.Recoup` não; isso pode distorcer comparação de custo líquido.

Esses itens não são prova de falha observada em produção, mas são dívidas concretas reveladas pela auditoria.

## 9. Roadmap recomendado

### Fase 0 — invariantes e contratos

- registrar em código/documentação que analisadores não recebem cliente EA;
- manter API baseada em `Store`, sem rede nos handlers;
- criar tipos distintos para notas;
- definir locks/exclusões como conceito de domínio local;
- criar fixtures de clube e SBC totalmente fictícias;
- preservar build padrão apenas com biblioteca padrão.

**Pronto quando:** testes conseguem provar que nenhuma proposta produz ação remota e toda decisão é determinística e explicável.

### Fase 1 — Gauntlet e squad builder parametrizados

Pontos principais: `internal/analyze/gauntlet.go`, `internal/analyze/squad_optimizer.go`, `internal/chemistry` e tela Gauntlet.

- criar `GauntletRequest`/`GauntletRules`;
- adicionar locks, exclusões, empréstimos, estratégias e formação;
- extrair assignment engine reutilizável sem expor seu grafo;
- integrar química em segunda fase;
- devolver funil de inviabilidade;
- versionar regras por evento/ciclo e mostrar a fonte/data.

**Pronto quando:** funciona com 3, 4 ou 5 rounds; nunca repete carta; respeita lock; explica posição inviável; reproduz resultado com a mesma entrada.

### Fase 2 — solver consultivo de SBC

Pontos sugeridos: `internal/domain/sbc_requirement.go`, `internal/analyze/sbc_solver.go`, integração com `store.Snapshot` e endpoint GET próprio.

- normalizador fail-closed;
- pool do clube + concepts opcionais;
- política lexicográfica de custo;
- branch-and-bound com limite de tempo;
- validador/certificado independente;
- preview, alternativas e motivo de falha;
- nenhuma compra ou submissão.

**Pronto quando:** todos os requisitos reconhecidos são comprovados; requisito desconhecido bloqueia; locks/titulares são preservados; solução cabe no orçamento; benchmark de clube realista é aceitável.

### Fase 3 — nota explicável

- `RatingBreakdown` e perfis versionados;
- página de comparação mostrando contribuição e dados ausentes;
- tipos fortes `BotScore`/`GGRating`;
- suporte opcional a função tática quando houver dados completos;
- testes de invariantes e snapshots de calibração.

**Pronto quando:** o usuário entende por que A supera B e uma mudança de peso aparece como nova versão, não alteração silenciosa.

### Fase 4 — Club Insights, Fodder Value e coleção

- valor por rating/raridade/negociável/storage;
- duplicatas e ativos protegidos;
- diferenças entre snapshots;
- coleção aproximada com nível de confiança;
- reutilização no custo de oportunidade do SBC.

**Pronto quando:** toda métrica aponta para dados locais e distingue observado de inferido.

### Fase 5 — importação/companion opcional

Preferência:

1. importação manual JSON/CSV validada;
2. exportação/API documentada de parceiro aprovado;
3. extensão apenas para abrir ou visualizar o dashboard local, sem content script em `ea.com`.

Antes de uma extensão conversar com o servidor local, o `serve` precisa bindar em loopback ou usar token de pareamento, validar `Origin`, CSRF e tamanho de corpo. Hoje o binário nativo escuta `0.0.0.0` sem autenticação e expõe operações além de GET; ligar uma extensão a isso ampliaria a superfície de ataque.

**Pronto quando:** nenhum segredo EA é aceito, logs não contêm dados sensíveis e a origem dos dados aparece na UI.

## 10. Fronteira explícita: não implementar

| Fora de escopo | Motivo |
|---|---|
| login EA dentro do eafc-bot | somente parceiros aprovados têm fluxo oficial; não aceitar credenciais |
| captura de cookie, token ou sessão do Web App | risco de conta e segurança; não é necessário para análise |
| extensão injetada em `ea.com` | modifica/intercepta a experiência e reproduz a parte de maior risco |
| endpoint EA não documentado | viola o princípio do projeto e pode contrariar regras da EA |
| POST/PUT/DELETE para EA | altera conta/ativos |
| auto-submit de SBC | ação irreversível sobre cartas |
| buy, bid, mass bid, list, snipe, quicksell | automação de mercado e risco direto de sanção |
| pack opener/player picks | ação na conta, consumo e roteamento de itens |
| aplicar squad, tática ou evolução | mutação da conta; exportar checklist é suficiente |
| contornar rate limit/softban ou mascarar User-Agent | evasão deliberada |
| engenharia reversa do FutGenie | desnecessária e proibida pelos termos do próprio serviço |
| treinar IA com conteúdo coletado da EA | a Positive Play Charter proíbe usar conteúdo/serviços EA para treinar IA |

O caráter pessoal, gratuito ou educacional do projeto não muda essa fronteira. Ele é compatível com estudo de algoritmos clean-room, fixtures sintéticas e análise de dados legitimamente obtidos; não transforma automação de conta em acesso autorizado.

## 11. Decisão recomendada

**Sim, implementar:**

- Gauntlet configurável e chemistry-aware;
- squad builder para objetivos;
- solver SBC somente leitura;
- locks e política de proteção de ativos;
- nota própria explicável e versionada;
- Club Insights/Fodder Value/coleção por snapshots;
- melhores previews de evolução, preço, break-even e checklist manual.

**Talvez, após segurança:**

- importação manual;
- companion local view-only que não roda no domínio EA.

**Não implementar:**

- a camada que usa a sessão EA para executar ações.

Essa direção entrega quase todo o valor intelectual do FutGenie — modelagem de restrições, otimização, explicabilidade, inventário e UX — sem depender de comportamento que o próprio projeto já decidiu corretamente evitar.

## 12. Lacunas que continuam abertas

- fórmula e calibração da Genie Grade;
- algoritmo e garantia de ótimo do solver FutGenie;
- algoritmo do Genie Squad Builder atual;
- arquitetura interna exata da extensão e do app móvel;
- tratamento exato de tokens no Web App;
- origem/frescor de cada feed de preços;
- evolução futura da lista de parceiros da Community API.

Essas lacunas não impedem o roadmap. Pelo contrário: justificam uma implementação própria, testável e transparente.

## Fontes principais

- [FutGenie — site oficial](https://www.futgenie.gg/)
- [FutGenie — release notes](https://www.futgenie.gg/release-notes)
- [FutGenie — SBC Solver](https://www.futgenie.gg/sbc-solver-extension)
- [FutGenie — Gauntlet Genie](https://www.futgenie.gg/posts/gauntlet-genie)
- [FutGenie — Privacy Policy](https://www.futgenie.gg/privacy-policy)
- [FutGenie — Terms of Service](https://www.futgenie.gg/terms-of-service)
- [Chrome Web Store — FutGenie Extension](https://chromewebstore.google.com/detail/futgenie-extension/olhalnjomgocehnhjpdemckdmeccnnfj)
- [EA — Community API update](https://www.ea.com/pt-br/games/ea-sports-fc/fc-26/news/pitch-notes-fc26-community-api-update)
- [EA Help — Regras do EA SPORTS FC](https://help.ea.com/pt-BR/articles/ea-sports-fc/fc-rules/)
- [EA — User Agreement](https://www.ea.com/legal/user-agreement?isLocalized=true)
- [EA — Positive Play Charter](https://www.ea.com/commitments/positive-play/charter)
- [Chrome — Storage API](https://developer.chrome.com/docs/extensions/reference/api/storage)
- [FUT.GG — GG Rating Explained](https://www.fut.gg/news/gg-rating-explained-how-players-really-play/)

> Nota: esta é uma análise técnica e de política de produto, não um parecer jurídico. Fontes e regras podem mudar; revalidar antes de lançar qualquer integração externa.
