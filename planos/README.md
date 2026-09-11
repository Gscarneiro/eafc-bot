# Plano de implementação restante — produto EA FC

Base inspecionada em **10/09/2026**, commit `206773f27e55b23463d0bdd5624cb4f82e81fd51`.
Estes documentos descrevem trabalho futuro. Sua criação não significa que as funcionalidades estejam concluídas.

## Objetivo e decisões já tomadas

Ajudar o jogador a montar um time melhor para seu estilo e orçamento, com recomendações explicáveis. Priorizar a qualidade da jornada local; FUTBIN/FUTWIZ entram após consolidá-la, e o beta hospedado vem por último.

- FUT.GG fornece inicialmente dados das cartas e uma avaliação externa. O avaliador é uma escolha independente do fornecedor dos dados.
- “Usar nota do bot” começa desligado. A escolha deve reger todos os rankings e recomendações. Outras notas ficam disponíveis para comparação, preservando métrica e escala.
- O bot calcula localmente, com motor determinístico e perfis versionados. IA participa da manutenção sob demanda, com aprovação explícita para ativar mudanças.
- Planos preservam titulares, banco, não relacionados, cópia física, revisão, função, posição, estilo e alvos simulados. Aplicar referência altera somente a referência do produto.
- Os quatro perfis iniciais são meta competitivo, posse, contra-ataque e pontas. A cobertura inicial pretendida é Ultimate Team em PS5, Xbox Series e PC, sujeita à evidência de cada ciclo/patch.
- Beta futuro: convite, até vinte pessoas, Google e teto operacional de R$300/mês. Preços comerciais e fornecedor de cobrança ainda precisam de decisão.

## Como outro modelo deve começar

1. Ler este arquivo, o plano escolhido e os `AGENTS.md` aplicáveis; consultar `C:\Users\gabri\.codex\RTK.md` no ambiente Windows atual.
2. Conferir `git status --short`, o commit atual e os símbolos indicados. O código pode ter avançado desde esta inspeção. Preservar alterações existentes e documentos removidos pelo usuário.
3. Ler apenas as dependências que alteram o contrato da tarefa. Símbolos e caminhos são pontos de entrada verificados; novos arquivos, tipos e rotas explicitamente propostos ainda precisam ser criados.
4. Implementar uma entrega vertical: domínio/persistência quando necessário, API, interface, testes e migração. Atualizar a seção “Registro da entrega” do plano com evidências reais.
5. Marcar como concluído apenas após os critérios de aceite. Separar “implementado”, “testado localmente”, “validado com dados reais” e “dependência externa pendente”.

Esta documentação autoriza implementação local dentro do plano aprovado pelo usuário; não constitui aprovação de pesos candidatos, compra de serviços ou publicação externa. Decisões de produto pendentes estão indicadas nos documentos correspondentes. Não reabrir decisões já registradas acima.

## O que já existe e deve ser reaproveitado

| Área | Estado observado | Limite atual |
|---|---|---|
| Editor | Campo, busca completa, troca de formação, banco/não relacionados, alvos, desfazer/refazer, autosave e planos salvos | Reconciliação textual; histórico/aplicação e integração da análise precisam de fechamento |
| Avaliador | `ContextoAvaliacao`, `AvaliacaoCarta`, quatro perfis JSON, subatributos e componentes | Pesos por grupos amplos; requisitos, mecânicas, validade e versões ainda incompletos |
| Fontes | Switch e importação local de notas FUTBIN/FUTWIZ | Não existem clientes de coleta direta desses dois sites |
| Meta | Propor, aprovar/rejeitar, comparar, ativar/reverter e registro de evidências | Pacote reproduzível e fluxo de pesquisa automatizado ainda incompletos |
| Feedback | Tela, API, persistência, separação ajuste/avaliação e concordância por função | Contexto histórico, deduplicação robusta, candidato versus vigente e incerteza pendentes |
| CI/Postgres | Workflow e migração `010_editor_meta_feedback.sql` | Aplicar SQL em banco vazio não comprova integração do store, atualização de base antiga ou restore |
| Estilos de entrosamento | Motor de incrementos com tabela versionada por ciclo | `fc27.json` está `nao_confirmado`, sem valores; exige pesquisa verificável |

Os testes não foram reexecutados nesta tarefa de documentação. Resultados de execuções anteriores não substituem a validação das próximas implementações.

## Índice, ordem e dependências

| ID | Documento | Dependências para fechar | Entrega |
|---|---|---|---|
| 01 | [Fundação: importação, escopo e cache](01-fundacao-importacao-escopo-cache.md) | Nenhuma | Dados preservados por clube/temporada e invalidação correta |
| 02 | [Reconciliação e revisões do elenco](02-reconciliacao-revisoes-elenco.md) | 01 | Resolver pendências e aplicar uma revisão atomicamente |
| 03 | [Cobertura e coleta prioritária](03-cobertura-coleta-prioritaria.md) | 01 | Diagnóstico por dado/carta e fila de enriquecimento |
| 04 | [Avaliação contextual completa](04-avaliacao-contextual-completa.md) | Contratos 03 | Requisitos por função, explicações e validade do modelo |
| 05 | [Química e estilos por ciclo](05-quimica-estilos-por-ciclo.md) | 04 | Regras comprovadas e limitações explícitas |
| 06 | [Unificação de rankings e recomendações](06-rankings-recomendacoes-por-vaga.md) | 02, 04; contrato 05 | Mesma avaliação por vaga em todos os fluxos |
| 07 | [Fechamento da jornada do editor](07-editor-jornada-completa.md) | 02, 03, 06 | Editor e recomendações sincronizados em desktop/celular |
| 08 | [Pacotes imutáveis de meta](08-pacotes-meta-versoes-ativacao.md) | 04; contrato 05 | Versão fixada, ativação segura e reprodução histórica |
| 09 | [Feedback e qualidade do ranking](09-feedback-qualidade-ranking.md) | 01, 04, 08 | Avaliação independente de candidato, vigente e fonte externa |
| 10 | [Agente de pesquisa de meta](10-agente-pesquisa-meta.md) | 08, 09 | Pesquisa sob demanda que entrega proposta auditável |
| 11 | [Integrações FUTBIN e FUTWIZ](11-integracoes-futbin-futwiz.md) | Jornada 07 e base de qualidade 09; 01, 03, 06 | Adaptadores reais e comparação de métricas compatíveis |
| 12 | [Validação final, CI e migrações](12-validacao-ci-migracoes.md) | 01–11, com bloqueios externos declarados | Evidência técnica e de qualidade da jornada completa |
| 13 | [Beta: identidade, acesso e isolamento](13-beta-identidade-acesso-isolamento.md) | Marco de produto do 12 | Google, convites e isolamento de usuários |
| 14 | [Beta: quotas, operação e hospedagem](14-beta-quotas-operacao-hospedagem.md) | 13 | Serviço operável com custo controlado |
| 15 | [Beta: cobrança e lançamento](15-beta-cobranca-lancamento.md) | 13, 14 e decisões comerciais | Cobrança quando habilitada e checklist real de lançamento |

Ordem sugerida: **01 → 02/03 → 04/05 → 06 → 07 → 08 → 09 → 10 → 11 → 12 → 13 → 14 → 15**. A barra indica frentes conceitualmente independentes, não autorização automática para criar agentes. A pesquisa de 05 pode começar antes do motor; a ausência de tabela verificada não impede implementar as demais partes.

## Responsabilidade dos contratos compartilhados

- **01** define escopo de armazenamento, procedência de importação e identidade de snapshot; **13** acrescenta a fronteira autenticada de usuário.
- **02** define resolução de referências, histórico de revisão e transação da referência ativa; **07** consome esses contratos na experiência do editor.
- **03** define presença/procedência dos dados; **04** define se esses dados bastam para uma função.
- **04** define contexto/resultado do avaliador; **05** produz transformações de química; **06** integra os consumidores.
- **08** define pacote e ativação; **09** define relatório de qualidade; **10** coordena a pesquisa e monta propostas usando ambos.
- **11** acrescenta adaptadores e metadados externos ao contrato estabelecido em 04/06.

Alterações em `domain/evaluation.go`, `api/squad_editor.go`, `store`, `web/src/types.ts` ou migrações exigem conferir o plano dono do contrato. Evitar duas implementações concorrentes para o mesmo conceito.

## Regras que atravessam as entregas

- Build Go padrão somente com biblioteca padrão. Driver Postgres permanece opcional; preservar `go.mod` padrão sem dependências.
- Sem login, automação ou escrita na conta EA. Clube vem do FUT.GG ou de importação escolhida pelo usuário. Respeitar as regras existentes de descoberta/robots e identidade do cliente.
- Zero pode ser nota legítima: usar disponibilidade explícita. Não preencher ausência com overall, GG global ou outro avaliador.
- Comparações requerem mesma fonte, métrica, versão e contexto pertinente. Preço e desempenho permanecem dimensões distintas; orçamento marca oportunidades, não as esconde.
- Cópia física, versão da carta e atleta são identidades diferentes. Vaga é índice físico; posições como CM/CB podem se repetir.
- Toda regra de FC27 deve indicar evidência e aplicabilidade. Ausência de evidência é um estado de produto, não licença para copiar FC26.
- `store` não importa `report`; contratos HTTP ficam em `api`. Estado local continua sob `.eafc-bot/` no workspace.
- A preferência explícita do usuário pelo avaliador substitui o trecho antigo de `AGENTS.md` que obrigava GG dentro do clube. Atualizar essa documentação na entrega 06, inclusive `web/AGENTS.md`.

## Verificação e passagem de trabalho

Executar comandos separadamente, usando `rtk` quando disponível/exigido neste ambiente. Para a UI, na pasta `web/`: `rtk npm ci`, `rtk npm run build`, `rtk npm test`. Na raiz, depois do build web: `rtk gofmt -l ./cmd ./internal`, `rtk go vet ./...`, `rtk go test ./... -count=1`, `rtk go build ./cmd/eafcbot`, `rtk git diff --check`.

Cada entrega deve registrar: commit/base, arquivos alterados, migrações, comandos e resultado, evidências de UI quando aplicável, dados reais versus fixtures, pendências e próximo plano desbloqueado. Se o ambiente impedir uma verificação, registrar o comando não executado e o motivo. Não apresentar esse impedimento como teste aprovado.

Fixtures devem ser isoladas e descartáveis. O usuário pediu para parar a demo; não trocar a instância que ele usa por dados fictícios nem iniciar coleta real apenas para testar layout.

## Instrução pronta para a próxima tarefa

Substituir o nome do arquivo pelo plano escolhido:

> Implemente `planos/01-fundacao-importacao-escopo-cache.md`. Leia primeiro `planos/README.md` e as instruções do repositório. Confira o estado atual antes de alterar arquivos, reaproveite o que já existir e preserve mudanças não relacionadas. Resolva as dependências deste plano, implemente suas etapas e execute seus critérios de aceite. Atualize o registro da entrega com arquivos, testes e pendências comprovadas. Não implemente os planos posteriores nesta tarefa; registre quais foram desbloqueados. Se um requisito depender de evidência externa ou decisão ainda ausente, conclua as partes independentes e identifique precisamente o item restante, sem marcá-lo como concluído.
