# 01 — Fundação: importação, escopo e cache

Estado: pendente. Dependências: [orientações gerais](README.md). Prioridade: primeira entrega, pois o restante depende de preservar o clube e o histórico corretos.

## Resultado esperado

Importar ou coletar dados para o clube A na temporada X preserva os dados válidos de outras fontes e não interfere no clube B ou na temporada Y. A API passa a usar imediatamente o snapshot comprometido, mantendo recuperação em caso de falha.

## Base existente e lacunas verificadas

- `internal/api/club_import.go:handleClubImport` valida cartas, depois atribui `club.Cycle = s.Cycle` e grava um `store.Snapshot` novo contendo essencialmente clube/erro. Isso pode ocultar um ciclo divergente e descartar mercado, evoluções, catálogos e outros ingredientes válidos do snapshot.
- `internal/store/json.go` usa arquivos como `club_<cycle>.json`, `watchlist_<cycle>.json`, `gameplay_feedback_<cycle>.json`; snapshots ficam em `snapshots/<cycle>/`. Os planos já recebem clube na API do store, mas nem todo estado pessoal possui esse escopo.
- `internal/api/cache.go:loadSnapshot` invalida por TTL/`Status().LastSuccess`. Uma escrita local precisa de invalidação própria.
- `internal/futgg/client.go` calcula chave de cache a partir de URL. Auditar o contexto efetivo antes de concluir quais respostas podem ser compartilhadas.
- `internal/ratingsource/import.go:Aplicar` modifica cartas durante a iteração; uma divergência posterior pode deixar alterações parciais em memória.

## Arquivos de entrada

`internal/store/{store,json,postgres}.go`, `internal/config/config.go`, `internal/api/{club_import,cache,api}.go`, `internal/futgg/{client,collect,observation}.go`, `internal/ratingsource/import.go`, `cmd/eafcbot/{main,serve}.go`, `migrations/`, testes das mesmas áreas.

## Contratos a estabelecer

1. **Escopo local**: identificador estável de clube + ciclo. Propor tipo explícito, como `EscopoClube`, na camada adequada. Gamertag é rótulo externo mutável; definir normalização e como preservar a associação ao mudar o rótulo. O usuário autenticado será acrescentado no plano 13.
2. **Snapshot**: identificador/revisão imutável de conteúdo, data da coleta e escopo. `GeneratedAt` sozinho não deve ser a única identidade de duas importações no mesmo instante.
3. **Procedência**: fonte, instante, ciclo confirmado e abrangência da coleta. Separar “campo ausente” de valor vazio/zero informado pela fonte.
4. **Publicação de atualização**: validar/mesclar em uma cópia; persistir estado consistente; invalidar cache somente após sucesso. Falha deixa o último estado válido legível.

Manter compatibilidade dos endpoints e arquivos antigos por adaptador/migração explícita. Evitar acrescentar parâmetros de clube de maneira diferente em cada método.

## Passos de implementação

### A. Inventário de escopo

- Listar todos os estados: clube, snapshots/histórico, favoritos, ledger, progresso de evolução, análises salvas, paths salvos, planos, feedback, preferências e histórico de ativação pessoal.
- Classificar cada estado como pessoal de clube/ciclo ou catálogo público de fonte/ciclo. Preços públicos podem ser compartilhados quando plataforma e origem forem compatíveis; decisões do usuário não.
- Conferir JSON, SQL, cache API, cache HTTP e localStorage. Produzir tabela de chaves antigas/novas e estratégia de migração.
- Conclusão: nenhum estado da lista fica sem escopo e dono definidos.

### B. Migração conservadora

- Acrescentar uma migração após a última existente no momento da implementação; não reescrever migrações já aplicadas.
- Migrar registros legados somente quando o clube de origem puder ser comprovado. Dados ambíguos ficam em legado pendente, com relatório de resolução; não distribuí-los a todos os clubes.
- Para JSON, preservar cópia anterior e usar escrita temporária/rename com recuperação. Tornar a migração repetível sem duplicar ou apagar registros.
- Conclusão: fixtures com dois clubes e dois ciclos migram e conservam contagens, identidade e conteúdo.

### C. Importação transacional

- Validar ciclo do envelope, clube e de cada carta antes de qualquer escrita. Ciclo explicitamente divergente é erro; ciclo ausente pode usar o destino selecionado, com origem dessa decisão registrada.
- Definir política de mesclagem por fonte/campo: presença confirmada substitui o próprio dado; omissão não apaga enriquecimento de outra fonte. Remoção de posse requer inventário declarado completo ou ação explícita.
- Ler último snapshot do mesmo escopo, substituir/mesclar somente a parte importada, recalcular ingredientes derivados ou invalidá-los declaradamente.
- Preservar mercado, evoluções, funções e metadados que ainda forem válidos. Não manter recomendações calculadas para um clube anterior sem recomputação.
- Tornar a importação FUTBIN/FUTWIZ atômica em memória: prevalidar todas as correspondências e aplicar sobre cópias após sucesso.
- Conclusão: erro na última carta não deixa as anteriores alteradas; importação parcial não elimina dados válidos de outras fontes.

### D. Cache e virada de ciclo

- Chave do snapshot bruto inclui escopo e revisão. Chave de resultado derivado inclui avaliador/métrica, pacote/perfil/motor, contexto, snapshot e revisão do plano.
- A implementação de contexto pertence a 04/08; neste plano criar a integração para invalidar sem duplicar serialização em cada endpoint.
- Escritas de importação, referência, preferências e perfil invalidam os caches dependentes. Não depender do scheduler para perceber escrita local.
- Auditar literais `/26/` e ciclo em código de produção com `rg`; preservar fixtures históricas explicitamente identificadas. Testar descoberta/configuração para dois ciclos e rejeição de endpoint incompatível.
- Conclusão: alternar clube, ciclo ou fonte nunca reapresenta resultado do contexto anterior.

## Testes e critérios de aceite

- [ ] Importar FC26 no destino FC27 falha sem escrita; ciclo ausente segue a política documentada.
- [ ] Dois clubes no mesmo ciclo preservam snapshots, planos, favoritos, progresso e feedback separados.
- [ ] Importação de um clube conserva mercado/catálogos válidos e invalida decisões derivadas antigas.
- [ ] Falha de persistência entre etapas recupera o último estado válido; API não apresenta sucesso parcial como sucesso total.
- [ ] Nota externa com divergência na última carta não modifica nenhuma carta.
- [ ] Cache é invalidado após importação sem mudança em `LastSuccess`.
- [ ] Migração antiga → nova é repetível; rollback/restore foi exercitado em cópia de dados.
- [ ] Build Go padrão continua sem dependência externa.

Executar testes de `internal/config`, `internal/store`, `internal/futgg`, `internal/ratingsource` e `internal/api`. Fechar com a verificação geral do README.

## Registro da entrega

Preencher ao implementar: versão do esquema, política de mesclagem, estados migrados, testes executados, recuperação exercitada e pendências legadas. Este plano desbloqueia 02/03 e fornece o escopo local reutilizado por 13.
