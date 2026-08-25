# Fase 06 — solver consultivo de SBC

## Contratos

Normalizar todas as linhas de requisito. Uma linha desconhecida ou ambígua bloqueia a solução. O solver distingue ótimo comprovado, melhor encontrada por limite, inviável comprovado, requisito desconhecido e dados indisponíveis.

## Implementação

- Montar pool por cópia física, respeitando locks, favoritos, XI, Evoluções ativas, empréstimos e quantidade preservada.
- Implementar branch-and-bound determinístico com biblioteca padrão, limite de nós e timeout configuráveis.
- Usar objetivo lexicográfico: validade, compras, proteção, custo de oportunidade, desperdício e duplicatas.
- Criar validador independente e certificado com rating, química, contagens, unicidade e elegibilidade recalculados.
- Entregar `POST /api/planos/sbc`, preview, checklist e exportação manual; jamais submissão para EA.

## Gate

Nenhuma solução inválida é mostrada; timeout não é rotulado como ótimo ou inviável; todas as linhas reconhecidas aparecem no certificado.

