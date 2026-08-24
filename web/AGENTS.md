# Frontend

## Direção: telemetria de campo

Cada tela ajuda a decidir uma ação no Ultimate Team. Use a mesma linguagem:
fundo `paper`, superfícies `panel`, bordas `rule`, texto e métricas em
`ink`, e verde `turf` apenas para estrutura, seleção e ação. Moedas usam
`coin`; ganhos/perdas usam `gain`/`cost`; avisos usam `alert`.

## Como alterar uma tela

1. Reutilize `PageHeader`, `RankingControls`, `Chip`, `StatTile` e as
   classes de `shared.css` antes de criar componente ou CSS local.
2. Trate filtros, paginação, ranking, vazios e erros como controles de
   decisão: mesma densidade, rótulos em minúsculas e foco de teclado visível.
3. Mantenha desktop e celular legíveis; prefira quebra de linha a rolagem
   horizontal, exceto linhas do tempo e tabelas de dados densos.
4. Respeite `prefers-reduced-motion` e nunca esconda uma ação apenas no hover.

Listagens usam `useCollection` e o subconjunto OData (`$search`, `$filter`,
`$orderby`, `$top` e `$skip`) antes de criar estado local. `DataList` concentra
cabeçalhos ordenáveis e expansão; `Pagination` e `SearchInput` mantêm a
paginação e a busca na URL. `useData` continua reservado para respostas
escalares, como status, configuração e detalhe de carta.

## Campo

O campo é a assinatura visual da aplicação. Cada linha é espelhada para a
perspectiva de quem escala o time: lado direito do elenco à direita na tela.
Mostre a arte da carta e a nota GG posicional; não envolva a carta em painel
branco, nem repita nome/overall no gramado. O clique deve abrir o detalhe da
carta e ter nome acessível.

## Verificação

Após alteração em `web/`, rode `npm run build`. Verifique uma tela de lista,
uma tela de detalhe e `/time` em largura desktop e mobile antes de entregar.
