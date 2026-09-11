# 12 — Validação final do produto, CI e migrações

Estado: CI inicial existe; campanha completa pendente. Dependências: entregas 01–11. Regras externas ainda desconhecidas devem aparecer como pendências; não simulá-las como confirmadas para liberar o produto.

## Resultado esperado

Evidência técnica e de experiência de que a jornada local funciona e de que os rankings têm a qualidade declarada. Este é o marco anterior ao beta. Código compilando é condição necessária, mas não comprova qualidade esportiva ou migração segura.

## Base observada

- `.github/workflows/ci.yml` instala UI/Chromium, compila/testa web, verifica Go e aplica todas as migrações em Postgres vazio.
- `web/package.json` usa `node --test`, não Vitest. O workflow passa `--run`; verificar e ajustar o comando para o runner real.
- `cmd/eafcbot/driver_pgx.go` é opcional sob tag `postgres`; `go.mod` padrão não tem `require`.
- A CI atual não demonstra round-trip do `PostgresStore`, migração de base preenchida, concorrência ou restore.
- `web/tests/editor-elenco.test.mjs` usa respostas interceptadas. Esses testes são úteis, mas precisam de uma jornada com API Go/store reais isolados.

## Matriz mínima de validação

| Dimensão | Casos |
|---|---|
| Fonte | FUT.GG, bot, FUTBIN/FUTWIZ quando integrados, fonte indisponível |
| Perfil/contexto | quatro perfis, função repetida, patch validado/desconhecido, plataforma coberta/fora |
| Dados | completos, essenciais ausentes, acessórios ausentes, fonte parcial/desatualizada |
| Identidade | duas cópias, duas versões do atleta, carta evoluída, alvo, carta removida |
| Persistência | JSON, Postgres, migração antiga, falha de escrita, reinício |
| Interação | desktop, mobile/toque, teclado, respostas fora de ordem, duas abas |
| Escopo | dois clubes × dois ciclos; usuário só será acrescentado no 13 |

## Etapas técnicas

1. Criar fixtures pequenas para regressões de identidade/nota e um clube grande para busca/desempenho. Identificar dados fictícios e retirar informações privadas de fixtures reais.
2. Rodar jornada com API real e diretório/banco temporário isolado: importar → estilo → editar XI/banco → alternativas → salvar → aplicar → feedback → reiniciar e verificar persistência.
3. Executar regressões dos incidentes relatados: badge de nota fora da carta; jogador indicado como elo fraco por outra posição; “ganha do titular” sem titular ou com delta incompatível.
4. Testar atualização de schema a partir da última versão anterior preenchida, não apenas banco vazio. Aplicar migrações na ordem, repetir quando a política permitir e exercitar restore.
5. Criar suite de contrato do store executada em JSON e Postgres: planos/revisões/referência, snapshots/escopo, feedback/dedup, pacotes/propostas. Incluir concorrência/rollback real.
6. Compilar e testar Postgres em job separado com dependência opcional isolada (por exemplo módulo/`-modfile` temporário e versão fixada). Conferir que `go.mod` padrão não foi alterado.
7. Verificar formatação, `go vet`, testes Go, build padrão após build web, testes de interação e diff. Avaliar race detector onde suportado para código concorrente modificado.
8. Medir latência de avaliação e memória/rede no clube grande. Definir orçamento de desempenho antes de otimizar e registrar ambiente/medianas/piores casos; evitar meta numérica arbitrária sem medição.
9. CI publica logs e artefatos úteis de falha (screenshot/trace quando disponíveis), sem clube privado ou credenciais. Falha de migração/interação deve falhar o job, não ser ignorada.

## Validação de qualidade

- Rodar avaliação do 09 com dataset reservado real e pacotes fixados.
- Publicar cobertura, divergências por função e incerteza. Se a evidência ainda for insuficiente, manter os perfis experimentais e registrar o que falta coletar.
- Executar roteiro de uso com o dono do produto para as decisões principais. Registrar problemas concretos e corrigir regressões antes de marcar jornada consolidada.
- Catálogo de formações, química/estilos e integrações externas exigem evidência de aplicabilidade atual. Fixtures comprovam comportamento do código, não regras do FC27.

## Critérios de aceite

- [ ] Todos os casos da matriz têm teste automatizado ou roteiro/evidência justificado.
- [ ] Jornada completa passa com API/store reais em ambiente isolado.
- [ ] Build padrão e opcional Postgres passam; dependência opcional não contamina o padrão.
- [ ] Migração de base antiga e restore recuperam dados, revisões e escopos corretos.
- [ ] Sem erro conhecido de mistura de avaliador, titular/vaga ou fonte no resultado principal.
- [ ] Interfaces desktop/mobile/teclado foram verificadas com evidências.
- [ ] Relatório de qualidade real distingue experimental de validado.
- [ ] Pendências externas e suas consequências estão escritas; nenhuma foi marcada concluída por conveniência.

## Marco de liberação para o beta

Criar relatório datado de prontidão com commit, resultados, migrações, evidência de qualidade e limitações. O beta só avança quando a jornada estiver validada e o usuário aceitar explicitamente qualquer limitação material remanescente. Não iniciar hospedagem para compensar problema de qualidade do produto.

## Registro da entrega

Anexar comandos/ambiente, links de CI efetivamente executada, artefatos, relatório esportivo e lista final de bloqueios. Escrever “workflow criado” quando ainda não houver execução remota; não afirmar “CI passou” sem run comprovado.
