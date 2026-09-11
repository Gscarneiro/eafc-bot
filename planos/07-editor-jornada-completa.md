# 07 — Fechar a jornada do editor de elenco

Estado: editor funcional, fechamento de produto pendente. Dependências: [02](02-reconciliacao-revisoes-elenco.md), [03](03-cobertura-coleta-prioritaria.md), [06](06-rankings-recomendacoes-por-vaga.md).

## Resultado esperado

Completar localmente: importar clube → escolher estilo → montar plano → comparar alternativas → salvar → aplicar referência → registrar feedback. Cada edição atualiza campo, notas, mapa de posições, química, elo fraco e alternativas na mesma revisão.

## Reaproveitar o editor atual

`web/src/pages/EditorElenco.tsx` já tem seleção, arrastar, busca, banco/não relacionados, alvos, desfazer/refazer, localStorage e controle de sequência para avaliação. `internal/api/squad_editor.go` oferece carregamento, avaliação e CRUD. Não reconstruir essas funcionalidades do zero.

Lacunas observadas:

- `FORMATIONS` é um catálogo local de quatro exemplos, não um catálogo comprovado por ciclo. Ter onze titulares atualmente pode marcar origem como confirmada sem comprovar a formação.
- O rascunho inicial usa vagas dos titulares, mas inicia estilo e banco vazios. A revisão ativa completa precisa ser exposta/restaurada com suas funções/estilos/banco.
- `AvaliacaoEditorElencoResponse` contém vagas/média/química/elo, mas não a análise conjunta completa de alternativas e mapa exigida no plano original.
- Cartas na coleção mostram GG bruto. A nota para a vaga selecionada deve acompanhar o avaliador escolhido, com notas comparativas rotuladas.
- O debounce/contador evita parte das respostas fora de ordem; confirmar descarte também ao mudar fonte, clube, snapshot, plano ou desmontar a tela.
- Autosave valida pouco o payload e não trata todas as falhas possíveis de armazenamento.

## Implementação por entregas

### A. Estado e catálogo

1. Expor plano/revisão ativa completos no carregamento, junto do XI observado. Inicializar o editor a partir da referência quando existir; preservar diferença entre observado e planejado.
2. Criar catálogo de formações versionado por ciclo com fonte/verificação e posições/índices físicos. Formações sem comprovação ficam manuais. Desenho do campo e seleção usam esse catálogo único.
3. Ao trocar formação, conservar o conjunto de cartas. Definir mapeamento determinístico para novas vagas e mostrar fora de posição. Substituição automática pelo clube é ação independente, com prévia e opção de desfazer.
4. Banco configurável possui quantidade explícita e regras de excesso. Se o limite do ciclo não for confirmado, identificá-lo como configuração manual. Não presumir número oficial.
5. Extrair operações de estado para módulo testável quando necessário: trocar, mover, retirar, mudar formação, resolver pendência, undo/redo. Operações devem ser atômicas no rascunho.

### B. Avaliação conjunta

1. Requisição de avaliação identifica escopo, snapshot, revisão de rascunho, avaliador/pacote, estilo e todas as vagas. Banco entra no contrato se influenciar alternativas/proteção.
2. Resposta carrega uma única identidade de revisão para campo, mapa, química, elo e alternativas por vaga, reaproveitando 06.
3. Invalidar a análise exibida assim que seu contexto muda. Durante carregamento, mostrar revisão anterior como antiga ou esconder deltas; não atribuí-la à edição atual.
4. Cancelar/desconsiderar respostas atrasadas por identidade completa, inclusive troca de fonte sem mudança no XI. Falha não restaura dado antigo como se fosse atual.
5. Selecionar uma vaga exibe função, posição, estilo, avaliação do ocupante e alternativas de clube/mercado/evolução. Cada opção identifica o titular e seus ganhos/perdas/custo/limitações.
6. Incluir cobertura do 03 e reconciliação do 02. Campo vazio, carta ausente e nota ausente têm apresentações distintas.

### C. Interação e acessibilidade

- Desktop: arrastar entre vagas e grupos com feedback de destino. Ocupar vaga preenchida realiza troca previsível ou movimento explicitamente rotulado.
- Toque/teclado: selecionar carta, escolher destino, confirmar movimento; nenhuma ação depende exclusivamente de drag/hover. Escape cancela seleção; foco permanece utilizável após troca.
- Identificar versão/cópia ao escolher duplicatas. Nome acessível deve incluir posição/vaga e ação.
- Em celular, campo/painel/lista permanecem legíveis, com alvos de toque adequados e sem sobreposição das notas. Seguir componentes/tokens de `web/AGENTS.md`.
- Busca funciona no clube inteiro e nos alvos, incluindo cartas sem análise ou abaixo do corte antigo de overall. Preservar distinção banco/não relacionados/disponíveis e origem simulada.

### D. Persistência e jornada

- Versionar schema do rascunho e migrá-lo conservadoramente. Validar arrays, índices e referências. Falha/quota de localStorage mostra que autosave falhou, preservando edição em memória.
- Carregar rascunho antigo exige reconciliação com o snapshot atual; não descartá-lo silenciosamente.
- Abrir outro plano com rascunho não salvo deve permitir preservar/salvar ou descartar explicitamente. Não sobrescrever rascunho de outro clube/ciclo.
- Salvar, criar como novo e aplicar revisão são ações distintas. Mostrar conflitos e revisão ativa conforme 02.
- Linkar uma comparação ao feedback de gameplay do 09 com cartas/contexto pré-preenchidos. Até a integração estar pronta, não apresentar formulário vazio como jornada completa.

## Testes obrigatórios

| Caso | Resultado esperado |
|---|---|
| Troca de formação | Mesmas cartas, posições novas e limitações visíveis |
| Troca entre duas vagas iguais | Identidade física e funções preservadas corretamente |
| Undo/redo | Recupera formação, cartas, banco, função e estilo como uma operação |
| Resposta R1 chega após R2 | Todos os painéis continuam em R2 |
| Troca de fonte enquanto avalia | Nenhuma nota R1 é rotulada com fonte nova |
| Duas abas salvam | Conflito apresentado sem perder rascunho |
| Carta removida na coleta | Pendência visível e resolvível no XI/banco/não relacionados |
| Alvo de mercado/evolução | Identificado como simulação; custo não vira reserva automática |
| Storage indisponível | Edição funciona com aviso de autosave, sem crash |
| Mobile e teclado | Jornada completa sem arrastar e sem ação apenas no hover |

## Aceite e arquivos

- [ ] Toda jornada local funciona com API real em ambiente de teste isolado.
- [ ] Todos os painéis identificam a mesma revisão/contexto.
- [ ] Formação confirmada exige catálogo do ciclo; manual é identificada.
- [ ] Banco/estilo/funções da referência sobrevivem a reload e nova coleta.
- [ ] As regressões de nota deslocada, titular não identificado e elo fraco inconsistente estão cobertas.

Arquivos: editor TSX/CSS, `web/src/{api,types}.ts`, componentes de campo/nota, `web/tests/editor-elenco.test.mjs`, `internal/api/squad_editor.go` e tipos/catálogo a acrescentar. Rodar testes de interação, build web e validar lista, detalhe e `/time` em desktop/mobile conforme regras locais.

## Registro da entrega

Registrar screenshots/roteiro reproduzível, resoluções testadas, tempo da avaliação em clube grande, testes de concorrência e pontos ainda manuais. Conclusão deste plano libera a integração direta de novas fontes após a base de qualidade do 09.
