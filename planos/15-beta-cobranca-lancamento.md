# 15 — Beta: cobrança e fechamento do lançamento

Estado: posterior ao produto e à infraestrutura. Dependências: [13](13-beta-identidade-acesso-isolamento.md) e [14](14-beta-quotas-operacao-hospedagem.md).

## Objetivo

Preparar a camada comercial sem comprometer a jornada por convite nem ativar cobranças indevidas. Encerrar o lançamento apenas com evidências de produto, isolamento, operação e limitações comunicadas.

## Decisões que ainda não foram tomadas

O usuário adiou cobrança para o final, mas não definiu preço, periodicidade, benefícios pagos, provedor, política de cancelamento/reembolso ou se os vinte convidados pagarão no beta. Essas decisões não podem ser inventadas pelo modelo.

Implementar estrutura e testes em modo de cobrança desligado/sandbox. Antes de configurar produtos/preços reais, apresentar opções concretas e obter as decisões comerciais necessárias. Convite e pagamento devem ser conceitos separados; beta gratuito por convite continua sendo uma opção até decisão explícita.

## Desenho funcional

```text
Conta do produto -> usuário interno
Direito de acesso -> convite/beta ou assinatura conforme política
Cliente/assinatura de cobrança -> IDs externos associados no servidor
Evento de cobrança -> idempotência, estado, data, origem verificada
```

- Acesso é decidido no servidor; não confiar em estado de pagamento enviado pelo navegador.
- Usar checkout/portal hospedados pelo provedor escolhido, quando adequados. O produto não deve armazenar dados completos de cartão.
- Falha do provedor não apaga clube/planos. Definir período de tolerância e efeito de suspensão explicitamente.
- Restringir configuração comercial a administrador autorizado, com trilha de alterações.

## Etapas

1. Após qualidade/infra validadas, reunir decisões comerciais acima e pesquisar documentação/preços atuais de provedores. Comparar custos reais, integração e meios de pagamento exigidos pelo público escolhido.
2. Definir máquina de estados de acesso e cobrança: convidado, ativo, pendente, inadimplente, cancelado, expirado e exceções necessárias. Definir quais estados conservam leitura/exportação de dados.
3. Implementar adaptador isolado do provedor com simulador/fakes para testes. Configuração padrão permanece desligada, sem chamadas/cobranças reais.
4. Criar checkout autenticado usando preço/benefício autorizado do servidor; associar conta externa ao usuário interno de maneira inequívoca.
5. Processar webhooks com verificação conforme documentação oficial atual, deduplicação e tolerância a ordem invertida. Quando necessário, reconciliar estado consultando fonte autoritativa do provedor.
6. Implementar cancelamento/gerenciamento e UI que mostra preço, periodicidade, próxima cobrança e acesso restante conforme contrato decidido. Exibir falhas sem perder rascunho ou dados.
7. Criar migrações e histórico mínimo de eventos/assinaturas; preservar isolamento de 13 e evitar dados de pagamento em logs comuns.
8. Testar o fluxo inteiro em sandbox antes de configurar modo real. Configuração de produtos/preços, compras de serviço e cobranças reais exigem autorização correspondente.

## Testes obrigatórios de cobrança

- [ ] Webhook repetido ou fora de ordem não duplica assinatura/direito nem reativa cancelamento incorretamente.
- [ ] Evento com assinatura inválida e preço/usuário adulterado é recusado.
- [ ] Checkout interrompido, pagamento recusado e indisponibilidade do provedor têm estado consistente.
- [ ] Cancelamento e reativação obedecem regras aprovadas e preservam dados.
- [ ] Usuário A não gerencia cobrança de B nem escolhe arbitrariamente preço/desconto.
- [ ] Modo desligado permite beta por convite conforme política e nunca inicia cobrança.
- [ ] Testes não usam cartão real nem criam cobrança real.

## Checklist final de lançamento

- [ ] Relatório do 12 aponta jornada técnica aprovada e qualidade/limitações esportivas reais.
- [ ] Fontes/notas experimentais, ausência de dados e patch não validado são visíveis no produto.
- [ ] Convites, limite de vinte e isolamento A/B do 13 foram comprovados.
- [ ] Operação/backup/restore/rollback e teto de R$300 do 14 estão demonstrados.
- [ ] Política comercial está registrada; cobrança está explicitamente desligada ou validada em modo aprovado.
- [ ] Textos de privacidade, termos de uso e canais de suporte refletem os dados/serviços efetivamente usados. Verificar requisitos aplicáveis com fontes atuais na implementação, sem inventar garantia jurídica neste plano.
- [ ] Onboarding explica importação FUT.GG/local, escolha de avaliador, perfil experimental e aplicação de referência no produto.
- [ ] Há roteiro de suporte para fonte indisponível, reconciliação, nota contestada e recuperação de acesso.
- [ ] Abertura aos participantes e eventuais mensagens/convites têm autorização explícita. Nenhuma divulgação é consequência automática do deploy.

## Registro da entrega

Registrar decisões comerciais, provedor/configuração sem segredos, resultados de sandbox, estado real da cobrança e evidências dos planos 12–14. Listar participantes somente em sistema apropriado, não neste repositório. Finalizar com relatório de lançamento datado e pendências assumidas explicitamente.
