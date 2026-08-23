# eafc-bot

Briefing diário do seu Ultimate Team: lê o seu elenco sincronizado no fut.gg,
compara com o mercado, e diz o que trocar, o que evoluir e o que fazer de
objetivo/SBC.

**Nunca toca na sua conta EA.** Não faz login, não automatiza o Web App, não
envia nada para a EA. Toda a leitura do elenco passa pelo GG Club do fut.gg,
que usa a Community API oficial da EA (OAuth, somente leitura). Automatizar o
Web App é justamente o que a EA bane — a punição listada nas regras deles
inclui deleção do clube e ban da conta em todos os jogos EA.

## Começando

```bash
# A UI React (web/) é embutida no binário via go:embed — e o embed exige
# que o build já exista. Sem este passo, o `go build` abaixo nem compila
# (falta o que embutir), não só o comando `serve`.
cd web && npm install && npm run build && cd ..

# Sem -o: o Go nomeia o binário pelo diretório e acrescenta .exe no Windows
# sozinho. Com "-o eafcbot" você ganha um arquivo sem extensão que o Windows
# não executa.
go build ./cmd/eafcbot

# 1. Ver a UI funcionando, sem rede e sem configuração:
./eafcbot serve -demo   # Windows: .\eafcbot.exe — abre sozinho no navegador

# 2. Sincronizar o clube em https://www.fut.gg/gg-club/ (login pela EA)

# 3. Configurar:
./eafcbot init
$EDITOR .eafc-bot/config.json   # preencha "gamer_tag"

# 4. Descobrir as rotas do fut.gg sozinho, e subir o bot de vez:
./eafcbot autoconfig
./eafcbot serve
```

`serve` é o comando do dia a dia: sobe a API + a UI numa porta só
(`http://localhost:4173`, `-port` muda) e tem um scheduler embutido que
coleta sozinho uma vez por dia — sem cron, sem Agendador de Tarefas, sem
rodar nada na mão (ver "Interface" e "Docker" abaixo). `run` continua
existindo como o mesmo trabalho, disparado manualmente — útil para depurar
sem esperar o horário configurado, ou junto de um cron seu se você preferir
não deixar o processo de pé.

## Descoberta automática dos endpoints

O fut.gg não publica contrato de API e muda as rotas entre temporadas. Em vez
de você abrir o DevTools e copiar caminho por caminho, o bot descobre sozinho:

```bash
./eafcbot autoconfig
```

O que ele faz, em três passos:

1. **Minera as rotas do próprio JavaScript do site.** Um SPA moderno não
   escreve a URL da API por extenso em lugar nenhum: ele guarda um *registro*
   de rotas com o caminho relativo (`` tc({path:`players/v2/26/`}) ``) e uma
   função que prefixa `/api/fut/` na hora da chamada. O bot acha essa função
   pela FORMA do corpo dela — o nome é minificado e muda a cada build — e daí
   segue as chamadas. Literais soltos como `` `/players/${id}/` `` continuam
   valendo, e viram `/players/{id}/`.

   Rota que MUDA estado (`purge-cache`, `ban-user`, qualquer `method: POST`)
   fica de fora da sondagem: uma descoberta que só quer ler não tem o que
   fazer nelas.

2. **Sonda cada candidata e classifica pelo FORMATO do JSON**, nunca pelo nome
   da rota. Um objeto com um inteiro entre 40 e 99 ao lado de uma string que é
   uma posição de futebol é um jogador — a rota pode se chamar
   `/v3/catalogue/item-index/` que o bot reconhece do mesmo jeito.

   Uma mesma resposta pode servir a dois tipos em profundidades diferentes: o
   fut.gg devolve as evoluções embrulhadas com a carta do jogador
   (`{"data":[{…carta…,"evolution":{…}}]}`). Por fora é uma lista de cartas,
   um nível abaixo é a lista de evoluções, e as duas leituras são guardadas.

3. **Aprende os nomes reais dos campos** no mesmo passe e grava tudo no
   `config.json`: rotas, mapa de campos (`rating → ovr`, `price → bin`) e o
   caminho até a lista (`data`, `results`, ou `data[].evolution` quando o
   recurso está um nível abaixo).

Rotas com parâmetro entram numa segunda passada, usando um id colhido da
listagem de jogadores e o seu gamertag — é assim que a rota do clube é achada.

Se nada aparecer via API, ele cai para o payload que o Next.js embute no HTML.

### O robots.txt do fut.gg bloqueia justamente a API

O `robots.txt` deles tem `Disallow: /api/*` e `Disallow: /gg-club/` — e é
exatamente aí que estão todos os dados. As páginas HTML não servem de
alternativa: são cascas de SPA, e a página de um jogador não traz nem preço
nem atributos.

Por padrão o bot **obedece**, e nesse caso não encontra nada — ele diz isso
com todas as letras, com o número real de rotas que deixou de sondar. Ler
assim mesmo é uma escolha sua, e é explícita:

```bash
./eafcbot autoconfig -ignore-robots
```

Pra não repetir a flag toda vez, o mesmo efeito também dá pra deixar
permanente no `config.json`:

```json
{"futgg": {"respect_robots": false}}
```

(a flag, quando passada, sempre vence o que estiver no config). Mesmo
assim ele continua se identificando pelo User-Agent, respeitando o limite
de requisições e sem tocar em rota de escrita.

### O clube não precisa de login

O fut.gg tem duas portas para o seu elenco:

| rota | precisa de sessão? |
|---|---|
| `/api/gg-club/overview/` | sim — *"You must be logged in"* |
| `/api/gg-club/{gamertag}/players/` | **não** |

A segunda é a que alimenta a sua página pública (`fut.gg/gg-club/<você>/`),
aquela que você manda para os amigos verem o time. É a que o bot usa, e é por
isso que **não existe cookie para colar nem token para renovar**: o único
dado que ela precisa é o `gamer_tag`, que já está no `config.json`.

O elenco vem paginado, 30 por página, e o bot percorre todas — são 647 cartas
num clube real, e ler só a primeira página faria o bot recomendar a compra de
um jogador que você já tem.

Duas coisas que essa rota **não** entrega, porque a Community API da EA não
as expõe a ninguém:

- **Seu saldo de moedas.** Sem ele o orçamento fica zero — mas isso NÃO some
  com as sugestões: uma troca fora do bolso continua na lista, só marcada
  "fora do orçamento" (`analyze.Upgrade.Affordable`), porque
  `IncludeUnaffordable` é o padrão. Preencha `market.extra_budget` no
  `config.json` para essa marcação ficar correta; se a lista de upgrades
  estiver vazia, o motivo é outro — veja o funil na tela de mercado ou no
  relatório, e ajuste `report.min_gain`/`report.allow_unpriced` em vez disso.
- **A escalação titular por posição.** O bot deduz pelos jogadores marcados
  como `isInActiveSquad`.

Os preços não precisam de rota própria: já vêm dentro da listagem de
jogadores (`price`, `currentDbPrice`).

### Ambiguidade não vira chute

Quando dois campos servem igualmente bem e nada desempata — seis inteiros entre
15 e 99 sem nome que ajude são indistinguíveis — o bot **não grava nenhum dos
dois**. O campo cai nos aliases do código, e o relatório do `autoconfig` avisa:

```
players      /v3/catalogue/item-index/
             confiança 87% · 25 amostras · lista em "records"
             aprendeu: rating → ovr, position → pos, price → bin, name → label
             ambíguos (usando os aliases do código): shooting, passing
```

Gravar um palpite ali faria o bot comparar ritmo com desarme e recomendar a
troca errada em silêncio. É o mesmo princípio do requisito de evolução
desconhecido: na dúvida, não afirma.

### Quando quiser olhar por baixo

```bash
./eafcbot autoconfig -dry-run          # mostra sem gravar
./eafcbot autoconfig -dump saida.json  # resultado bruto, com amostras
./eafcbot discover players             # resposta crua de um endpoint já configurado
```

Se algum campo continuar errado, dá para somar o nome novo à lista de aliases
em `internal/futgg/map.go` — o nome aprendido tem prioridade, os aliases são a
reserva:

```go
Rating: n.int(l.k("rating", "overall", "rating", "ovr")...),
```

## Histórico

Por padrão o histórico fica em JSON em `.eafc-bot/`, dentro do próprio
repositório — config, cache, elenco e relatórios ficam todos ali, num só
lugar, e já estão no `.gitignore`. Não é preciso sair da pasta do projeto
para achar nada. Funciona no primeiro dia, sem instalar nada. Para histórico
longo com consulta:

```bash
go get github.com/jackc/pgx/v5
go build -tags postgres ./cmd/eafcbot

export EAFC_DSN="postgres://user:senha@localhost:5432/eafc?sslmode=disable"
psql "$EAFC_DSN" -f migrations/001_init.sql
./eafcbot run
```

O histórico é o que permite as seções "novidades de hoje" e "mercado": sem ele
o bot não sabe o que mudou desde ontem.

## Interface

```bash
./eafcbot serve
```

Sobe a API + a UI React (`web/`, embutida no binário) numa porta só —
`http://localhost:4173` (`-port` muda), abre sozinho no navegador, fica no
ar até `Ctrl+C`. Cinco telas, cada uma lendo só o endpoint que precisa:

| rota | o que mostra |
|---|---|
| `/` | status diário — saldo, nota do elenco, elo mais fraco, o que mudou desde ontem, tendência de 30 dias |
| `/time` | os titulares + reservas, com GG Rating |
| `/time/:slug` | atual x potencial de uma carta — funções por posição (Role++/Role+), teto das evoluções disponíveis, preço no tempo |
| `/mercado` | oportunidades de troca, ordenadas por ganho por moeda gasta |
| `/evolucoes` | quais evoluções ativas valem a pena **no seu elenco** hoje |

Um scheduler embutido coleta sozinho, uma vez por dia, no horário de
`serve.daily_at` (`config.json`, padrão `"05:00"`) — sem cron, sem Agendador
de Tarefas. Na subida, se o último snapshot já passou de `stale_after_hours`
(padrão 20h), ele coleta na hora em vez de esperar o horário; o botão
"atualizar agora" no topo da UI faz o mesmo sob demanda. A análise
carta-a-carta (atual x potencial, o `cards_min_rating` mais caro de buscar —
~1,3 MB por carta contra o fut.gg) roda junto, à noite, para `/time/:slug`
nunca esperar isso na hora do clique.

`serve -demo` sobe as 5 telas com dado fictício, sem rede — útil para
conhecer a interface antes de calibrar o `config.json` (a análise
carta-a-carta não está disponível no demo, por depender do fut.gg de
verdade).

Para mexer na interface com hot-reload:

```bash
./eafcbot serve            # a API de verdade, numa aba
cd web && npm run dev      # o Vite na 5173, proxeando /api pra 4173
```

### Docker

```bash
cp .env.example .env
$EDITOR .env   # preencha EAFC_GAMERTAG
docker compose up -d --build
```

Publica só em `127.0.0.1:4173` (sem autenticação — é a sua máquina),
`restart: unless-stopped` para sobreviver a reboot, e um volume em
`.eafc-bot/` com tudo que o bot grava (config, cache, snapshots,
relatórios). `go build` local continua funcionando igual para desenvolver;
o Docker é só o caminho de "esquece e funciona". Postgres é um profile
opcional — ver os comentários do `docker-compose.yml`.

## Como o bot decide

O ranking **não** usa o overall da carta. O overall da EA premia atributos que
mal aparecem em campo — um zagueiro 89 com 62 de ritmo é inútil em Rivals.
`internal/analyze/roles.go` define, por função, quanto cada atributo pesa de
verdade, mais bônus por PlayStyle relevante (PlayStyle+ vale dobrado) e um
corte de ritmo abaixo do qual a carta é penalizada.

É o arquivo para calibrar com o seu jeito de jogar. Se você joga com posse e
passe curto, suba `pas`/`dri` no meio; se joga em contra-ataque, suba `pac`.

As sugestões saem ordenadas por **ganho por moeda gasta**, não por ganho
bruto: três trocas baratas costumam mudar mais o time que uma cara.

Evolução com requisito que o parser não entendeu é **descartada de
propósito** — sugerir uma evolução impossível é pior que não sugerir nada.

## Agendar

`./eafcbot serve` já tem scheduler embutido (ver "Interface" acima) — é o
caminho recomendado, um processo só, sem depender de cron nem do Agendador
de Tarefas continuarem configurados certos. Cron/Agendador ainda fazem
sentido se você preferir não deixar processo nenhum de pé entre as coletas:

Linux/macOS:

```cron
0 9 * * * cd /caminho/eafc-bot && ./eafcbot run >> .eafc-bot/run.log 2>&1
```

Windows, com o Agendador de Tarefas:

```powershell
schtasks /create /tn "eafc-bot" /tr "C:\Users\gabri\source\repos\eafc-bot\eafcbot.exe run" /sc daily /st 09:00
```

## Ciclo FC 26 → FC 27

O FC 27 sai em **25/09/2026** (early access 18/09, Web App alguns dias antes).
Nada do ciclo é hardcoded: `cycle` no config separa os dados por temporada no
banco, e os endpoints são configuração, não código. Na virada, ajuste
`futgg.cycle` para `"27"` e rode `autoconfig` de novo — ele reaprende as rotas
e os nomes de campo sozinho.

## Testes

```bash
go test ./...
```

Os testes de `internal/analyze` cobrem o que mais importa acertar: que o
zagueiro rápido de 84 vença o lento de 89, que PlayStyle+ desempate, que a
mesma carta renda diferente por função, e que requisito de evolução
desconhecido bloqueie a sugestão.

Os de `internal/discover` sobem um site falso com rotas de nomes inventados
(`/v3/catalogue/item-index/`, `/v3/progression/upgrade-paths/`) e campos
abreviados (`ovr`, `pos`, `bin`, `label`) para provar que a descoberta acha e
classifica sem nenhuma pista de nome — que é exatamente a situação do FC 27.

Os de `internal/scheduler` cobrem o cálculo do próximo disparo com um
relógio injetado (virada de dia, horário já passado, exatamente em cima do
horário) — nada de `time.Sleep` real no teste. Os de `internal/store`
cobrem o round-trip do snapshot e a poda dos 30 dias de retenção. Os de
`internal/api` sobem cada rota via `httptest` contra um snapshot fixo.
