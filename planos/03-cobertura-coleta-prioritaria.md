# 03 — Cobertura dos dados e coleta prioritária

Estado: pendente. Dependência: [01](01-fundacao-importacao-escopo-cache.md). Consumidores: 04, 07, 10 e 11.

## Resultado esperado

O jogador vê quais dados estão confirmados, ausentes, desatualizados ou incompatíveis e como isso afeta a avaliação. A coleta detalhada atende primeiro titulares, banco da referência, favoritos e alvos; o restante do clube continua pesquisável.

## Base existente

- `internal/futgg/observation.go` registra observações por capacidade, mas não substitui diagnóstico de subatributos por carta.
- `internal/domain/player.go` contém `DetailedAttributes`, PlayStyles, dados físicos e familiaridade. Omissão e lista vazia precisam ter significado explícito.
- `internal/cards/report.go:BuildReports` usa `minRating` e `requiredIDs` para análises de detalhe. Reaproveitar sua seleção, garantindo identidade de versão/cópia no diagnóstico.
- `AvaliacaoCarta` já possui `DadosAusentes`, `Limitacoes` e `Cobertura` textuais. O editor resume notas disponíveis em `Cobertura int`; esse número não informa cobertura de coleta.
- `internal/api/{avisos,leitura,evolution_paths}.go` e `web/src/pages/Status.tsx` são pontos de apresentação já existentes.

## Modelo a acrescentar

Definir presença/procedência na camada de domínio/ingestão; definir agregados HTTP na API. Proposta:

```text
EstadoDado: confirmado | ausente | desatualizado | conflitante | nao_aplicavel
ObservacaoDado: chave, estado, fonte, coletado_em, ciclo, evidencia opcional
CoberturaCarta: referencia, categorias[], pendencias[], prioridade
ResumoCobertura: escopo, snapshot, grupos[], contagens, cursor/facetas
```

Categorias mínimas: identidade, subatributos de linha/GK, PlayStyles/+ (incluindo ausência confirmada), estrelas/pé, porte/AcceleRATE, posições/funções, nota externa por posição, química observada, caminhos de evolução. Separar preço de qualidade esportiva.

O perfil no plano 04 decide “essencial/acessório/irrelevante por função”. Este plano informa presença; não duplicar requisitos do avaliador em filtros da API.

## Passos

1. Mapear campo lógico → parser → origem → enriquecimento. Para cada categoria, documentar como distinguir lista vazia confirmada de campo não coletado. Auditar IDs numéricos de PlayStyles e catálogo de funções carregado no ciclo correto.
2. Preservar observações ao mesclar fontes. Não marcar campo como atualizado só porque outro campo da carta foi coletado. Identidade divergente bloqueia enriquecimento, não gera merge por nome.
3. Criar resumo por grupos: titulares, banco da referência, favoritos, alvos e restante do clube. Grupos podem se sobrepor; informar que seus totais não são somáveis e publicar total único deduplicado.
4. Acrescentar uma rota de diagnóstico — proposta `GET /api/cobertura` e uma coleção de cartas com pendências — com paginação/facetas do `internal/query`. Preservar os diagnósticos atuais por compatibilidade.
5. Montar fila determinística de enriquecimento: primeiro XI/banco ativo, depois favoritos/alvos, depois demais cartas por carência relevante e antiguidade. Planos inativos podem ser enriquecidos sob demanda, sem reservar capital.
6. Integrar ao job/cliente existente com limites de requisição, cache, cancelamento e retomada. A API só consulta estado e solicita job por contrato existente; evitar chamadas remotas escondidas em GET de análise.
7. Uma falha individual atualiza observação e permite continuar. Expor progresso real: pendentes, processadas, falhas, última tentativa, motivo e possibilidade de nova tentativa. Repetição respeita backoff e rate limit.
8. Na UI, apresentar resumo e filtro “dados necessários para esta função”. Ligar dados ausentes da avaliação ao diagnóstico da carta. Cartas sem detalhe/nota continuam na busca, sem corte obrigatório de overall.
9. Separar “coleta completa” de “nota disponível” e “modelo validado”. Uma fonte com todos os campos pode continuar sem perfil validado para o patch.

## Testes e aceite

- [ ] Subatributo ausente é distinto de zero; GK usa os campos corretos, sem reaproveitamento indevido de linha.
- [ ] Nenhum PlayStyle confirmado é distinto de PlayStyles não coletados.
- [ ] Duplicatas físicas não duplicam requisição pública para a mesma versão, mas conservam estado pessoal independente.
- [ ] XI/banco/favorito/alvo abaixo do mínimo de overall entra na prioridade; carta comum segue pesquisável.
- [ ] Fila repetida no mesmo snapshot mantém ordem e respeita cache; cancelamento não destrói progresso.
- [ ] Fonte parcial preserva valores válidos anteriores com data real e estado de desatualização.
- [ ] Mudança de ciclo/clube não reutiliza diagnóstico pessoal de outro escopo.
- [ ] UI diferencia cobertura de coleta, disponibilidade da nota e validação do perfil.
- [ ] Lista paginada funciona em clube grande e contagens/facetas correspondem ao filtro.

Testar parsers com servidor HTTP falso e formatos sem pistas de nome, conforme o padrão do projeto. Evitar chamadas reais em testes de rotina. Executar suites de `futgg`, `cards`, `api` e interação da tela alterada.

## Registro da entrega

Registrar contrato de presença, prioridade, limites de coleta, contagens de fixtures/dados reais e campos ainda sem fonte. Entregar essa matriz ao plano 04 para requisitos do perfil.
