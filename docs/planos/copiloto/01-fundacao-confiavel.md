# Fase 01 — fundação confiável

## Contratos

Adicionar uma referência de ativo com três identidades:

- `ClubItemID`: cópia física, somente quando a fonte provar estabilidade e unicidade.
- `CardID`: `Player.ID`, a versão da carta.
- `PlayerKey`: atleta-base, sem confundir versões ou homônimos.

Adicionar metadados de observação por capability — fonte, horário, cobertura, avisos, erro e estado — sem envolver todo escalar em `Observation<T>`. Criar `/api/saude` para expor esse estado.

Criar um tipo de capital com saldo observado/manual, orçamento extra, reserva, bruto vendável, líquido vendável e capital comprometido. A taxa de 5% e o arredondamento ficam centralizados.

## Estado da implementação

Concluída nesta entrega. Os contratos abaixo agora têm cobertura no código:

- `domain.ClubPlayer` preserva o `ClubItemID` opcional do envelope do GG
  Club, e `store.DiffClubs` compara filas por identidade/carta sem colapsar
  duas cópias — testado com fixtures de duplicata (ids distintos e id comum,
  ver `TestClubPreservaDuplicatasComItemIDsDistintos`/
  `TestClubDuplicatasComIDComumNaoInventaUnicidade`) e probes sucessivos
  (`TestClubItemIDEstavelEntreProbesSucessivos`).
- `futgg.Observation`/`Snapshot.Capabilities` (`internal/futgg/observation.go`)
  dão a cada capability (clube, mercado, evoluções, objetivos, SBCs,
  notícias) fonte, horário, cobertura, avisos, erro e um dos cinco estados
  (confirmado/estimado/incompleto/indisponível/erro) — sem embrulhar cada
  escalar do bot num `Observation<T>`. `GET /api/saude` expõe isso; um
  snapshot gravado antes deste contrato aparece como `legacy_snapshot`, nunca
  como "tudo certo" por omissão.
- Clube sem nenhuma cópia com `ClubItemID` vira `estimado` (identidade
  derivada do id da carta, sem linhagem por cópia); duas cópias da mesma
  carta na escalação ativa — que só informa `playerEaId`, não a cópia física —
  viram `incompleto`, em vez de o bot escolher uma cópia em silêncio. A
  resolução de QUAL cópia está de fato em campo (mudar `Club.Starter`/
  `PlayerByID` para usar identidade física em vez do primeiro match) fica
  para a fase 02 (elenco): aqui o objetivo era parar de mascarar a
  ambiguidade, não resolvê-la.
- `domain.Capital` registra bruto, líquido, extra, reserva e comprometido;
  `market.reserve` (config + UI) agora alimenta essa reserva de verdade — o
  orçamento usado por `FindUpgrades`/`FindEvolutions`/`/api/evolucoes` é
  sempre `Capital(...).Available`, nunca mais um `cash+raisable+extra` ad-hoc
  que ignorava a reserva configurada.
- O servidor nativo ouve loopback por padrão (Docker opta explicitamente por
  `EAFC_LISTEN_HOST=0.0.0.0`). Origin, CSRF e o teto de corpo (1 MiB) para
  toda escrita local (`POST /api/job`, `PUT /api/config`,
  `PUT /api/evolucoes/favoritos`) passam por um único `guardLocalWrite`, em
  vez de cada rota reimplementar sua própria checagem parcial.
- `config.Config.RedactSecrets` apaga DSN do Postgres e cookie de sessão de
  qualquer mensagem de erro antes dela chegar ao stderr ou a
  `JobStatus.LastError` (exposto por `/api/job`) — o driver do Postgres pode
  ecoar a DSN inteira num erro de conexão malformada, e este é o ponto único
  que intercepta isso.
- JSON e Postgres continuam sem migração nova: `Capabilities` e `Capital`
  vivem dentro do mesmo payload (arquivo JSON / coluna `JSONB`) que já
  guardava o snapshot inteiro — não há coluna estruturada nova para migrar.

## Implementação

- Migrar índices, diffs, favoritos e snapshots para multiconjuntos. Snapshots antigos continuam legíveis e recebem identidade derivada apenas como estimada, sem sustentar lineage.
- Validar o ID opaco do GG Club em fixtures de duplicata e probes sucessivos. Se a escalação só trouxer `playerEaId` e houver cópias ambíguas, retornar estado incompleto.
- Aplicar observação por capability em `futgg.Snapshot`, API e store; manter coleta parcial e erros acumulados.
- Corrigir `Budget`, `Upgrade.Recoup`, vendas e futuros solvers para usarem líquido, reserva e disponibilidade coerentes.
- Fazer o servidor nativo ouvir loopback por padrão; centralizar Origin, CSRF e limite de corpo para toda escrita local. LAN será opt-in.
- Estender JSON e Postgres com migrações aditivas e interfaces estreitas; `store.Snapshot` continua sem importar `report`.

## Gate

Snapshots antigos, JSON e Postgres fazem roundtrip; duas cópias não colapsam; erros e incompletude aparecem em `/api/saude`; build Go padrão permanece sem dependências externas.
