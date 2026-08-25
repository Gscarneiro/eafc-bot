# Fase 04 — plano de mercado e ledger

## Contratos

`PlanejarMercado` recebe necessidades do elenco, opções de Evolução, capital, histórico, watchlist e observações de preço. Devolve ações manuais de comprar, vender, esperar ou observar, conflitos, custo líquido, confiança e justificativa.

O ledger é append-only para compra, venda, SBC, Evolução, ajuste e reversão. Watchlist e ledger são estado local, particionado por ciclo, com implementações JSON e Postgres.

## Implementação

- Registrar plataforma, fonte, frescor, cobertura e qualidade de cada preço.
- Calcular P&L e break-even líquidos; nunca afirmar liquidez ou causalidade sem volume/spread observado.
- Produzir plano global que respeite reserva e capital comprometido, em vez de decidir cada carta isoladamente.
- Adicionar `/api/planos/mercado` e recursos locais para watchlist e lançamentos; manter listagens OData atuais.

## Gate

Casos de múltiplas compras, vendas planejadas, preço ausente/stale, taxa, reserva e conflito entre compra, proteção e Evolução são determinísticos e explicáveis.

