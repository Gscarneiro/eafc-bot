# 06 — Unificar rankings e recomendações por vaga física

Estado: migração iniciada, auditoria e fechamento pendentes. Dependências: [02](02-reconciliacao-revisoes-elenco.md), [04](04-avaliacao-contextual-completa.md), contrato [05](05-quimica-estilos-por-ciclo.md).

## Resultado esperado

Campo, mapa de posições, elo fraco, banco, otimizador, mercado, evoluções e Gauntlet usam o avaliador escolhido e a mesma comparação por vaga. A recomendação informa posição/função, candidato, titular substituído, notas comparáveis, ganho, custo e limitações.

Exemplo de experiência esperada (valores apenas ilustrativos): “Use A na vaga CM direita, função X, no lugar de B: nota 91,2 → 92,4 neste contexto; melhora passe e controle, perde força; custo 40 mil; química simulada do XI cai 1 ponto”. Os números precisam vir do resultado calculado, nunca do texto da recomendação.

## Base existente e dívida

- `FindUpgrades`, `FindEvolutionsWithOptions`, `FindSquadSwapsWithOptions`, `OptimizeSquadWithOptions` e `BuildSquadPlan` já recebem avaliador/contextos em parte dos caminhos.
- `internal/api/squad_reference.go` recalcula análises para a referência atual.
- `internal/analyze/gauntlet.go:BuildGauntletPlanWithOptions` ainda usa `ClubeNaRegua`, que projeta notas no campo GG por posição. Essa ponte não expressa duas funções diferentes na mesma posição física.
- `avaliarOpcional` permite fallback legado quando não há avaliador. Auditar se algum caminho de produto ainda cai nesse ramo.
- `internal/cards/report.go:bestPaths` escolhe paths por nota GG. Campos GG publicados podem continuar comparativos; a ordenação ativa precisa de política independente.
- Campos como `weakest_gg_rating` e `FinalGGRating` permanecem por compatibilidade. Não renomear a semântica silenciosamente.
- `AGENTS.md` e `web/AGENTS.md` ainda orientam uso exclusivo de GG em partes do elenco. A decisão explícita do usuário substituiu essa regra.

## Contrato da comparação

Propor resultado comum reutilizável em todos os consumidores:

```text
ComparacaoNaVaga
  vaga_id/index, posicao, funcao
  titular_ref, candidato_ref e origem (clube/mercado/evolucao)
  avaliacao_titular, avaliacao_candidato
  comparavel, motivo, ganho opcional
  custo conhecido/desconhecido, ganho_por_moeda opcional, dentro_orcamento
  efeitos_quimica, aspectos_melhoram[], aspectos_pioram[], limitacoes[]
  snapshot, revisao_plano, pacote/metrica/escala
```

O contexto de papel/plataforma/patch é igual para ambos; efeitos da substituição no XI são simulações identificadas. Se a métrica externa não modelar função/química, informar essa cobertura em vez de sugerir que o número foi ajustado.

## Etapas

1. Inventariar chamadas a `Score`, `EvaluateBotScore`, `GGRatingAt`, `ClubeNaRegua`, `Starter(pos)` e ordenações por overall em produção, incluindo CLI/relatório. Classificar cada uso: dado externo comparativo, regra EA legítima (ex.: SBC), compatibilidade ou ranking ativo a migrar.
2. Estabelecer avaliador obrigatório no limite dos fluxos do produto. Preservar wrappers legados para compatibilidade/testes se necessário, com uso explícito; ausência da fonte selecionada nunca troca automaticamente para outro motor.
3. Criar matriz de avaliação por carta e vaga/contexto. Repetições de CM/CB não compartilham a mesma nota se função/estilo diferirem. Cachear por identidade completa.
4. Migrar Gauntlet para essa matriz. Cada rodada tem XI/química próprios; conservar regras de não repetir atleta/carta entre rodadas conforme o domínio vigente. Uma iteração de otimização deve ter limites e desempate determinísticos.
5. Auditar otimização de química: se a formação/troca muda o XI, comparar resultado conjunto recomputado, não só notas com química antiga. Explicar limitações quando um solver usar aproximação.
6. Unificar seleção do elo mais fraco: menor nota disponível no XI/contexto ou outra definição explicitamente rotulada. Empate, nota ausente e vaga vazia não elegem um jogador arbitrário. Campo e painel usam o mesmo objeto.
7. Em reservas, mostrar todas as vagas compatíveis relevantes ou a melhor oportunidade identificada com titular e índice. Corrigir a classe de erro “ganha do titular” sem nome/vaga e deltas de posições diferentes.
8. Para evolução, construir resultado apenas de ganhos comprovados, conservar identidade base/path e reavaliar usando a fonte ativa. Ausência de subatributos finais deixa nota do bot indisponível/parcial conforme 04. Nota GG final publicada continua dado externo, não valida subatributos inventados.
9. Ordenar oportunidades por ganho por moeda somente dentro de escala/contexto comparável. Troca gratuita, preço desconhecido e custo zero real são casos separados. Preservar oportunidades fora do orçamento com rótulo.
10. Proteger XI/banco possuídos da referência contra venda/SBC. Alvos não entram como posse e planos inativos não reservam moedas.
11. Atualizar documentação de avaliador no `AGENTS.md`/`web/AGENTS.md`, preservando orientações de leitura de dado GG comparativo. Usar skill de escrita de documentos para agentes se disponível.

## Testes e aceite

- [ ] Matriz fonte × consumidor comprova que o switch altera todos os rankings sem trocar escala.
- [ ] Dois CM/CB, duplicatas e carta fora de posição produzem comparação com índice/titular corretos.
- [ ] Regressão inspirada no caso Nico: carta com maior nota na vaga não é marcada como pior por GG global ou outra posição.
- [ ] Campo, tabela, mapa, elo fraco e recomendação exibem a mesma avaliação/revisão.
- [ ] Gauntlet recalcula contexto por rodada e não serve plano de outro avaliador/pacote.
- [ ] Evolução com apenas atributos resumidos não inventa subatributos para obter nota do bot.
- [ ] Sem nota ou sem preço não equivale a zero; ganho não comparável é omitido com motivo.
- [ ] Venda/SBC conserva proteção da cópia possuída, sem proteger equivocadamente todas as versões do atleta.
- [ ] Código antigo preservado tem finalidade documentada e não é caminho implícito do produto.

Entradas: `internal/analyze/{squad_optimizer,squad_planner,squad_evaluation,squad_swap,upgrade,evolution,gauntlet,evaluation}.go`, `internal/cards/report.go`, `internal/domain/evolution_path.go`, `internal/api/{api,squad_reference,squad_plan,evolution_paths,resumo,posicoes,mesa}.go`, `internal/report/`, telas `Time`, `PlanoElenco`, `Mercado`, `PlanoMercado`, `AnaliseEvolucoes`, `Gauntlet` e componentes de nota.

## Registro da entrega

Anexar inventário de chamadas/decisão por chamada, matriz de regressões por fonte/tela e limitações de otimização. Esta entrega fornece resultados estruturados para a jornada 07 e comparações externas 11.
