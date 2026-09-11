# 11 — Integrações diretas FUTBIN e FUTWIZ

Estado: importação local disponível; clientes remotos pendentes. Iniciar após jornada [07](07-editor-jornada-completa.md) e base de qualidade [09](09-feedback-qualidade-ranking.md) consolidadas. Reutilizar 01/03/04/06.

## Resultado esperado

O jogador escolhe a fonte externa e compara notas com identidade, métrica, escala e contexto visíveis. FUT.GG pode continuar fornecendo os atributos enquanto FUTBIN ou FUTWIZ fornece avaliação. Falha de uma fonte mantém a carta pesquisável e sua nota indisponível com motivo.

## Base atual e limites

- `internal/ratingsource/import.go` importa JSON local; não consulta FUTBIN/FUTWIZ.
- `internal/domain/external_rating.go:NotaExterna` preserva métrica, escala, ciclo, patch, plataforma e evidência.
- `Player.ExternalRatings` é um mapa por fornecedor; isso comporta uma métrica por fonte, mas não necessariamente métricas simultâneas distintas.
- `internal/api/evaluation.go` e configurações web já expõem as fontes e caminhos de importação. “Disponível no catálogo” não comprova cobertura da carta ou conectividade real.
- `RatingAt` trata patch/plataforma vazios como ausência de restrição; definir quando ausência significa desconhecido em vez de compatibilidade garantida.
- O contrato atual de `AvaliacaoCarta` precisa expor escala/métrica de maneira estruturada pelo plano 04. Usar valor externo em gráfico 0–100 sem verificação pode distorcer a leitura.

## Pesquisa antes do cliente de rede

1. Consultar fontes atuais de cada site: rotas públicas/documentadas, condições de acesso, limites, cache e formatos. Verificar ciclo alvo em vez de deduzir URL de exemplo de FC26.
2. Identificar o significado de cada métrica. Investigar separadamente avaliação geral, nota por posição, votos de usuários e qualquer índice de meta.
3. Registrar exemplo real por fonte com ID do site, identidade EA da versão, atleta, versão, ciclo, posição, métrica, escala, data e contexto publicado.
4. Confirmar correspondência exata da carta. Nome, clube e overall sozinhos não bastam. Referência útil para investigação, sem presumir validade futura: a página de Luka Modrić em FUTBIN indicada pelo usuário (`https://www.futbin.com/26/player/27525/luka-modric`).
5. Se não houver acesso sustentável e autorizado ao dado necessário, entregar diagnóstico e manter importação local identificada; não chamar isso de integração direta concluída.

## Adaptadores

- Definir interface por capacidade: catálogo de métricas, resolução de identidade, obtenção de avaliações e diagnóstico de disponibilidade. Implementar FUTBIN e FUTWIZ em módulos separados com mesmo contrato.
- Separar ID do fornecedor, ID da versão EA e cópia do clube. Armazenar mapeamento com fonte/evidência e invalidar ambiguidades; não reaproveitar ID de outra temporada.
- Permitir várias métricas de uma fonte, indexadas por identidade completa. Preservar leitor para o mapa legado quando migrar.
- Buscar fora da API de leitura: coleta/job com rate limit, timeout, retries limitados, cache por ciclo/plataforma/métrica e falha parcial.
- Incrementar apenas dados pertencentes à fonte; atualizar sobre cópia e publicar de forma atômica conforme 01.
- Diferenciar “sem nota publicada”, “identidade ambígua”, “fonte indisponível”, “dados antigos” e “métrica não compatível”.
- Validar valores finitos, faixa, posições canônicas, datas de coleta, fonte configurada versus declarada no documento e aplicabilidade. Dados importados antigos seguem a mesma validação do adaptador remoto.

## Comparação e interface

1. Manter switch: desligado usa fonte externa escolhida; ligado usa pacote do bot selecionado. Troca deve invalidar todas as análises afetadas do 06.
2. Mostrar nota ativa e comparativas, cada qual com fonte/métrica/escala/contexto. A nota do bot permanece 0–100; notas externas mantêm escala original ou transformação explicitamente validada/rotulada.
3. Só calcular delta direto dentro da mesma métrica/escala/contexto. Entre fontes, apresentar ordenação/concordância compatível, não média aritmética dos números.
4. Qualquer avaliação externa que não modele química/função do plano informa essa limitação. Não atribuir efeito de simulação a número externo estático.
5. Configurações mostram estado da integração, última atualização e cobertura; mensagens não pedem ao usuário caminhos de arquivo no futuro modo hospedado.

## Testes e aceite

- [ ] Fixtures de resposta por fonte e métrica, incluindo mudança de envelope/campos e falha parcial.
- [ ] Mesmo nome em duas versões/ciclos não se associa automaticamente.
- [ ] Nota em escala diferente não recebe legenda/barra fixa de 100 indevidamente.
- [ ] Dois tipos de nota FUTBIN coexistem sem serem tratados como uma métrica única.
- [ ] Fonte escolhida indisponível não provoca fallback silencioso; carta continua pesquisável.
- [ ] Importação não apaga GG nem outra fonte e falha atômica conserva dados anteriores.
- [ ] Troca de avaliador chega a todos os consumidores da matriz do 06.
- [ ] Evidência de contrato remoto atual fica registrada; testes de rotina não dependem da internet.

## Registro da entrega

Registrar por fonte: acesso confirmado, rotas/ciclo/métricas, identidade, limitações, cobertura real, política de atualização e data de verificação. Se só houver importação, manter claramente o status “importação local” e deixar cliente remoto pendente.
