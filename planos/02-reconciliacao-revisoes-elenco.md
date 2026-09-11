# 02 — Reconciliação de cartas e revisões do elenco

Estado: pendente. Dependências: [01](01-fundacao-importacao-escopo-cache.md); ler [README](README.md).

## Resultado esperado

Após nova coleta, todo plano continua disponível. O usuário identifica exatamente qual vaga/carta perdeu correspondência, escolhe uma resolução e salva nova revisão. Aplicar referência aponta para uma revisão imutável; editar o plano não muda silenciosamente a referência vigente.

## Base e problemas concretos

- `internal/domain/squad_plan.go` já define `ReferenciaCartaElenco`, `VagaPlanoElenco` e `PlanoElencoSalvo`.
- `internal/api/squad_editor.go` resolve clube/mercado/evolução, normaliza cartas e oferece CRUD. `planoElencoSalvoView` expõe apenas `Pendencias []string`.
- `jogadorDaReferencia` e `referenciaDoClube` fazem verificações diferentes: alinhar identificação física com validação de `PlayerID`; evitar aceitar uma cópia cujo conteúdo/versão mudou sem diagnóstico.
- `handleSavedSquadPlanReference` percorre planos e grava um de cada vez. Não há transação única de troca da referência no handler.
- A atualização preserva `previous.Referencia`, logo deve ser revisada para a promessa de aplicação explícita de revisão.
- `RevisaoEsperada` é opcional e a checagem ocorre antes de gravar. Teste concorrência de verdade: duas requisições podem passar pela leitura inicial.
- `internal/api/squad_reference.go` ignora referência com pendência. A UI precisa expor essa condição e o que está sendo usado em seu lugar.
- `web/src/pages/EditorElenco.tsx` mostra pendências textuais e instrui a substituir cartas manualmente. Referências ausentes no banco/lista precisam continuar visíveis e removíveis.

## Contratos propostos

Adicionar resposta estruturada, preservando `pendencias` para clientes antigos:

```text
PendenciaPlanoElenco
  id estável no plano/revisão/snapshot
  area: titular | banco | nao_relacionados
  indice físico ou índice na lista; posicao quando titular
  referencia_original
  motivo: ausente | copia_ambigua | versao_divergente | alvo_indisponivel
  mensagem e acoes_permitidas
  candidatas[]: referencia exata, nome, versão, posição, cópia física, justificativa
```

Uma candidata é opção para o usuário, nunca correspondência automática por nome. No caso de duplicatas sem identidade física suficiente, apresentar a limitação; não fabricar `ClubItemID`.

Separar plano, revisão e referência ativa:

```text
Plano -> várias Revisoes imutáveis
ReferenciaAtiva(escopo) -> plano_id + revisao
Salvar revisão -> compare-and-swap com revisao_esperada
Aplicar referência -> transação que valida revisão/snapshot e troca o ponteiro
```

Definir política para exclusão de plano ativo: remover referência explicitamente ou recusar com ação de desativar; conservar revisões exigidas por avaliações históricas.

## Implementação em etapas

1. Extrair um resolvedor canônico usado por normalização, avaliação, referência e diagnóstico. Retornar resultado tipado; não inferir motivo por substring de erro. Incluir origem mercado/evolução e identidade da carta base do alvo.
2. Diagnosticar todos os locais, inclusive banco e não relacionados. Para alvo de evolução, distinguir base vendida, caminho expirado/ausente e resultado divergente. Manter dados antigos necessários para o usuário reconhecer a carta.
3. Acrescentar `reconciliacoes` à listagem/detalhe de planos. Gerar `pendencias` legadas a partir do mesmo resultado.
4. Implementar histórico e operação atômica no contrato `SavedSquadPlanStore`, JSON e Postgres. Migrar o registro atual como primeira revisão conhecida, sem inventar histórico anterior.
5. Fazer `PUT` do editor exigir revisão esperada; manter compatibilidade antiga apenas em caminho explicitamente documentado. Conflito retorna 409 com revisão atual. A checagem e a escrita pertencem à mesma operação do store.
6. Validar o plano antes da troca de referência. Pendência impede ativação e devolve diagnóstico; duas ativações concorrentes deixam exatamente uma referência vigente.
7. Na UI, apresentar área/vaga, carta anterior, motivo, candidatas e ações “substituir”/“retirar”. Ações alteram rascunho e entram em desfazer/refazer; salvar ainda é explícito.
8. A UI deve permitir remover referências que não existem mais em `cardsByKey`, especialmente banco e não relacionados. Desabilitar aplicar referência enquanto houver pendências e mostrar o motivo junto ao botão.
9. Mostrar separadamente revisão aberta, última salva e revisão ativa. Salvar uma nova revisão não a aplica automaticamente. Nova coleta também não troca o plano ativo.

## Cenários obrigatórios

- [ ] Mesma versão com cópias A/B: escolha de A preservada; se A sumir, B é sugestão explícita.
- [ ] Mesmo `ClubItemID` com `PlayerID` divergente gera pendência de versão/identidade.
- [ ] Duas vagas CB e duas CM são diagnosticadas pelo índice correto.
- [ ] Carta ausente no XI, banco e não relacionados continua visível e pode ser retirada.
- [ ] Alvo de compra não vira posse ao reconciliar; alvo de evolução conserva cadeia e base.
- [ ] Duas atualizações concorrentes da mesma revisão produzem um sucesso e um 409.
- [ ] Falha na segunda escrita não deixa zero/duas referências ativas.
- [ ] Salvar revisão N+1 não altera referência N; aplicar N+1 atualiza avaliações e cache.
- [ ] Reinício e nova coleta preservam planos e histórico.
- [ ] Pendência da referência ativa é visível; eventual uso do XI observado é identificado como tal.

## Arquivos e validação

Entradas: `internal/api/{squad_editor,squad_reference}.go`, `internal/domain/squad_plan.go`, `internal/store/{store,json,postgres}.go`, `migrations/010_editor_meta_feedback.sql` como esquema de origem, `web/src/{api,types}.ts`, `web/src/pages/EditorElenco.{tsx,css}`, `web/tests/editor-elenco.test.mjs`.

Criar migração nova para histórico/ponteiro, sem alterar a aplicada. Testes de resolução e API em `internal/api/squad_editor_test.go`, paridade JSON/Postgres e testes de interação de pendências. Na UI usar o padrão visual de `web/AGENTS.md`.

## Registro da entrega

Registrar schema, exemplos de diagnóstico, política de conflito/exclusão, provas de atomicidade e casos de duplicata. Desbloqueia a referência confiável para 06/07/09.
