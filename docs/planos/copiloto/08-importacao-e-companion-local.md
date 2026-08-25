# Fase 08 — importação e companion local

## Implementação

- Criar `eafcbot import club -file ... -dry-run` e upload local para JSON/CSV documentado.
- Validar em memória e gravar atomicamente; importação inválida ou clube vazio nunca substitui snapshot bom.
- Rejeitar senha, cookie, token e sessão EA; fontes sem identidade física ficam incompletas.
- Aceitar adapter externo apenas para API pública, documentada e consentida.
- Se o companion for implementado, ele será view-only, consumirá a API local e não terá permissão ou content script para `ea.com`.
- LAN exige opt-in e token de pareamento; loopback continua default.

## Gate

Logs não expõem segredos, todo endpoint mutável passa pelo middleware central e nenhuma capacidade de execução é adicionada.

