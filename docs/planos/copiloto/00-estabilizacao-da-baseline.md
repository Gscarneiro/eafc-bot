# Fase 00 — estabilização da baseline provisória

## Objetivo

Transformar as alterações locais atuais em uma baseline auditada, sem apagá-las nem presumir que estejam prontas. Fechar regressões que podem fazer um erro parecer ausência ou recomendação válida.

## Estado da implementação

Concluída nesta entrega. Os contratos abaixo agora têm cobertura no código:

- falha de path de Evolução mantém `EvolutionStatus=fetch_error` e bloqueia a
  venda da carta;
- cálculo líquido de venda (95%) é compartilhado por orçamento, upgrade e
  recomendações;
- reinício restaura `LastSuccess`, a retenção do histórico é configurável,
  filtros de banco usam OData, e a UI tem fallback visual sem imagem;
- a tendência preserva o histórico, mas anexa a cotação corrente sem inventar
  preço zero;
- o diff do elenco preserva duplicatas e reconhece `ClubItemID` quando o
  envelope do fut.gg fornece um identificador físico.

A suíte de frontend dedicada para hooks e serializadores continua sendo um
item posterior; o build TypeScript/Vite e o `serve -demo` são o gate visual
atual.

## Trabalho

1. Revisar `PlayerKey`, matching do optimizer, Gauntlet por rodada, enriquecimento de cópias e mapeamento de goleiro. Preservar testes que comprovem os invariantes e adicionar casos para duas cópias, duas versões e dados sem identidade-base.
2. Corrigir o contrato híbrido de Evoluções: a UI não deve misturar parâmetros legados com OData na mesma requisição. Cobrir a URL real React → API → envelope com teste de integração.
3. Diferenciar falha ao buscar path, path inexistente, carta inelegível e carta ainda não verificada. O relatório não pode sugerir venda após falha de path.
4. Fechar regressões de `LastSuccess` após restart, retenção configurável, filtros de posição/negociabilidade da reserva, fallback visual sem imagem, tendência que omite o preço corrente e sinal de out-of-packs dependente de momentum.
5. Adicionar uma suíte mínima de frontend para serializadores, hooks e estados loading/empty/error. A validação visual fica em `serve -demo` até a fase 07.

## Gate

- `go test ./... -count=1`, `go vet ./...` e build da UI passam.
- Nenhum erro de coleta produz `Best == nil` sem estado explicativo.
- A baseline local é documentada em um commit separado, sem `git reset` ou descarte de alterações do usuário.
