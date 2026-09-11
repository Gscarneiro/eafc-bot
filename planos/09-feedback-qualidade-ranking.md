# 09 — Feedback de gameplay e comprovação de qualidade

Estado: coleta e métricas básicas presentes; validação independente incompleta. Dependências: [01](01-fundacao-importacao-escopo-cache.md), [04](04-avaliacao-contextual-completa.md), [08](08-pacotes-meta-versoes-ativacao.md).

## Resultado esperado

Comparar candidato, perfil vigente e fonte externa compatível sobre as mesmas experiências, preservando contexto histórico. Mostrar concordância/divergências por função, cobertura, tamanho da amostra e incerteza. Separar qualidade esportiva de compra e resultado financeiro.

## Base existente e limitações

- `FeedbackGameplay` em `internal/domain/evaluation.go` tem cartas, contexto, preferência, decisão de compra e resultado financeiro.
- API/UI em `internal/api/gameplay_feedback.go` e `web/src/pages/FeedbackGameplay.tsx` já permitem registrar preferência A/B, empate e experiência insuficiente.
- O servidor divide grupos pelo hash de `ComparacaoID` (uma em cinco para avaliação). Esse ID fornecido pelo cliente não garante por si só independência nem impede repetir a mesma experiência com novos IDs.
- `ordemFeedback` resolve cartas no clube do snapshot atual. Evolução, venda ou atualização podem alterar a carta usada na experiência histórica.
- `QualidadeRankingGameplay` publica concordância e grupos por função, mas não incerteza; o endpoint compara candidata e GG, sem comparação completa com versão anterior explícita.
- Contexto possui uma química única e não registra todos os detalhes necessários de A/B; campos importantes podem vir vazios.

## Contrato de evidência

Evoluir a avaliação sem perder registros antigos:

```text
ExperienciaComparativa
  identidade estável de experiência e autor/escopo
  carta A e B: cópia/versão + dados arquivados/hash
  ciclo, patch, plataforma, modo, posição/função, estilo
  contexto de A e B: química/estilo de entrosamento, uso/duração quando informado
  preferência: A | B | empate | insuficiente
  origem: experiência direta | opinião sem uso (não equivalente)
  decisão de compra e resultado financeiro separados
  amostra: ajuste | avaliação, atribuída e congelada no servidor
  revisões e data, sem multiplicar evidências ao editar
```

Não exigir identidade sensível do jogador. No modo local, usar escopo local estável; incorporar usuário autenticado em 13. Experiência insuficiente fica no denominador de cobertura relevante, mas não vira preferência forçada.

## Etapas

1. Abrir formulário a partir de comparação do editor/mercado/evolução com cartas/contexto preenchidos. O usuário confirma experiência real e pode corrigir contexto; nunca inferir vitória esportiva de compra/venda.
2. Validar identidade das duas cartas e contexto mínimo para comparação. Se uma avaliação antiga não tiver dados suficientes, mantê-la como evidência incompleta, identificada no relatório.
3. Arquivar os dados necessários ao lado da evidência ou em artefato imutável do 08. Reavaliar candidato e vigente sobre exatamente a mesma carta/contexto histórico.
4. Definir deduplicação por experiência, com idempotência de envio e histórico de edição. Canonicalizar par A/B com orientação da preferência; repetição acidental ou inversão A↔B não cria prova independente.
5. Particionar ajuste/avaliação no servidor usando grupo de experiência/origem, não evento de clique. Congelar o split em dataset versionado; impedir realocar exemplos após observar resultado.
6. Congelar baseline e candidato por hash. Calcular ambos no mesmo conjunto reservado e informar interseção coberta versus cobertura individual. Não comparar percentuais com denominadores diferentes sem mostrar a diferença.
7. Acrescentar métricas: concordância, divergência, empates, insuficientes, cobertura, mudanças de preferência prevista, recortes por função/patch/plataforma e incerteza.
8. Escolher método estatístico transparente e reproduzível adequado ao tamanho/correlação dos dados; documentar fórmula/semente quando houver reamostragem. Amostra pequena apresenta intervalo amplo/insuficiência, não confiança inventada.
9. Comparar ordenação externa apenas para mesma versão/contexto que a métrica realmente cobre. Não equiparar concordância com fonte externa à verdade esportiva.
10. Publicar relatório JSON + Markdown associado ao pacote: dataset/hash, critérios de inclusão, exclusões, recortes, limitações e decisão recomendada. O agente 10 usa esse artefato.
11. Definir critério de promoção antes de ajustar pesos: cobertura mínima por função/contexto, tolerância de regressão e evidência independente exigida. Registrar parâmetros como política versionada e exigir aprovação; não escolher limiares depois para fazer candidato passar.
12. Feedback pessoal orienta perfil pessoal experimental. Promoção ao meta geral é explícita e respeita origem/independência das amostras.

## Testes e aceite

- [ ] Reenvio/edição/inversão do mesmo par não multiplica evidências indevidamente.
- [ ] Mesma experiência mantém split em retentativas e migrações.
- [ ] Carta evoluída/vendida após feedback não altera a avaliação histórica arquivada.
- [ ] Candidato e vigente usam mesmos dados; exclusões e denominadores são apresentados.
- [ ] “Insuficiente” e empate não são contados como compra acertada nem vitória financeira.
- [ ] Amostra vazia/pequena, função sem cobertura e fonte incompatível têm resposta explícita.
- [ ] Relatório é determinístico para dataset/pacotes/política iguais.
- [ ] Um perfil tecnicamente válido e sem evidência suficiente continua experimental.
- [ ] Isolamento por clube/ciclo é mantido; dados legados ambíguos não entram no meta geral.

## Arquivos e registro

Entradas: domínio de avaliação, `internal/analyze/{gameplay_quality,gameplay_quality_test}.go`, `internal/api/{gameplay_feedback,gameplay_feedback_test}.go`, `internal/store/{json,postgres}.go`, `web/src/pages/FeedbackGameplay.tsx`, tipos/API web e novas migrações.

Registrar método estatístico, política de deduplicação/split, dataset de demonstração (fictício versus real), métricas e evidência ainda necessária. Não preencher parecer de qualidade real a partir apenas de fixtures.
