package futgg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Snapshot é tudo que a coleta diária traz do fut.gg de uma vez.
type Snapshot struct {
	Club       domain.Club        `json:"club"`
	Market     []domain.Player    `json:"market"`
	Evolutions []domain.Evolution `json:"evolutions"`
	Objectives []domain.Objective `json:"objectives"`
	SBCs       []domain.SBC       `json:"sbcs"`
	News       []domain.NewsItem  `json:"news"`
	Stats      Stats              `json:"stats"`
	Errors     []string           `json:"errors"`
	// Capabilities é a procedência por fonte desta coleta — fonte, horário,
	// cobertura, avisos, erro e estado (ver Observation). Errors continua
	// existindo como a lista plana que o relatório já imprime; este mapa é o
	// que /api/saude expõe estruturado.
	Capabilities     map[string]Observation       `json:"capabilities"`
	PlayStyleCatalog []domain.PlayStyleDefinition `json:"play_style_catalog,omitempty"`
	RoleCatalog      RolesTable                   `json:"role_catalog,omitempty"`
}

// formationByID traduz o identificador que o GG Club guarda na tática para
// a notação que a UI desenha. O payload público só expõe o id, embora já
// exponha todos os slots; manter a tradução aqui impede que a tela trate um
// XI completo como formação desconhecida.
var formationByID = map[string]string{
	"18": "4-4-1-1",
}

// Collect busca todas as fontes em paralelo. Uma fonte que falha não
// derruba as outras: o relatório sai com o que deu para coletar e diz
// o que faltou, porque um bot diário que não entrega nada quando o site
// muda uma rota é pior que um que entrega 80%.
func (c *Client) Collect(ctx context.Context, gamerTag string, marketFilter PlayerFilter) (*Snapshot, error) {
	// Carrega a tabela de PlayStyles ANTES de disparar as fontes em
	// paralelo: Club, Players e Evolutions dependem dela para traduzir os
	// eaIds numéricos em nome, e chamar aqui uma vez evita que as três
	// disputem a mesma busca ao mesmo tempo (o sync.Once dentro dela já
	// protegeria, mas assim nenhuma delas fica bloqueada esperando a outra).
	c.ensurePlayStyles(ctx)

	snap := &Snapshot{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	capErrs := make(map[string]error, 6)

	fail := func(what string, err error) {
		mu.Lock()
		snap.Errors = append(snap.Errors, fmt.Sprintf("%s: %v", what, err))
		capErrs[what] = err
		mu.Unlock()
	}

	run := func(what string, fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				fail(what, err)
			}
		}()
	}

	if gamerTag != "" {
		run("clube", func() error {
			club, err := c.Club(ctx, gamerTag)
			if err != nil {
				return err
			}
			mu.Lock()
			snap.Club = club
			mu.Unlock()
			return nil
		})
	}

	run("mercado", func() error {
		players, err := c.Players(ctx, marketFilter)
		if err != nil {
			return err
		}
		mu.Lock()
		snap.Market = players
		mu.Unlock()
		return nil
	})

	run("evoluções", func() error {
		evos, err := c.Evolutions(ctx)
		if len(evos) > 0 {
			// A paginação pode entregar dados válidos antes de uma falha
			// posterior. Guardamos o parcial e ainda propagamos o erro para
			// Capabilities/Errors, em vez de transformar a falha em lista vazia.
			mu.Lock()
			snap.Evolutions = evos
			mu.Unlock()
		}
		if err != nil {
			return err
		}
		mu.Lock()
		snap.Evolutions = evos
		mu.Unlock()
		return nil
	})

	run("objetivos", func() error {
		objs, err := c.Objectives(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		snap.Objectives = objs
		mu.Unlock()
		return nil
	})

	run("SBCs", func() error {
		sbcs, err := c.SBCs(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		snap.SBCs = sbcs
		mu.Unlock()
		return nil
	})

	run("notícias", func() error {
		news, err := c.News(ctx)
		if err != nil {
			return err
		}
		mu.Lock()
		snap.News = news
		mu.Unlock()
		return nil
	})

	run("robots", func() error {
		c.checkRobots(ctx)
		return nil
	})

	wg.Wait()
	if len(snap.Club.Players) > 0 && len(snap.Club.Squad.Starters) > 0 {
		if err := c.enrichPositionRatings(ctx, &snap.Club); err != nil {
			snap.Errors = append(snap.Errors, "notas por posição: "+err.Error())
		}
	}
	snap.Stats = c.Stats()
	snap.Capabilities = c.buildCapabilities(gamerTag, capErrs, snap)
	return snap, nil
}

type metarankRow struct {
	EaID     int64   `json:"eaId"`
	Position int     `json:"position"`
	Score    float64 `json:"score"`
}
type metarankResponse struct {
	Data []metarankRow `json:"data"`
}

func (c *Client) enrichPositionRatings(ctx context.Context, club *domain.Club) error {
	positions := map[domain.Position]bool{}
	for _, s := range club.Squad.Starters {
		positions[s.Position] = true
	}
	for pos := range positions {
		var ids []string
		for _, p := range club.Players {
			if p.PlaysAt(pos) {
				ids = append(ids, strconv.FormatInt(p.ID, 10))
			}
		}
		for start := 0; start < len(ids); start += 50 {
			end := start + 50
			if end > len(ids) {
				end = len(ids)
			}
			raw, err := c.URL("metarank", nil)
			if err != nil {
				return err
			}
			u, _ := url.Parse(raw)
			q := u.Query()
			q.Set("ids", strings.Join(ids[start:end], ","))
			q.Set("positions", strconv.Itoa(positionID(pos)))
			u.RawQuery = q.Encode()
			var resp metarankResponse
			if err := c.GetJSON(ctx, u.String(), &resp); err != nil {
				return err
			}
			for _, r := range resp.Data {
				if got, ok := domain.PositionFromID(r.Position); !ok || got != pos {
					continue
				}
				// TODAS as linhas com aquele id, sem break: ter duas cópias da
				// mesma carta no clube é normal no FUT, e parar na primeira
				// deixava a segunda só com a nota escalar — ela chegava mais
				// fraca do que é na hora de escalar (analyze.gauntletValue).
				for i := range club.Players {
					if club.Players[i].ID != r.EaID {
						continue
					}
					if club.Players[i].GGRatings == nil {
						club.Players[i].GGRatings = map[domain.Position]float64{}
					}
					club.Players[i].GGRatings[pos] = r.Score
				}
			}
		}
	}
	return nil
}

func positionID(p domain.Position) int {
	for id := 0; id < 30; id++ {
		if q, ok := domain.PositionFromID(id); ok && q == p {
			return id
		}
	}
	return -1
}

// PlayerFilter delimita quais cartas do mercado interessam. Puxar o
// catálogo inteiro todo dia é desperdício: o bot só precisa das cartas
// que podem entrar no seu time.
type PlayerFilter struct {
	MinRating int
	MaxRating int
	MaxPrice  int
	Positions []domain.Position
	Pages     int // quantas páginas percorrer
	PerPage   int
}

func (f PlayerFilter) query(page int) string {
	q := "?page=" + strconv.Itoa(page)
	if f.PerPage > 0 {
		q += "&limit=" + strconv.Itoa(f.PerPage)
	}
	if f.MinRating > 0 {
		q += "&overall__gte=" + strconv.Itoa(f.MinRating)
	}
	if f.MaxRating > 0 {
		q += "&overall__lte=" + strconv.Itoa(f.MaxRating)
	}
	if f.MaxPrice > 0 {
		// Mandado mesmo sabendo que o fut.gg não filtra por ele hoje —
		// testado ao vivo em 22/08/2026, 82 das 240 cartas de uma coleta com
		// max_price=100000 voltaram acima do teto (a mais cara, 10.000.000).
		// É barato de manter e volta a valer se o site consertar; Players()
		// faz o corte de verdade do lado de cá e conta em
		// Stats.MarketPriceSkipped.
		q += "&price__lte=" + strconv.Itoa(f.MaxPrice)
	}
	for _, p := range f.Positions {
		q += "&position=" + string(p)
	}
	return q
}

// Players percorre as páginas do catálogo em paralelo.
func (c *Client) Players(ctx context.Context, f PlayerFilter) ([]domain.Player, error) {
	if f.Pages <= 0 {
		f.Pages = 5
	}
	base, err := c.URL("players", nil)
	if err != nil {
		return nil, err
	}

	type result struct {
		page    int
		players []domain.Player
		err     error
	}
	results := make(chan result, f.Pages)
	var wg sync.WaitGroup

	for page := 1; page <= f.Pages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			body, err := c.GetRaw(ctx, base+f.query(page))
			if err != nil {
				results <- result{page: page, err: err}
				return
			}
			nodes, err := c.decodeList(body, "players")
			if err != nil {
				results <- result{page: page, err: err}
				return
			}
			out := make([]domain.Player, 0, len(nodes))
			skipped := 0
			for _, n := range nodes {
				p := mapPlayer(n, c.cfg.Cycle, c.lensFor("players"))
				if p.ID == 0 || p.Rating <= 0 {
					continue
				}
				// Carta SEM preço (Coins <= 0) não é descartada aqui: é a
				// que report.AllowUnpriced existe para tratar, e tratá-la
				// como "cara demais" seria um chute na direção errada (ver
				// o comentário de Stats.MarketPriceSkipped).
				if f.MaxPrice > 0 && p.Price.Coins > f.MaxPrice {
					skipped++
					continue
				}
				out = append(out, p)
			}
			if skipped > 0 {
				c.mu.Lock()
				c.stats.MarketPriceSkipped += skipped
				c.mu.Unlock()
			}
			results <- result{page: page, players: out}
		}(page)
	}

	wg.Wait()
	close(results)

	var all []domain.Player
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		all = append(all, r.players...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return dedupe(all), nil
}

// MomentumOptions controla a leitura de Client.Momentum.
type MomentumOptions struct {
	// Hours é a janela que o fut.gg usa pra calcular a queda em relação à
	// própria média — testado ao vivo em 22/08/2026 aceitando 6, 12 ou 24;
	// outro valor pode não ser reconhecido pelo site. Padrão 24.
	Hours int
	// Pages limita quantas páginas ler. O endpoint devolve a base inteira
	// (~6178 cartas, 206 páginas de 30) JÁ ORDENADA por maior desconto
	// primeiro — não precisamos do catálogo inteiro pra achar os melhores
	// candidatos, só o topo. Padrão 5 (150 cartas).
	Pages int
}

// Momentum lê quanto cada carta caiu da própria média recente — sinal que
// o fut.gg já calcula e recalcula a cada poucos minutos do lado deles
// (testado ao vivo em 22/08/2026). O bot não infere tendência a partir do
// próprio histórico de preço, esparso demais pra isso (1 ponto/carta/dia
// na operação normal): lê o resultado já pronto. Mesmo envelope e paginação
// de Players — decodeList/mapPlayer não mudam.
func (c *Client) Momentum(ctx context.Context, opt MomentumOptions) ([]domain.Player, error) {
	if opt.Hours <= 0 {
		opt.Hours = 24
	}
	if opt.Pages <= 0 {
		opt.Pages = 5
	}
	// hours é segmento de path ({hours} no endpoint configurado), não
	// query param — ver o comentário do endpoint "momentum" em client.go.
	base, err := c.URL("momentum", map[string]string{"hours": strconv.Itoa(opt.Hours)})
	if err != nil {
		return nil, err
	}

	type result struct {
		page    int
		players []domain.Player
		err     error
	}
	results := make(chan result, opt.Pages)
	var wg sync.WaitGroup

	for page := 1; page <= opt.Pages; page++ {
		wg.Add(1)
		go func(page int) {
			defer wg.Done()
			q := fmt.Sprintf("?page=%d", page)
			body, err := c.GetRaw(ctx, base+q)
			if err != nil {
				results <- result{page: page, err: err}
				return
			}
			nodes, err := c.decodeList(body, "momentum")
			if err != nil {
				results <- result{page: page, err: err}
				return
			}
			out := make([]domain.Player, 0, len(nodes))
			for _, n := range nodes {
				p := mapPlayer(n, c.cfg.Cycle, c.lensFor("momentum"))
				if p.ID != 0 && p.Rating > 0 {
					out = append(out, p)
				}
			}
			results <- result{page: page, players: out}
		}(page)
	}

	wg.Wait()
	close(results)

	var all []domain.Player
	var firstErr error
	for r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		all = append(all, r.players...)
	}
	if len(all) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return dedupe(all), nil
}

func dedupe(players []domain.Player) []domain.Player {
	seen := make(map[int64]bool, len(players))
	out := players[:0]
	for _, p := range players {
		if seen[p.ID] {
			continue
		}
		seen[p.ID] = true
		out = append(out, p)
	}
	return out
}

// Club lê o clube sincronizado no GG Club.
func (c *Client) Club(ctx context.Context, gamerTag string) (domain.Club, error) {
	// Sem isto, o elenco do GG Club manda PlayStyles como número cru
	// (playstyles:[0,34,5]) e ficam assim pra sempre — mapPlayer resolve o
	// nome NA HORA de ler cada carta, não depois. Collect() já chamava isto
	// antes de despachar pra Club/Players/Evolutions, então o run diário
	// nunca pegou o bug; mas Club() chamado sozinho, fora de Collect(),
	// parseava o elenco com a tabela vazia.
	c.ensurePlayStyles(ctx)

	u, err := c.URL("club", map[string]string{"gamertag": gamerTag})
	if err != nil {
		return domain.Club{}, err
	}

	lc := c.lensFor("club")
	rosterKeys := lc.k("players", "players", "club", "items", "cards", "roster", "data")

	// O elenco vem paginado, 30 por página. Ler só a primeira devolvia 30 de
	// 647 cartas — e um clube truncado faz o bot recomendar a compra de um
	// jogador que você já tem.
	var wrapper node
	var roster []node
	for page := 1; page <= maxClubPages; page++ {
		body, err := c.GetRaw(ctx, withPage(u, page))
		if err != nil {
			if page == 1 {
				if errors.Is(err, ErrNotFound) {
					return domain.Club{}, clubNotFoundError(gamerTag, withPage(u, page))
				}
				return domain.Club{}, err
			}
			break // o que já veio vale mais que nada
		}
		var pn node
		if err := jsonUnmarshalNode(body, &pn); err != nil {
			if page == 1 {
				return domain.Club{}, err
			}
			break
		}
		if page == 1 {
			wrapper = pn
		}
		got := pn.nodes(rosterKeys...)
		roster = append(roster, got...)
		// Sem "next" a resposta não é paginada (ou acabou): os dois casos
		// terminam aqui, e é o que mantém isto compatível com um endpoint
		// de clube que devolva tudo de uma vez.
		if len(got) == 0 || pn.get("next", "nextPage") == nil {
			break
		}
	}
	club := domain.Club{
		GamerTag: gamerTag,
		Platform: wrapper.str(lc.k("platform", "platform", "console")...),
		Coins:    wrapper.int(lc.k("coins", "coins", "balance")...),
		Cycle:    c.cfg.Cycle,
		Source:   "futgg",
	}
	if ts := wrapper.str("syncedAt", "synced_at", "updatedAt"); ts != "" {
		if t, err := parseTime(ts); err == nil {
			club.SyncedAt = t
		}
	}

	// A chave da lista de elenco veio da descoberta quando existe ("roster",
	// por exemplo); os nomes fixos ficam de reserva.
	for _, n := range roster {
		cp := mapClubPlayer(n, c.cfg.Cycle, c.lensFor("players"))
		if cp.ID != 0 {
			club.Players = append(club.Players, cp)
		}
	}

	// A escalação vem de uma rota PRÓPRIA (/active-squad/), separada da
	// listagem de elenco: o /players/ não traz escalação nenhuma, é só
	// cartas. Falha aqui não pode derrubar o clube inteiro — as 647 cartas
	// continuam valendo, o briefing só perde as seções que dependem do XI
	// (nota do time, química, elo mais fraco), do mesmo jeito que Collect()
	// já deixa uma fonte falhar sem arrastar as outras.
	if sq, err := c.ActiveSquad(ctx, gamerTag, club.Players); err == nil {
		club.Squad = sq
	} else {
		// ChemistrySynced fica no zero-value (false) DE PROPÓSITO: sem
		// escalação não existe química nenhuma para comparar, e marcar
		// "sincronizado" aqui faria um zero acidental passar por medição
		// boa (ver Squad.ChemistrySynced). Não "conserte" isto.
		club.Squad = domain.Squad{SyncedAt: club.SyncedAt}
	}
	return club, nil
}

// ActiveSquad lê o XI titular sincronizado no GG Club.
//
// O fut.gg devolve os 11 titulares em "activeGroupPositions" com
// group:"FIELD" (o resto é group:"SUBSTITUTE", que não nos interessa aqui),
// um positionIdx de 0 a 10, e um playerEaId. A posição de cada slot vem de
// "positionOverride" quando o jogador está usando carta de posição
// (positionOverride é o id numérico — a mesma tabela de domain.ParsePosition
// que a listagem de mercado usa); sem carta, cai na posição natural da carta
// no elenco. Não há mapa formação->slot público, então um titular fora de
// posição SEM carta de posição entra pela posição natural — é a aproximação
// possível sem inventar uma tabela por formationId.
func (c *Client) ActiveSquad(ctx context.Context, gamerTag string, roster []domain.ClubPlayer) (domain.Squad, error) {
	u, err := c.URL("club_squad", map[string]string{"gamertag": gamerTag})
	if err != nil {
		return domain.Squad{}, err
	}
	body, err := c.GetRaw(ctx, u)
	if err != nil {
		return domain.Squad{}, err
	}
	var wrapper node
	if err := jsonUnmarshalNode(body, &wrapper); err != nil {
		return domain.Squad{}, err
	}
	// O payload embrulha em "data" DUAS vezes: o de fora carrega o usuário
	// e um "title" que por coincidência repete o de dentro; o conteúdo de
	// verdade — activeGroupPositions, activeFormationId — só existe no
	// "data" de dentro. Confundir os dois faz o título aparecer (o de fora
	// também tem um) e a escalação sumir em silêncio, porque
	// activeGroupPositions no nível errado é só uma lista vazia.
	outer := wrapper.sub("data")
	data := outer.sub("data")
	if len(data) == 0 {
		data = outer // formato mais simples, sem o embrulho duplo: usa direto
	}

	byID := make(map[int64]domain.ClubPlayer, len(roster))
	for _, p := range roster {
		byID[p.ID] = p
	}

	sq := domain.Squad{Name: data.str("title", "name")}
	if id := data.str("activeFormationId"); id != "" {
		sq.Formation = formationByID[id]
	}
	chem := 0
	// Química só vira oráculo se TODOS os titulares resolverem contra o
	// elenco: a soma é por carta, então um titular que não aparece em byID
	// contribui 0 e o total sai menor sem nada indicar isso. Uma soma parcial
	// é pior que número nenhum — ela parece exata (ver Squad.ChemistrySynced).
	todosResolvidos := true
	for _, gp := range data.nodes("activeGroupPositions") {
		if gp.str("group") != "FIELD" {
			continue
		}
		eaID := gp.i64("playerEaId")
		if eaID == 0 {
			continue
		}
		pos := domain.Position("")
		if ov := gp.str("positionOverride"); ov != "" {
			if p, err := domain.ParsePosition(ov); err == nil {
				pos = p
			}
		}
		if p, ok := byID[eaID]; ok {
			if pos == "" {
				pos = p.Position
			}
			chem += p.Chemistry
		} else {
			todosResolvidos = false
		}
		if pos == "" {
			continue // nem carta de posição nem carta no elenco: não dá pra plotar
		}
		sq.Starters = append(sq.Starters, domain.SquadSlot{
			Index: gp.int("positionIdx"), Position: pos, PlayerID: eaID,
		})
	}
	sq.Chemistry = chem
	sq.ChemistrySynced = todosResolvidos && len(sq.Starters) > 0
	return sq, nil
}

func (c *Client) Evolutions(ctx context.Context) ([]domain.Evolution, error) {
	u, err := c.URL("evolutions", nil)
	if err != nil {
		return nil, err
	}

	// O catálogo atual devolve {currentPage,next,totalPages,data:[...]};
	// endpoints antigos continuam devolvendo uma lista única. Percorremos
	// as páginas somente quando a própria resposta confirma que há outra,
	// para não inventar paginação em uma rota legada.
	const maxEvolutionPages = 100
	var out []domain.Evolution
	seen := map[string]bool{}
	lens := c.lensFor("evolutions")
	for page := 1; page <= maxEvolutionPages; page++ {
		body, getErr := c.GetRaw(ctx, withPage(u, page))
		if getErr != nil {
			if page > 1 && len(out) > 0 {
				// Uma falha posterior não apaga as páginas já
				// confirmadas; o snapshot fica parcial e o
				// capability registra a cobertura da coleta.
				return out, fmt.Errorf("página %d: %w", page, getErr)
			}
			return nil, getErr
		}
		nodes, decodeErr := c.decodeList(body, "evolutions")
		if decodeErr != nil {
			if page > 1 && len(out) > 0 {
				return out, fmt.Errorf("página %d: %w", page, decodeErr)
			}
			return nil, decodeErr
		}
		for _, n := range nodes {
			e := mapEvolution(n, c.cfg.Cycle, c.cfg.BaseURL, lens)
			if e.Name == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(e.ID + "\x00" + e.Slug + "\x00" + e.Name))
			if key != "\x00\x00" && seen[key] {
				continue
			}
			if key != "\x00\x00" {
				seen[key] = true
			}
			out = append(out, e)
		}

		var wrapper node
		if err := jsonUnmarshalNode(body, &wrapper); err != nil {
			break // lista pura/legada: não há metadado de página
		}
		totalPages := wrapper.int("totalPages", "total_pages", "pages")
		next := strings.TrimSpace(wrapper.str("next", "nextPage", "next_page"))
		// Algumas respostas usam next=false/0 em vez de omitirem o campo.
		// toStr preserva o literal; tratá-lo como cursor faria a coleta
		// repetir a última página até o teto de segurança.
		if strings.EqualFold(next, "false") || next == "0" || strings.EqualFold(next, "null") {
			next = ""
		}
		hasMore := next != "" || (totalPages > 0 && page < totalPages)
		if len(nodes) == 0 || !hasMore {
			break
		}
		// next pode ser um cursor/URL, mas a API observada usa apenas
		// um indicador booleano. A próxima requisição sempre usa o
		// parâmetro page para manter cache e limites determinísticos.
	}
	return out, nil
}

func (c *Client) Objectives(ctx context.Context) ([]domain.Objective, error) {
	u, err := c.URL("objectives", nil)
	if err != nil {
		return nil, err
	}
	body, err := c.GetRaw(ctx, u)
	if err != nil {
		return nil, err
	}
	nodes, err := c.decodeList(body, "objectives")
	if err != nil {
		return nil, err
	}
	out := make([]domain.Objective, 0, len(nodes))
	for _, n := range nodes {
		lo := c.lensFor("objectives")
		o := domain.Objective{
			ID:    n.str(lo.k("slug", "id", "slug")...),
			Name:  n.str(lo.k("name", "name", "title")...),
			Group: n.str(lo.k("group", "group.name", "groupName", "category", "group")...),
			Tasks: n.strs(lo.k("tasks", "tasks", "objectives", "challenges")...),
			Cycle: c.cfg.Cycle,
		}
		if ts := n.str(lo.k("expires_at", "expiresAt", "endsAt", "endDate")...); ts != "" {
			if t, err := parseTime(ts); err == nil {
				o.ExpiresAt = t
			}
		}
		for _, r := range n.nodes(lo.k("rewards", "rewards", "reward")...) {
			o.Rewards = append(o.Rewards, mapReward(r))
		}
		if o.Name != "" {
			out = append(out, o)
		}
	}
	return out, nil
}

func (c *Client) SBCs(ctx context.Context) ([]domain.SBC, error) {
	u, err := c.URL("sbcs", nil)
	if err != nil {
		return nil, err
	}
	body, err := c.GetRaw(ctx, u)
	if err != nil {
		return nil, err
	}
	nodes, err := c.decodeList(body, "sbcs")
	if err != nil {
		return nil, err
	}
	out := make([]domain.SBC, 0, len(nodes))
	for _, n := range nodes {
		ls := c.lensFor("sbcs")
		s := domain.SBC{
			ID:           n.str(ls.k("slug", "id", "slug")...),
			Name:         n.str(ls.k("name", "name", "title")...),
			Group:        n.str(ls.k("group", "category", "group.name", "group")...),
			Repeatable:   n.bool_(ls.k("repeatable", "repeatable", "isRepeatable")...),
			SolutionCost: n.int(ls.k("solution_cost", "cheapestSolution", "solutionCost", "estimatedCost", "price")...),
			Cycle:        c.cfg.Cycle,
		}
		if ts := n.str(ls.k("expires_at", "expiresAt", "endsAt", "endDate")...); ts != "" {
			if t, err := parseTime(ts); err == nil {
				s.ExpiresAt = t
			}
		}
		for _, r := range n.nodes(ls.k("rewards", "rewards", "reward")...) {
			s.Rewards = append(s.Rewards, mapReward(r))
		}
		// challenges[] é o array que field_maps.sbcs.challenges já aponta
		// pra "challenges" (aprendido pelo autoconfig, ver CLAUDE.md), mas
		// ficava sem leitor — o requisito de cada desafio e o preço da
		// solução mais barata (já resolvida pelo fut.gg) morava só na
		// resposta crua. Sub-campos de challenges[] são fixos (não passam
		// pelo lens de ls): não são campo de topo de "sbcs", é outra forma,
		// mesmo padrão de mapReward abaixo.
		for _, cn := range n.nodes(ls.k("challenges", "challenges")...) {
			s.Challenges = append(s.Challenges, mapSBCChallenge(cn, c.cfg.Platform))
		}
		if s.Name != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

func (c *Client) News(ctx context.Context) ([]domain.NewsItem, error) {
	u, err := c.URL("news", nil)
	if err != nil {
		return nil, err
	}
	body, err := c.GetRaw(ctx, u)
	if err != nil {
		return nil, err
	}
	nodes, err := c.decodeList(body, "news")
	if err != nil {
		return nil, err
	}
	out := make([]domain.NewsItem, 0, len(nodes))
	for _, n := range nodes {
		ln := c.lensFor("news")
		item := domain.NewsItem{
			ID:      n.str(ln.k("slug", "id", "slug")...),
			Title:   n.str(ln.k("title", "title", "headline", "name")...),
			Summary: n.str(ln.k("summary", "summary", "excerpt", "description")...),
			Tags:    n.strs("tags", "categories"),
		}
		if slug := n.str(ln.k("slug", "slug", "path", "url")...); slug != "" {
			item.URL = c.cfg.BaseURL + "/news/" + slug + "/"
		}
		if ts := n.str(ln.k("published_at", "publishedAt", "published_at", "createdAt", "date")...); ts != "" {
			if t, err := parseTime(ts); err == nil {
				item.PublishedAt = t
			}
		}
		if item.Title != "" {
			out = append(out, item)
		}
	}
	return out, nil
}

func mapReward(n node) domain.Reward {
	r := domain.Reward{
		Description: n.str("description", "name", "title", "label"),
		PlayerID:    n.i64("playerId", "eaId", "player.eaId"),
		PackValue:   n.int("packValue", "estimatedValue", "value"),
		Coins:       n.int("coins", "coinReward"),
		Untradeable: n.bool_("untradeable", "isUntradeable"),
	}
	switch {
	case r.PlayerID != 0:
		r.Kind = "player"
	case r.Coins > 0:
		r.Kind = "coins"
	case n.str("type", "kind") != "":
		r.Kind = n.str("type", "kind")
	default:
		r.Kind = "pack"
	}
	return r
}

// mapSBCChallenge lê um desafio dentro de um SBC. O fut.gg publica um
// preço de solução POR PLATAFORMA — visto ao vivo em 22/08/2026 divergindo
// de verdade (uma solução de 39300 no console e 56850 no PC) — plataforma
// vazia ou desconhecida cai no console, que é o valor que a chave
// "cheapestSolutionPrice" (sem sufixo) representa.
func mapSBCChallenge(n node, platform string) domain.SBCChallenge {
	ch := domain.SBCChallenge{
		Name:             n.str("name", "title", "label"),
		RequirementsText: n.strs("requirementsText", "requirements_text"),
	}
	if strings.EqualFold(platform, "pc") {
		ch.CheapestSolutionCoins = n.int("cheapestSolutionPricePc", "cheapest_solution_price_pc")
	} else {
		ch.CheapestSolutionCoins = n.int("cheapestSolutionPrice", "cheapest_solution_price")
	}
	return ch
}

// maxClubPages é um teto de segurança: 40 páginas de 30 são 1200 cartas,
// mais que qualquer clube real, e impede que um "next" que nunca acaba vire
// um laço infinito contra o site.
const maxClubPages = 40

// withPage acrescenta ?page=N. A primeira página vai sem parâmetro nenhum,
// para um endpoint de clube que não pagine continuar funcionando igual.
func withPage(rawURL string, page int) string {
	if page <= 1 {
		return rawURL
	}
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + "page=" + strconv.Itoa(page)
}

// clubNotFoundError explica o 404 do clube em vez de deixá-lo passar cru.
// O engano mais comum é usar a gamertag da EA em vez do nome do perfil no
// fut.gg — são identidades diferentes, e um "404 não encontrado" sozinho não
// dá pista nenhuma disso.
func clubNotFoundError(gamerTag, url string) error {
	return fmt.Errorf(
		"o perfil %q não existe no fut.gg (GET %s -> 404). "+
			"Esse nome NÃO é a sua gamertag da EA: é o nome do perfil no "+
			"fut.gg. Abra https://www.fut.gg/gg-club/ logado e copie o nome "+
			"que aparece na URL (pode colar a URL inteira em \"gamer_tag\" "+
			"no config, ela é normalizada sozinha).",
		gamerTag, url)
}
