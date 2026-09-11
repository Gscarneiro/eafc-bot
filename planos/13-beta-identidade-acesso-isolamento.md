# 13 — Beta: identidade, convites e isolamento de usuários

Estado: etapa posterior. Começar somente após o marco de produto do [12](12-validacao-ci-migracoes.md). Reutilizar escopo local do [01](01-fundacao-importacao-escopo-cache.md).

## Resultado esperado

Beta por convite para até vinte pessoas, com login Google. Cada pessoa acessa somente seus clubes, preferências, snapshots e planos. Modo local continua utilizável sem conta; identidade Google serve ao produto, sem login/integração com a conta EA.

## Base a auditar

O servidor atual foi desenhado para uso local e configuração única. Antes de expô-lo, ler `cmd/eafcbot/serve.go`, `internal/api/api.go`, `internal/config/config.go`, `internal/store/`, rotas de escrita/configuração/importação e recursos estáticos. Proteções locais existentes não comprovam autorização multiusuário.

## Decisões já fixadas e pendentes

Fixadas: Google, convites, teto de vinte participantes, beta por último. Pendentes operacionais: domínio/URLs de callback, credenciais Google do projeto e destinatários dos convites. Preparar tudo que pode ser testado com provedor falso; pedir apenas dados externos necessários quando o resultado estiver pronto para configurar.

## Modelo e fronteiras

```text
Usuario: id interno estável, subject do provedor, perfil mínimo, estado
Convite: id/token protegido, destinatário quando aplicável, expiração, estado
Sessao: identidade interna, expiração, revogação
EscopoAutenticado: usuario_id + clube_id + ciclo
```

- Identidade estável deriva de identificador verificado do provedor, não de e-mail enviado pelo navegador.
- Catálogo público pode ser compartilhado; dados pessoais e feedback pessoal possuem proprietário.
- Autorizar no servidor toda leitura/escrita. IDs de plano/clube recebidos na URL não demonstram propriedade.
- Admin de pesquisa/promoção de meta é capacidade separada. Participante do beta não ativa perfil geral nem muda configuração global.

## Etapas

1. Desenhar modo local versus hospedado com fronteira explícita. No hospedado, rotas sensíveis exigem sessão válida; o modo local não abre exceção acidental quando executado fora do localhost.
2. Implementar autenticação Google conforme documentação oficial atual no momento da execução. Validar emissor, destinatário, assinatura, expiração, `state`/`nonce` e fluxo adequado; testar com provedor falso. Preservar build padrão sem dependência externa obrigatória, usando módulo/tag opcional quando necessário.
3. Criar sessões de servidor com cookies protegidos, expiração, logout e revogação; proteger escritas contra requisições forjadas e avaliar origem/CORS na topologia escolhida.
4. Convite é resgatado de modo atômico. Limite de vinte usuários habilitados não é só regra da UI. Definir como revogação/liberação de vaga funciona e se convites pendentes reservam vaga; tornar a política explícita.
5. Introduzir middleware/serviço que resolve escopo autenticado e injeta dependências adequadas por usuário, evitando `Server.Store`/configuração global mutable compartilhada.
6. Migrar estado pessoal de 01 para namespace de proprietário. Importação do acervo local para um usuário requer associação explícita; não atribuir dados antigos ao primeiro visitante.
7. Revalidar todas as rotas: planos, cartas privadas, snapshots, exportação, favoritos, saldo, ledger, feedback, paths, progresso, configuração e jobs. Validar também arquivos estáticos/relatórios se contiverem clube privado.
8. Isolar caches, localStorage, filas e logs. Trocar login remove estado privado da sessão anterior no navegador; artefatos públicos não podem embutir dados pessoais.
9. Implementar tela de entrada, convite inválido/expirado, acesso aguardando convite, limite atingido e sessão expirada. Evitar perder rascunho ao renovar login; oferecer recuperação no mesmo proprietário.
10. Implementar ações administrativas mínimas: convidar, revogar participante/sessões e exportar/remover dados do próprio usuário conforme política definida. Envio de convites a pessoas requer autorização explícita do usuário da tarefa.

## Testes adversariais e de funcionamento

- [ ] Usuário A não lê/altera plano, snapshot, feedback, arquivo ou job de B, mesmo adivinhando IDs.
- [ ] Alterar `usuario_id`, gamertag ou clube no body/query não muda proprietário autorizado.
- [ ] Cache de A nunca responde para B; logout/login não reapresenta rascunho privado anterior.
- [ ] Convite reutilizado, expirado/revogado e resgates concorrentes são tratados corretamente.
- [ ] Duas admissões concorrentes no limite não criam o participante 21.
- [ ] Token adulterado/expirado ou com destinatário incorreto é recusado.
- [ ] Participante não promove meta global, altera paths de servidor ou lê segredos da configuração.
- [ ] Falha de autenticação não destrói dados; sessões revogadas perdem acesso.
- [ ] Modo local e build padrão continuam funcionando.

## Registro da entrega

Registrar arquitetura de identidade/escopo, migrações, inventário de rotas, resultados dos testes A/B, configurações externas ainda necessárias e política de convites. Não registrar tokens/credenciais nos arquivos Markdown ou fixtures.
