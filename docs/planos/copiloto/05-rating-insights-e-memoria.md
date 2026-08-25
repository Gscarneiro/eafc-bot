# Fase 05 — rating, Insights e memória

## Contratos

Separar `BotScore` — opinião do bot para mercado — de `GGRating` — nota observada para comparar cartas do elenco. Adicionar perfil, ciclo, versão e breakdown reproduzível.

Memória do clube opera como multiconjunto: entradas, saídas, duplicatas, cartas protegidas, fodder, permanência e origem observada. Origem desconhecida permanece desconhecida.

## Implementação

- Migrar consumidores de `Score()` para um wrapper compatível de `BotScore` e publicar componentes de nota, dados faltantes e confiança.
- Corrigir diffs por quantidade e criar rollups compactos independentes da retenção dos snapshots completos.
- Criar Insights, Fodder Value e coleção aproximada, todos com fonte e horário.
- Expor `/api/clube/insights` e `/api/clube/colecao` como coleções OData.

## Gate

Breakdown soma exatamente ao total, perfis incompatíveis não são comparados e duas cópias geram deltas corretos.

