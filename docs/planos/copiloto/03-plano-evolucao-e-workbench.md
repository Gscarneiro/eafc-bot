# Fase 03 — plano de Evolução e Workbench

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

