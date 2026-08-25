# Fase 07 — agenda de decisões e experiência

## Contratos

`MontarAgenda` compõe os resultados dos planners sem recalculá-los. Cada ação contém ID determinístico, alvo, impacto, moedas, prazo, confiança, procedência, conflitos e deep link.

Buckets padrão: Agora para prazo até 72 horas, preço-alvo atingido ou bloqueio crítico; Esta semana até sete dias; Observando para watchlist, baixa confiança ou dados stale.

## Implementação

- Consolidar Today/Club/Plan/Capital/Data no desktop e Today/Club/Plan/More no mobile; preservar rotas antigas com redirects.
- Manter a identidade visual atual e criar a faixa de decisão, estados honestos, empty/error/loading e alternativas manuais.
- Testar 320/390 px, desktop, teclado, foco, contraste AA, reduced motion, headings, skip link, touch targets e alternativa tabular para gráficos.
- Adicionar testes de componentes e E2E para os fluxos Today → detalhe, cenário de elenco e Workbench.

## Gate

Nenhum estado depende apenas de cor; URLs profundas preservam filtros/scroll; todas as ações críticas funcionam por teclado e com conteúdo reduzido.

