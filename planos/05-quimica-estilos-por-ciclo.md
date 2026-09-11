# 05 — Química e estilos de entrosamento por ciclo

Estado: motor parcial; regras FC27 pendentes de evidência. Dependência técnica: [04](04-avaliacao-contextual-completa.md). Pesquisa pode começar antes. Integração de versões: 08.

## Resultado esperado

Química observada e simulada aparecem separadamente. Estilo de entrosamento modifica apenas subatributos cujo incremento foi comprovado para o ciclo/contexto. Cada efeito aplicado aponta para tabela versionada, e desconhecimento permanece explícito.

## Estado real do repositório

- `internal/chemistry/` já calcula química e compara com observação do jogo.
- `internal/analyze/chemistry_style.go` carrega tabelas embutidas por ciclo e só usa `status=confirmado`.
- `internal/analyze/chemistry_styles/fc27.json` está `nao_confirmado`, com `estilos: {}`. Sua referência `docs/pesquisa-estilos-entrosamento-fc27.md` está ausente no workspace inspecionado porque o documento foi removido. Preservar a remoção; publicar evidência nova em local próprio se necessário.
- O editor oferece uma lista curta fixa de estilos. Ela não comprova o catálogo válido para FC27.
- Os perfis têm uma penalidade genérica `quimica_por_ponto`; é preciso avaliar sua relação com os incrementos reais para não contar o mesmo efeito duas vezes.

Não concluir desta documentação que regras oficiais ainda estejam ausentes na data futura de implementação. Repetir a pesquisa quando executar o plano; este arquivo registra apenas o estado local de 10/09/2026.

## Pesquisa e artefatos

1. Confirmar ciclo, modo de jogo, patch e plataformas alvo. Consultar fontes primárias atuais, incluindo notas oficiais e regras publicadas; complementar com testes públicos reproduzíveis quando necessário.
2. Registrar URL, data de publicação/consulta, método, valores observados, discrepâncias e limites. Publicar relatório novo junto das evidências do pacote, sem restaurar automaticamente os documentos removidos.
3. Obter catálogo de estilos, aliases/idiomas, aplicabilidade GK/linha e incremento por subatributo em cada nível de química. Verificar teto de atributo e comportamento fora de posição.
4. Auditar regras do cálculo de química: clubes/ligas/nações, treinador, cartas especiais e exceções do ciclo. Tratar divergências entre observado/simulado como diagnóstico.
5. Se a tabela completa não puder ser comprovada, manter as partes desconhecidas indisponíveis/experimentais, indicando exatamente quais regras faltam. Não copiar números de FC26 sem evidência de continuidade.

Conclusão da pesquisa: tabela rastreável ou relatório de lacunas específico. “O site parece usar esse valor” sem demonstração não confirma incremento.

## Implementação

- Substituir chave exclusivamente por ciclo por identidade de tabela que também permita versões/patches/plataformas quando as regras diferirem. Conservar tabelas antigas para reprodução.
- Validar catálogo, nomes normalizados, nível 0–3, atributos finitos, incrementos e limites. Definir representação explícita de incremento zero confirmado, distinta de nível não documentado.
- Função pura de transformação recebe carta base/contexto/tabela e devolve atributos simulados + lista dos incrementos efetivos + limitações. Não gravar os valores simulados na coleta original.
- Química simulada do XI deve ser recalculada antes das notas. Para candidato de troca, simular o XI resultante quando a recomendação prometer efeito coletivo de química; não reutilizar cegamente a química do titular.
- Revisar penalidade genérica de química do bot: manter apenas efeito independente documentado. Preservar cálculos legados como versão antiga, em vez de alterá-los retroativamente.
- Na nota, apresentar base, incremento efetivo e contribuição para a função sem duplicar parcelas; implementar o contrato do 04.
- Expor catálogo confirmado à UI por ciclo/contexto. Estilo manual/desconhecido pode ser guardado, mas não ganha efeito fictício. Seletores distinguem estilos de GK e linha.
- Incluir versão da tabela na avaliação e chaves de cache, usando o pacote do 08.

## Testes e critérios de aceite

- [ ] Níveis 0, 1, 2 e 3 testados com fixtures de regra confirmada, incluindo zero confirmado.
- [ ] Atributo perto do teto recebe somente incremento efetivo; componente bate com nota.
- [ ] GK não recebe transformação de linha por acidente e vice-versa.
- [ ] Estilo inexistente, regra parcial, patch incompatível e química desconhecida produzem limitações distintas.
- [ ] Fora de posição segue regra comprovada do ciclo, sem bonus presumido.
- [ ] Troca que altera química de companheiros mostra impacto consistente no XI.
- [ ] Química observada não é sobrescrita pela simulada nem apresentada como validada pela EA.
- [ ] Pacote antigo reproduz regra antiga após instalar tabela nova.
- [ ] Testes com tabela fictícia estão marcados como fixtures e não contam como validação de gameplay.

## Arquivos e registro

Entradas: `internal/chemistry/{chemistry,modelos,verifica,calibra}.go`, seus testes, `internal/analyze/{chemistry_style,evaluation}.go`, `internal/analyze/chemistry_styles/`, `internal/api/{squad_editor,squad_reference}.go`, editor e tipos web.

Ao fechar, registrar quais regras foram confirmadas, fontes, cobertura por contexto, tabela/hash e quais pendências continuam externas. Uma entrega técnica pode estar pronta com tabela FC27 desconhecida; o item de regras verificadas permanece aberto e visível no plano 12.
