# Fase 09 — feedback, backtest e calibração

## Implementação

- Registrar feedback append-only: aceitou, adiou ou descartou, motivo e resultado posterior.
- Reproduzir decisões usando somente dados disponíveis no momento original.
- Exibir calibração apenas após 30 decisões encerradas por perfil/ciclo; antes disso informar amostra insuficiente.
- Medir cobertura, calibração, tempo até decisão, impacto por moeda e arrependimento evitado, sem alegações causais.
- Toda mudança de peso cria nova versão de perfil; o sistema recomenda revisão e não altera pesos sozinho.
- Comparar ciclos separadamente, salvo normalização documentada.

## Gate

Uma recomendação pode ser auditada por snapshot, versão do planner, perfil, fontes e feedback associado.

