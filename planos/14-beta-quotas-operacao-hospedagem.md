# 14 — Beta: quotas, operação e hospedagem

Estado: etapa posterior. Dependência: [13](13-beta-identidade-acesso-isolamento.md), com marco de qualidade do 12 preservado. Limite operacional definido pelo usuário: **R$300/mês**, até vinte participantes.

## Resultado esperado

Ambiente hospedado com dados persistentes, HTTPS, coleta controlada, backup/restore e custos observáveis. Cada usuário tem limites justos de trabalho; falha de uma coleta ou pico de uso não derruba a experiência dos demais.

## Base a reaproveitar

`Dockerfile`, `docker-compose.yml` se presente, `cmd/eafcbot/serve.go`, `internal/scheduler/`, status do job, cache/rate limit FUT.GG e Postgres opcional. Conferir caminhos reais antes de escolher montagem/volume: `.eafc-bot/` é relativa ao CWD e mover apenas `EAFC_DATA_DIR` não move toda configuração/cache.

## Preparação sem contratação

1. Medir carga real da jornada: tempo/memória de avaliação, tamanho de snapshots, frequência de coleta, custos de rede/armazenamento e duração do job. Separar custo do aplicativo do custo eventual do agente de pesquisa em desenvolvimento.
2. Pesquisar provedores/preços atuais somente nesta etapa. Registrar moeda, câmbio consultado, impostos/taxas quando aplicáveis, disco, backup, tráfego, domínio e banco; não usar preços memorizados.
3. Comparar topologias simples compatíveis com build/estado: instância com volume e banco opcional versus app/banco separados. Escolher com base em custo total, restore e manutenção, não apenas preço promocional.
4. Preparar orçamento com margem para variação e estratégia de redução/suspensão antes de ultrapassar R$300. Se nenhuma alternativa couber com requisitos, apresentar o trade-off concreto antes de contratar.

## Quotas e execução de jobs

- Definir limites configuráveis por usuário/escopo: requisições caras, avaliações concorrentes, importações, coletas, tamanho de arquivo, retenção e espaço ocupado.
- Aplicar limites no servidor com resposta que informa motivo e quando/como tentar de novo; evitar botão que parece executar mas não faz nada.
- Scheduler deve carregar contexto do usuário/clube correto. Jobs idênticos podem ser deduplicados, mas posse/credenciais/resultado privado não podem ser compartilhados por acidente.
- Permitir cancelamento, timeout e retentativa limitada. Coleta parcial produz diagnóstico; não publica snapshot vazio sobre bom.
- Coordenar quotas por fonte remota, além das quotas pessoais, respeitando regras de acesso e rate limits confirmados.
- Trabalho de análise usa CPU local e perfis aprovados; execução de pesquisa IA não nasce automaticamente do acesso à tela.

## Implantação e operação

1. Criar build reproduzível, com web embutida e migrações versionadas. Segredos são injetados por ambiente/gestor apropriado, sem imagem ou repositório.
2. Preparar healthcheck de processo e prontidão de banco/esquema, sem expor dados privados. Definir comportamento em migração pendente e falha de dependência.
3. Persistir todos os diretórios necessários e criar backup criptografado/protegido conforme a topologia. Documentar retenção, periodicidade e recuperação de configurações/planos/pacotes.
4. Exercitar restore em ambiente separado e medir perda máxima de dados/tempo de recuperação alcançados. Escolher objetivos de recuperação a partir desse ensaio, com parâmetros explícitos.
5. Logs estruturados incluem identificador de requisição/job, escopo opaco, duração, erro e uso de quota. Não incluir cookies, tokens, payload completo de clube ou dados pessoais desnecessários.
6. Métricas mínimas: latência/erro API, fila/tempo/falha de coleta, idade do snapshot, espaço, sucesso de backup e custo previsto/real.
7. Definir alertas acionáveis e playbooks para disco cheio, coleta quebrada, banco indisponível, quota esgotada e custo alto. Usar mecanismo do ambiente escolhido; automações externas somente se autorizadas.
8. Preparar deploy com teste de smoke e rollback. Migração irreversível precisa de estratégia de compatibilidade/restore; voltar o binário sozinho não resolve mudança incompatível de banco.
9. Deixar infraestrutura e configuração concretas/revisáveis antes de pedir aprovação para contratação/publicação quando necessária. Não comprar serviço nem abrir o beta a pessoas sem autorização correspondente.

## Testes e critérios de aceite

- [ ] Carga representativa de vinte usuários respeita limites sem misturar escopos.
- [ ] Quota simultânea é atômica e não permite bypass por duas requisições concorrentes.
- [ ] Reinício conserva dados/configuração; disco cheio/falha de banco gera erro e recuperação previsíveis.
- [ ] Backup restaura planos, revisões, feedback e pacote ativo em ambiente separado.
- [ ] Coleta com falha parcial deixa UI funcional e diagnóstico visível.
- [ ] HTTPS/callback/login funcionam na URL final configurada.
- [ ] Orçamento datado inclui todos os custos e permanece até R$300 com margem explicitada.
- [ ] Rollback foi exercitado; logs e métricas permitem identificar incidente sem expor dados privados.

## Registro da entrega

Registrar provedor/topologia escolhidos, orçamento com fontes/data, variáveis sem valores secretos, comandos de deploy/restore/rollback, resultados de carga e URL somente quando realmente publicada. Distinguir “infra preparada”, “aprovada” e “publicada”.
