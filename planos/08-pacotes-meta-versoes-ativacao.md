# 08 — Pacotes imutáveis de meta, ativação e reversão

Estado: CLI/lifecycle básicos existentes; integridade e reprodução pendentes. Dependências: [04](04-avaliacao-contextual-completa.md), contrato de tabelas do [05](05-quimica-estilos-por-ciclo.md).

## Resultado esperado

Uma nota histórica pode ser reproduzida com o mesmo snapshot, carta, contexto, motor, perfil e tabela de química. Aprovar e ativar afeta somente o pacote revisado. Publicar outro arquivo com mesmo ID nunca altera silenciosamente avaliações antigas.

## Base e lacunas verificadas

- `cmd/eafcbot/perfil.go` oferece validar/comparar/propor/aprovar/rejeitar/ativar/reverter e histórico em `historico-perfis.json`.
- `RegistroAvaliadores` indexa perfis por ID e carrega JSON embutidos. `Avaliar` escolhe o perfil por ID e preenche versões do carregador; a versão solicitada não seleciona uma versão antiga.
- `PropostaMeta` possui `Pacote`, evidências, resultado textual e mudanças de código. Persistir hash/metadados não garante armazenamento do conteúdo completo aprovado.
- Ativação altera configuração, grava histórico e atualiza proposta em etapas separadas. Falha intermediária pode deixar essas visões divergentes.
- Reversão restaura preferências anteriores. Ela precisa restaurar conteúdo/versão efetivos e associar corretamente eventos empilhados, inclusive após múltiplas ativações/reversões.
- O caminho de configurações que permite selecionar perfil também deve obedecer a política de ativação, sem contornar aprovação para o meta geral.

## Pacote proposto

Manifesto versionado, carregável sem rede, com identidade imutável calculada sobre conteúdo canônico:

```text
manifesto: schema, id, hash, criado_em, autor/origem
  perfil_id/versao, motor_id/versao, ciclo/patches/plataformas/modos
  arquivos de regras/curvas/interações/requisitos
  tabelas de química e demais dependências identificadas por hash
  evidências e hipóteses com referências/data/aplicabilidade
  corpus de regressão e relatórios de qualidade identificados por hash
  limites de cobertura e status experimental/validado
  proveniência do código/commit necessário
```

Status de aprovação pertence ao registro de decisões, não à simples edição do campo `status` no JSON. Conteúdo aprovado é imutável; edição produz novo hash e nova proposta.

## Passos

1. Definir serialização/hash estável e schema do manifesto. Validar dependências, compatibilidade do motor, números finitos, arquivos ausentes e alterações após aprovação.
2. Estender registro para `(perfil_id, versão/hash)` e resolver versão exata do contexto. Solicitação histórica incompatível retorna motivo; nunca substitui pela versão mais recente.
3. Armazenar pacotes imutáveis em escopo de catálogo, com ponteiro de seleção pessoal separado. Perfis embutidos podem continuar como pacotes iniciais experimentais.
4. Preservar conteúdo da carta/snapshot necessário à avaliação histórica. Retenção diária do snapshot não deve apagar artefato pinado por avaliação/feedback; definir arquivo compacto ou referência durável.
5. Documentar política de motor antigo: conservar implementação compatível ou artefato executável/corpus reproduzível. Se não for possível reexecutar, distinguir resultado arquivado de reprodução efetiva.
6. Expandir `perfil validar` para pacote candidato e `perfil comparar` para versão vigente versus candidata explícitas. Expor JSON além do resumo humano, reutilizando o relatório do 09 quando disponível.
7. Máquina de estados: proposta → aprovada/rejeitada → ativada → revertida. Aprovar verifica integridade e relatório exigido. Ativar confere hash aprovado, ciclo/patch/plataforma e dependências.
8. Separar ativação pessoal experimental, quando o usuário a solicitar explicitamente, de promoção do meta geral. Nunca mudar perfil global a partir de preferência/feedback individual.
9. Tornar configuração/ponteiro/histórico/status consistentes por transação ou journal recuperável. Validar antes de qualquer mudança; escrever ponteiro por último. Repetição da mesma ação deve ser idempotente.
10. Reversão seleciona o pacote anterior exato, invalida caches e registra origem/destino. Testar cadeia A → B → C → B → A.
11. Catálogo na UI mostra pacote/versão, cobertura, status, motivo experimental e histórico. Aplicar aprovação pelo mesmo serviço de domínio na CLI/API.

## Testes e aceite

- [ ] Mesmo pacote gera mesmo hash; conteúdo diferente gera hash novo mesmo com ID/versão textual repetidos.
- [ ] Aprovação de hash A não permite ativar conteúdo B.
- [ ] Duas versões do mesmo perfil coexistem e contexto antigo reproduz nota/explicação.
- [ ] Pacote sem tabela/regras/motor requerido falha com mensagem acionável.
- [ ] Falha entre configuração/histórico/proposta recupera estado consistente.
- [ ] Ativação/reversão repetida não duplica evento nem salta versão incorreta.
- [ ] Configurações da UI não contornam política de promoção/ativação.
- [ ] Perfil sem evidência continua experimental mesmo com testes aprovados.
- [ ] Histórico pessoal e catálogo global ficam separados, inclusive por clube/ciclo quando necessário.

## Arquivos e registro

Entradas: `cmd/eafcbot/{perfil,perfil_test}.go`, `internal/domain/evaluation.go`, `internal/analyze/evaluation.go`, `internal/config/config.go`, `internal/api/evaluation.go`, `internal/store/{store,json,postgres}.go`, `internal/store/meta_proposal_test.go`, novas migrações e fixtures.

Registrar formato/hash, política de retenção, demonstração de reprodução de duas versões, falhas simuladas e rollback. 09 e 10 devem consumir esse contrato, não inventar outro formato de pacote.
