# FUT Gallery FC 27

Pesquisa feita em 19/09/2026, em leitura pública, sem login ou automação da conta EA.

## Regras confirmadas

- A EA permite reutilizar um item em vários conjuntos; o item não é consumido.
- Itens de empréstimo não contam.
- Itens vendidos continuam válidos porque a Gallery registra que passaram pelo clube.
- Evoluções usam a versão original do item.
- As notas são D, C, B, A e S; recompensas de notas intermediárias são cumulativas.

Fontes: [guia da EA](https://help.ea.com/en/articles/ea-sports-fc/gallery-hub/), [catálogo FUT.GG](https://www.fut.gg/fut-gallery/), [tags FUT.GG](https://www.fut.gg/fut-gallery/tags/).

## Dados públicos do FUT.GG

O catálogo público usa `/api/fut/gallery/fc27/`. Pools de conjuntos usam `/api/fut/gallery/fc27/sets/{id}/pool/` e trazem `score` por item, `poolSize` e `isTruncated`. O Starter Set declarou 19.470 itens e devolveu somente 1.000; portanto uma carta ausente de pool truncado não prova inelegibilidade. O bot mantém essa cobertura explícita na avaliação.

O cálculo usa Item Score e bônus das tags. A pontuação Gallery não é GG Rating, overall nem `analyze.Score()`. First Owner e tags de raridade podem mudar drasticamente a melhor combinação. Dados de primeiro dono e empréstimo desconhecidos ficam como pendência manual.

O “best possible” apresentado no catálogo pode ser C ou B mesmo quando o conjunto tem limiar S. O acompanhamento só termina com S registrado pelo usuário.

## Multiplicadores confirmados na segunda fase

O envelope `data` do catálogo tem `schemaVersion: 1`, 21 tags globais e, em cada tag, `rules`, `target`, `attribute`, `values`, `tiers`, `bonusType` e `thresholdType`. As tags são copiadas para o registro de cada conjunto para que a previsão possa ser recalculada sem rede.

Os operadores observados no bundle público são `COUNT`, `COUNT_ANY`, `MIN_COUNT`, `COUNT_DIFF` e `MAX_COUNT_ALL_SAME`. `COUNT_DIFF` escolhe o maior Item Score de cada grupo distinto; `MAX_COUNT_ALL_SAME` escolhe o grupo cujo bônus calculado é maior e usa a quantidade como desempate. `MIN_COUNT` compara o atributo com o mínimo publicado: a regra de Skill Moves usa o valor da tag mais uma estrela, conforme o modelo do site. Multiples agrupa pelo `BASE_DEF_ID`/`playerEaId`, permitindo versões distintas do atleta.

Cada tag aplica sua porcentagem somente ao subtotal das cartas correspondentes. O bundle implementa `floor(max(percentual * subtotal - 1, 0) / 100)` e soma os resultados das tags; o `-1` é preservado porque aparece no código publicado e difere da descrição resumida da página de ajuda. Tipos ou operadores que o parser não entende ficam pendentes e não entram na previsão conservadora.

Fontes adicionais: [contrato público FC27](https://www.fut.gg/api/fut/gallery/fc27/) e [bundle que contém `tagBonus`, `groupKey` e `matchItem`](https://assets.fut.gg/ts/assets/index-CBpMWNCS.js).

## Implementação dos multiplicadores e recompensas

As regras de agrupamento do contrato publicam `values:["0"]` como marcador.
Esse valor não filtra clubes, ligas ou nações: o motor agrupa todas as cartas
confirmadas pelo atributo da regra. A regra `FIRST_OWNED` também tem
`values:["1"]`, mas esse campo representa uma confirmação booleana; somente o
estado verdadeiro concede o bônus. Empréstimos sempre ficam fora de todas as
tags.

Cada recompensa é persistida com identificador, tipo, rótulo, quantidade,
valor e imagem quando o catálogo os fornece. A tela apresenta a recompensa
associada à letra registrada e as faixas alcançáveis pela previsão, sem
afirmar que um item foi resgatado no jogo.

## Limitações

O perfil público do GG Club não fornece um histórico Gallery completo nem comprova primeiro dono para todos os itens. A implementação acumula cartas observadas nos snapshots e aceita correções manuais. Pools truncados, regras desconhecidas e combinações limitadas pelo orçamento de busca mantêm cobertura parcial; o bot não afirma impossibilidade ou máximo global nesses casos.

## Validação no jogo: LALIGA EA SPORTS em 20/09/2026

O bundle público contém `floor(max(percentual * subtotal - 1, 0) / 100)`, mas
a interface do jogo diverge nos produtos exatamente divisíveis por 100. No
caso real de 30 cartas, a tela mostra `500% × 12.910 = 64.550` para First
Owner. O `-1` produziria 64.549 e o total seria 81.449, enquanto o jogo mostra
81.454. A implementação usa, portanto, `floor(percentual * subtotal / 100)`
com inteiros de 64 bits: as tags continuam aditivas, e a previsão reproduz a
fonte de verdade jogável.

O GG Club fornece `playerDef.isFirstOwner` e `playerDef.loanDuration` quando
esses dados existem. A coleta preserva os dois como verdadeiro, falso ou
desconhecido: inegociável não prova primeiro dono e duração nula não vira
empréstimo falso. Para esta combinação, as seis cartas históricas sem
procedência recuperável receberam confirmação manual limitada ao caso; Gordon
foi marcado explicitamente como não-primeiro-dono.
