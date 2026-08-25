# Plano do copiloto local de decisões

Este diretório transforma o estudo de evolução do EA FC Bot e a pesquisa do FutGenie em uma sequência implementável. Os documentos são propostas de produto; não autorizam login, automação da conta EA, submissão de SBC, compra, venda ou aplicação de Evolução.

## Ordem

| Fase | Documento | Saída principal | Depende de | Estado |
|---|---|---|---|---|
| 00 | [Estabilização da baseline](00-estabilizacao-da-baseline.md) | regressões fechadas e contrato de erro honesto | baseline atual | concluída (gate) |
| 01 | [Fundação confiável](01-fundacao-confiavel.md) | identidade física, procedência, capital e segurança local | 00 | concluída (gate) |
| 02 | [Plano de elenco e Gauntlet](02-plano-elenco-e-gauntlet.md) | cenários de XI, química, locks e regras de Gauntlet | 01 | concluída (gate) |
| 03 | [Plano de Evolução e Workbench](03-plano-evolucao-e-workbench.md) | grafo, estados e caminhos confirmados | 01, 02 | planejada |
| 04 | [Plano de mercado e ledger](04-plano-mercado-e-ledger.md) | plano global de capital, watchlist e ledger | 01, 02, 03 | planejada |
| 05 | [Rating, Insights e memória](05-rating-insights-e-memoria.md) | notas explicáveis e histórico do clube | 01, 04 | planejada |
| 06 | [Solver consultivo de SBC](06-solver-sbc-consultivo.md) | soluções locais validadas e exportáveis | 01, 02, 05 | planejada |
| 07 | [Agenda e experiência](07-agenda-de-decisoes-e-experiencia.md) | faixa de decisão, navegação e acessibilidade | 02–06 | planejada |
| 08 | [Importação e companion local](08-importacao-e-companion-local.md) | importação manual e acesso local endurecido | 01, 07 | planejada |
| 09 | [Feedback e calibração](09-feedback-backtest-e-calibracao.md) | backtest temporal e revisão de perfis | 04–08 | planejada |

O estado acima é deliberadamente incremental: as Fases 00, 01 e 02 estão
fechadas (identidade física opcional, diff de elenco com multiplicidade,
procedência por capability em `/api/saude`, capital com reserva e segurança
local, Gauntlet generalizado com locks/estratégias/3-5 rodadas, e o Squad
Planner com fronteira Pareto nota×química). A atribuição de qual CÓPIA
física ocupa cada slot ambíguo da escalação — que a Fase 01 deixou como
estado `incompleto` em vez de resolver — segue sem uma resolução automática:
os locks da Fase 02 (Gauntlet e Squad Planner) exigem `ClubItemID`
confirmado para prender uma cópia específica, e degradam para "quantidade
preservada" quando a fonte não prova qual cópia é qual, em vez de adivinhar.
"Empréstimos" (cartas de loan) ficou de fora dos locks/exclusões da Fase 02:
o modelo de domínio não marca essa condição por carta hoje, e não há
confirmação de que a fonte exponha esse dado — ver o "Estado da
implementação" de `02-plano-elenco-e-gauntlet.md`. As demais fases continuam
como especificação executável, sem fingir que uma integração externa já foi
validada.

## Contratos que atravessam as fases

- `Player.ID` continua sendo o ID da versão da carta; a cópia física e o atleta-base são identidades separadas.
- Toda capability informa fonte, horário, cobertura e estado: confirmado, estimado, incompleto, indisponível ou erro.
- Erro de coleta nunca é convertido em ausência de dado ou recomendação de venda.
- Preço líquido aplica a taxa de 5% em um único ponto do domínio de capital.
- Analisadores permanecem puros; rede fica em `internal/futgg`, persistência em `internal/store` e DTOs HTTP em `internal/api`.
- O build Go padrão continua sem dependências externas; fixtures são sintéticas e o ciclo do jogo permanece configuração.

## Definition of done global

Cada fase precisa ter testes em português, migração retrocompatível, API sem quebrar envelopes existentes, `serve -demo` atualizado e uma nota de riscos externos ainda não provados. A verificação mínima é:

```text
cd web && npm install && npm run build && cd ..
go build ./cmd/eafcbot
gofmt -l ./cmd ./internal
go vet ./...
go test ./... -count=1
```

Nenhum documento desta pasta muda a regra permanente: o bot lê FUT.GG e dados importados, mas nunca toca na conta EA.
