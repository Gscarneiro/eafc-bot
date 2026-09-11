# 04 — Completar a avaliação contextual e explicável

Estado: parcial; motor atual reaproveitável. Dependência: contrato de presença do [03](03-cobertura-coleta-prioritaria.md). Ler [README](README.md).

## Resultado esperado

Uma avaliação reproduzível por posição, função, estilo, química, ciclo, patch e plataforma. Cada subatributo disponível tem contribuição ou irrelevância documentada para a função. A nota do bot usa escala 0–100, apresenta componentes consistentes, pontos fortes, limitações e dados ausentes separados.

## O que já existe

`internal/analyze/evaluation.go` carrega quatro JSON de `internal/analyze/profiles/`, usa `DetailedAttributes` e considera PlayStyles, estrelas, química, porte, AcceleRATE, pé e familiaridade. Os perfis estão experimentais. A interface `Avaliador` e os contratos de domínio já existem: evoluí-los em vez de criar segundo motor.

## Lacunas concretas a resolver

- Pesos são divididos em cinco grupos amplos (`goleiro`, `defensor`, `lateral`, `meio`, `ataque`). A função escolhida hoje acrescenta sobretudo familiaridade; não configura todos os pesos e interações da função.
- `ctx.EstiloJogo` é transportado, mas o cálculo não faz seleção explícita de pesos por esse campo. Definir relação entre perfil selecionado e estilo do plano.
- Todo atributo presente em `pesos` acaba exigido; atributos omitidos não têm justificativa de irrelevância. Peso zero ainda pode gerar exigência indevida.
- Bônus de PlayStyle são globais ao perfil, com `Plus × 2` e teto fixo no motor. Interações/curvas não são representadas como dados versionados.
- Patch preenchido com texto qualquer deixa de cair na advertência de patch vazio. É necessária lista/matriz de aplicabilidade comprovada.
- A soma dos componentes merece correção: `base` já inclui ganho de estilo e também é emitido um componente com essa contribuição. A nota não o soma duas vezes, mas a explicação pode fazê-lo parecer duplicado.
- Cobertura inicial é declarada antes de confirmar todos os dados. A ausência de PlayStyles não pode parecer coleção confirmada vazia.
- O contexto não registra com clareza escala/métrica externa nem origem observada/simulada da química. O nome da versão da carta deve continuar independente de perfil/motor.

## Contrato do perfil proposto

Evoluir o schema mantendo carregador para a versão anterior até migrar os quatro JSON:

```text
perfil: id, versão, schema, status, ciclos/patches/plataformas/modos cobertos
funcoes[]: id estável, posições, estilo aplicável
  dados_essenciais[], dados_acessorios[], dados_irrelevantes{chave: motivo}
  pesos/curvas por atributo, regras de PlayStyle/+ e interações
  regras de estrelas, porte, pé, familiaridade e limites
  referências de evidência ou hipótese por regra
```

Utilizar identificadores de função estáveis com rótulos traduzidos. Função desconhecida devolve limitação/indisponibilidade conforme requisitos; não cai silenciosamente em uma função inventada.

A seleção deve ser inequívoca: perfil base + estilo do plano + função resolvem um conjunto versionado de regras. Se forem incompatíveis, explicar; evitar manter um estilo no cabeçalho e aplicar outro nos pesos.

## Passos

1. Expandir `ContextoAvaliacao` e `AvaliacaoCarta` com metadados necessários: identidade do pacote (contrato com 08), modalidade, referência da carta base/evoluída, contexto observado/simulado, métrica/escala e aplicabilidade. Adicionar campos sem remover os antigos.
2. Definir schema e validador estrito: números finitos, intervalos, posições/funções válidas, massa de pesos positiva, chaves conhecidas, regras sem conflito, referências de evidência presentes e abrangência explícita.
3. Para cada função dos quatro perfis, classificar todos os subatributos de linha/GK. Zero relevância deve ter justificativa; não acrescentar efeito artificial apenas para “usar todos os dados”.
4. Implementar requisitos por perfil: ausência essencial indisponibiliza a avaliação; ausência acessória produz parcial. Aplicar presença do 03 também a coleções e dados físicos.
5. Externalizar curvas, teto/multiplicadores e interações. Representar apenas mecânicas verificadas; manter hipóteses rotuladas e perfis experimentais. Incluir regra para efeitos sobrepostos que evita dupla contagem.
6. Integrar química/estilos pelo contrato do 05. Aplicar transformações a cópia da carta; atributos resumidos não entram junto dos subatributos equivalentes.
7. Construir explicação cuja soma chega ao total antes/depois do limite: base sem bônus, contribuições, penalidades, ajuste de clamp/arredondamento. Separar lista de entradas usadas da lista de contribuições.
8. Gerar forças/limitações em relação à função: por exemplo aceleração, passe sob pressão, finalização, defesa aérea. “Sem PlayStyle relevante” é ausência/limitação, não ponto forte.
9. Validar patch/plataforma contra cobertura do pacote. Plataforma vazia ou patch desconhecido continua visível mesmo que um número seja calculável em modo experimental.
10. Atualizar APIs e componentes consumidores sem dar nomes GG a notas do bot. O plano 06 conclui a migração dos algoritmos.

## Testes e critérios de aceite

- [ ] Mesma carta/contexto/pacote gera a mesma nota e explicação, independentemente da ordem dos maps.
- [ ] Dois CM com funções diferentes podem receber ranking diferente pelo efeito documentado, não apenas pelo rótulo.
- [ ] Estilo do plano altera a regra prevista ou informa incompatibilidade; campo não é decorativo.
- [ ] Todo subatributo tem uso ou motivo de irrelevância por função; atributo irrelevante ausente não bloqueia nota.
- [ ] Essencial ausente → indisponível; acessório ausente → parcial; preço/arte/raridade cosmética não geram bônus esportivo.
- [ ] Componentes somam o total com tolerância explícita de arredondamento, inclusive estilo, penalidade e teto de 100.
- [ ] PlayStyle/+ e interações obedecem fixtures versionadas, com testes de ausência/duplicação.
- [ ] Patch não coberto, plataforma ausente e perfil experimental continuam identificados.
- [ ] Carta original não é modificada pela simulação ou evolução.
- [ ] Não há dependência de rede/IA no cálculo nem dependência Go externa no build padrão.

## Arquivos e entrega

Entradas: `internal/analyze/{evaluation,evaluation_test,roles,chemistry_style}.go`, `internal/analyze/profiles/*.json`, `internal/domain/{evaluation,player}.go`, `internal/api/{evaluation,squad_editor,club_insights}.go`, `web/src/{types,format}.ts` e componentes de nota.

Registrar schema novo, matriz de requisitos, regras verificadas versus hipóteses, fixtures de explicação e limitações reais. Aprovação científica/esportiva dos pesos pertence ao 09/10; testes técnicos aprovados não tornam o perfil validado.
