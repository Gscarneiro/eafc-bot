package futgg

import (
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Os nomes candidatos abaixo cobrem as convenções que o fut.gg já usou
// (camelCase da API nova, snake_case da antiga, e os nomes que aparecem
// no payload do Next.js). Acrescentar um alias aqui é a forma de
// consertar o parser quando o site muda.

// cdnImageBase é o prefixo que o CDN de imagens do fut.gg exige antes de um
// "*Path" pra virar URL de verdade — testado ao vivo contra o cardImagePath
// de uma carta real. Algumas respostas (a listagem de mercado, por exemplo)
// já mandam a URL pronta em "*Url"; outras (o elenco do GG Club) só mandam
// o caminho relativo em "*Path", e é aí que este prefixo entra.
const cdnImageBase = "https://game-assets.fut.gg/cdn-cgi/image/quality=85,format=auto,width=300/"

// cardImageURL acha a arte da carta em qualquer um dos dois formatos que o
// fut.gg usa.
func cardImageURL(n node) string {
	if u := n.str("cardImageUrl", "card_image_url", "imageUrl", "image_url", "simpleCardImageUrl"); u != "" {
		return u
	}
	if p := n.str("cardImagePath", "card_image_path", "imagePath", "image_path", "simpleCardImagePath"); p != "" {
		return cdnImageBase + p
	}
	return ""
}

func mapPlayer(n node, cycle string, l lens) domain.Player {
	p := domain.Player{
		ID:         n.i64(l.k("id", "eaId", "ea_id", "id", "resourceId", "definitionId")...),
		FutGGSlug:  n.str("slug", "url", "path"),
		Name:       n.str(l.k("name", "name", "fullName", "full_name", "playerName")...),
		CommonName: n.str("commonName", "common_name", "shortName", "short_name", "nickname"),
		Rating:     n.int(l.k("rating", "overall", "rating", "ovr")...),
		Version:    n.str(l.k("version", "rarity.name", "rarityName", "rarity", "version", "cardType")...),
		Club:       n.str(l.k("club", "club.name", "clubName", "club", "team")...),
		League:     n.str(l.k("league", "league.name", "leagueName", "league")...),
		Nation:     n.str(l.k("nation", "nation.name", "nationName", "nation", "country")...),
		WeakFoot:   n.int(l.k("weak_foot", "weakFoot", "weak_foot", "weakFootAbility")...),
		SkillMoves: n.int(l.k("skill_moves", "skillMoves", "skill_moves", "skillMoveLevel")...),
		Foot:       normalizeFoot(n),
		Height:     n.int("height", "heightCm", "height_cm"),
		Cycle:      cycle,
	}

	if pos, err := domain.ParsePosition(n.str(l.k("position", "position.shortLabel", "position", "mainPosition", "preferredPosition")...)); err == nil {
		p.Position = pos
	}
	// As alternativas vêm como rótulo no mercado e como id no elenco do
	// clube; ParsePosition aceita os dois.
	for _, s := range n.strs("alternativePositions", "alternativePositionIds",
		"alternative_positions", "altPositions", "otherPositions", "positions") {
		if alt, err := domain.ParsePosition(s); err == nil && alt != p.Position {
			p.AltPositions = append(p.AltPositions, alt)
		}
	}

	p.Attributes = mapAttributes(n, l, p.Position)
	p.DetailedAttributes = mapDetailedAttributes(n)
	p.PlayStyles = mapPlayStyles(n, l)
	p.Price = mapPrice(n, l)

	// Só o elenco do GG Club manda esses dois; a listagem de mercado não
	// tem. GGRating fica 0 quando ausente — é assim que report.squadSummary
	// sabe que a fonte não trouxe o dado e cai para o Score() próprio.
	//
	// 0 é GK de verdade na tabela de posições (ver domain.PositionFromID),
	// então "campo ausente" e "GK" não podem virar o mesmo caso: sem
	// checar a presença da chave, todo jogador sem GGRatingPos herdaria
	// GK por acidente.
	p.GGRating = n.float64("ggRating", "gg_rating")
	if v, ok := n.lookup("ggRatingPos"); ok && v != nil {
		if pos, ok := domain.PositionFromID(n.int("ggRatingPos", "gg_rating_pos")); ok {
			p.GGRatingPos = pos
		}
	}
	p.ImageURL = cardImageURL(n)
	p.BasePlayerEaID = n.i64("basePlayerEaId", "base_player_ea_id")
	p.BasePlayerSlug = n.str("basePlayerSlug", "base_player_slug")
	if ts := n.str("dateOfBirth", "date_of_birth", "birthDate", "birth_date"); ts != "" {
		if t, err := parseTime(ts); err == nil {
			p.BirthDate = &t
		}
	}
	if v, ok := optionalInt(n, "weight", "weightKg", "weight_kg"); ok {
		p.WeightKg = &v
	}
	if v, ok := optionalBool(n, "isRealFace", "realFace", "real_face"); ok {
		p.RealFace = &v
	}
	p.AccelerateType = n.str("accelerateType", "accelerate_type", "accelerateTypes")
	p.RolesPlus = n.ints("rolesPlus", "roles_plus")
	p.RolesPlusPlus = n.ints("rolesPlusPlus", "roles_plus_plus")

	// Só a rota de momentum (Client.Momentum) manda este campo — testado
	// ao vivo em 22/08/2026 contra /api/fut/players/v2/momentum/?hours=24.
	// Fica 0 pra qualquer outra fonte, mesmo convênio de GGRating.
	p.MomentumPct = n.float64(l.k("momentum_pct", "momentumPercentage", "momentum_percentage")...)

	if ts := n.str("releasedAt", "released_at", "createdAt", "created_at"); ts != "" {
		if t, err := parseTime(ts); err == nil {
			p.ReleasedAt = t
		}
	}
	return p
}

func normalizeFoot(n node) string {
	if s := strings.TrimSpace(n.str("foot", "preferredFoot", "preferred_foot")); s != "" {
		return domain.NormalizeFoot(s)
	}
	if v, ok := optionalInt(n, "foot", "preferredFoot", "preferred_foot"); ok {
		return domain.NormalizeFoot(strconv.Itoa(v))
	}
	return ""
}

func optionalInt(n node, keys ...string) (int, bool) {
	for _, k := range keys {
		if v, ok := n.lookup(k); ok && v != nil {
			if i, ok := toIntOK(v); ok {
				return i, true
			}
		}
	}
	return 0, false
}
func optionalBool(n node, keys ...string) (bool, bool) {
	for _, k := range keys {
		if v, ok := n.lookup(k); ok && v != nil {
			switch x := v.(type) {
			case bool:
				return x, true
			case string:
				b, e := strconv.ParseBool(x)
				return b, e == nil
			case float64:
				return x != 0, true
			}
		}
	}
	return false, false
}

func mapDetailedAttributes(n node) *domain.DetailedAttributes {
	keys := map[string][]string{"Acceleration": {"attributeAcceleration", "acceleration"}, "SprintSpeed": {"attributeSprintSpeed", "sprintSpeed"}, "Agility": {"attributeAgility", "agility"}, "Balance": {"attributeBalance", "balance"}, "Jumping": {"attributeJumping", "jumping"}, "Stamina": {"attributeStamina", "stamina"}, "Strength": {"attributeStrength", "strength"}, "Reactions": {"attributeReactions", "reactions"}, "Aggression": {"attributeAggression", "aggression"}, "Composure": {"attributeComposure", "composure"}, "Interceptions": {"attributeInterceptions", "interceptions"}, "Positioning": {"attributePositioning", "positioning"}, "Vision": {"attributeVision", "vision"}, "BallControl": {"attributeBallControl", "ballControl"}, "Crossing": {"attributeCrossing", "crossing"}, "Dribbling": {"attributeDribbling", "dribbling"}, "Finishing": {"attributeFinishing", "finishing"}, "FKAccuracy": {"attributeFkAccuracy", "fkAccuracy"}, "HeadingAccuracy": {"attributeHeadingAccuracy", "headingAccuracy"}, "LongPassing": {"attributeLongPassing", "longPassing"}, "ShortPassing": {"attributeShortPassing", "shortPassing"}, "DefensiveAwareness": {"attributeDefensiveAwareness", "defensiveAwareness"}, "ShotPower": {"attributeShotPower", "shotPower"}, "LongShots": {"attributeLongShots", "longShots"}, "StandingTackle": {"attributeStandingTackle", "standingTackle"}, "SlidingTackle": {"attributeSlidingTackle", "slidingTackle"}, "Volleys": {"attributeVolleys", "volleys"}, "Curve": {"attributeCurve", "curve"}, "Penalties": {"attributePenalties", "penalties"}, "GKDiving": {"attributeGkDiving", "gkDiving"}, "GKHandling": {"attributeGkHandling", "gkHandling"}, "GKKicking": {"attributeGkKicking", "gkKicking"}, "GKReflexes": {"attributeGkReflexes", "gkReflexes"}, "GKSpeed": {"attributeGkSpeed", "gkSpeed"}, "GKPositioning": {"attributeGkPositioning", "gkPositioning"}}
	d := &domain.DetailedAttributes{}
	found := false
	set := func(dst **int, names []string) {
		if v, ok := optionalInt(n, names...); ok {
			*dst = &v
			found = true
		}
	}
	set(&d.Acceleration, keys["Acceleration"])
	set(&d.SprintSpeed, keys["SprintSpeed"])
	set(&d.Agility, keys["Agility"])
	set(&d.Balance, keys["Balance"])
	set(&d.Jumping, keys["Jumping"])
	set(&d.Stamina, keys["Stamina"])
	set(&d.Strength, keys["Strength"])
	set(&d.Reactions, keys["Reactions"])
	set(&d.Aggression, keys["Aggression"])
	set(&d.Composure, keys["Composure"])
	set(&d.Interceptions, keys["Interceptions"])
	set(&d.Positioning, keys["Positioning"])
	set(&d.Vision, keys["Vision"])
	set(&d.BallControl, keys["BallControl"])
	set(&d.Crossing, keys["Crossing"])
	set(&d.Dribbling, keys["Dribbling"])
	set(&d.Finishing, keys["Finishing"])
	set(&d.FKAccuracy, keys["FKAccuracy"])
	set(&d.HeadingAccuracy, keys["HeadingAccuracy"])
	set(&d.LongPassing, keys["LongPassing"])
	set(&d.ShortPassing, keys["ShortPassing"])
	set(&d.DefensiveAwareness, keys["DefensiveAwareness"])
	set(&d.ShotPower, keys["ShotPower"])
	set(&d.LongShots, keys["LongShots"])
	set(&d.StandingTackle, keys["StandingTackle"])
	set(&d.SlidingTackle, keys["SlidingTackle"])
	set(&d.Volleys, keys["Volleys"])
	set(&d.Curve, keys["Curve"])
	set(&d.Penalties, keys["Penalties"])
	set(&d.GKDiving, keys["GKDiving"])
	set(&d.GKHandling, keys["GKHandling"])
	set(&d.GKKicking, keys["GKKicking"])
	set(&d.GKReflexes, keys["GKReflexes"])
	set(&d.GKSpeed, keys["GKSpeed"])
	set(&d.GKPositioning, keys["GKPositioning"])
	if !found {
		return nil
	}
	return d
}

func mapAttributes(n node, l lens, pos domain.Position) domain.Attributes {
	if pos.IsGK() {
		return mapGoalkeeperAttributes(n)
	}

	// Formato A: campos soltos no objeto do jogador.
	//
	// Os "face*" são os seis números grandes que a carta mostra, e o fut.gg
	// os serve de dois jeitos: aninhados em faceStatsV2 na listagem de
	// mercado, e SOLTOS no elenco do GG Club. Sem os nomes soltos aqui, todo
	// jogador do seu clube chegava com os seis atributos zerados — e o motor
	// de troca comparava carta real com carta vazia.
	a := domain.Attributes{
		Pace:      n.int(l.k("pace", "pace", "facePace", "attributePace", "pac", "gkFaceDiving", "gkDiving", "diving")...),
		Shooting:  n.int(l.k("shooting", "shooting", "faceShooting", "attributeShooting", "sho", "gkFaceHandling", "gkHandling", "handling")...),
		Passing:   n.int(l.k("passing", "passing", "facePassing", "attributePassing", "pas", "gkFaceKicking", "gkKicking", "kicking")...),
		Dribbling: n.int(l.k("dribbling", "dribbling", "faceDribbling", "attributeDribbling", "dri", "gkFaceReflexes", "gkReflexes", "reflexes")...),
		Defending: n.int(l.k("defending", "defending", "faceDefending", "attributeDefending", "def", "gkFaceSpeed", "gkSpeed")...),
		Physical:  n.int(l.k("physical", "physicality", "physical", "facePhysicality", "attributePhysicality", "phy", "gkFacePositioning", "gkPositioning", "positioning")...),
	}
	if a != (domain.Attributes{}) {
		return a
	}

	// Formato B: agrupados em "attributes" ou "stats".
	sub := n.sub("attributes", "stats", "faceStats")
	if len(sub) > 0 {
		return domain.Attributes{
			Pace:      sub.int("pace", "pac", "diving"),
			Shooting:  sub.int("shooting", "sho", "handling"),
			Passing:   sub.int("passing", "pas", "kicking"),
			Dribbling: sub.int("dribbling", "dri", "reflexes"),
			Defending: sub.int("defending", "def", "speed"),
			Physical:  sub.int("physicality", "physical", "phy", "positioning"),
		}
	}

	// Formato C: array ordenado [PAC, SHO, PAS, DRI, DEF, PHY].
	if list, ok := n.get("attributes", "stats").([]any); ok && len(list) >= 6 {
		return domain.Attributes{
			Pace:      toInt(list[0]),
			Shooting:  toInt(list[1]),
			Passing:   toInt(list[2]),
			Dribbling: toInt(list[3]),
			Defending: toInt(list[4]),
			Physical:  toInt(list[5]),
		}
	}
	return a
}

// mapGoalkeeperAttributes preserva no modelo plano a correspondência
// PAC=DIV, SHO=HAN, PAS=KIC, DRI=REF, DEF=SPD e PHY=POS. O payload também
// traz facePace/faceShooting/etc. para goleiros, mas eles são os atributos
// de linha e rotulá-los como atributos de GK no detalhe da carta é errado.
func mapGoalkeeperAttributes(n node) domain.Attributes {
	a := goalkeeperFace(n)
	if a != (domain.Attributes{}) {
		return a
	}

	// Alguns endpoints agrupam a face de GK em attributes, stats ou
	// faceStats. O vocabulário continua sendo o de goleiro, nunca PAC/SHO.
	return goalkeeperFace(n.sub("attributes", "stats", "faceStats"))
}

func goalkeeperFace(n node) domain.Attributes {
	return domain.Attributes{
		Pace:      n.int("gkFaceDiving", "gkDiving", "gk_face_diving", "attributeGkDiving", "attribute_gk_diving", "diving"),
		Shooting:  n.int("gkFaceHandling", "gkHandling", "gk_face_handling", "attributeGkHandling", "attribute_gk_handling", "handling"),
		Passing:   n.int("gkFaceKicking", "gkKicking", "gk_face_kicking", "attributeGkKicking", "attribute_gk_kicking", "kicking"),
		Dribbling: n.int("gkFaceReflexes", "gkReflexes", "gk_face_reflexes", "attributeGkReflexes", "attribute_gk_reflexes", "reflexes"),
		Defending: n.int("gkFaceSpeed", "gkSpeed", "gk_face_speed", "speed"),
		Physical:  n.int("gkFacePositioning", "gkPositioning", "gk_face_positioning", "attributeGkPositioning", "attribute_gk_positioning", "positioning"),
	}
}

// mapPlayStyles lê os PlayStyles da carta. O mercado manda o nome direto
// ("Rapid"); o elenco do GG Club manda o eaId numérico ("5"), e só a tabela
// de /api/fut/playstyles/ (carregada em l.ps) resolve um para o outro. Sem
// tabela ou sem correspondência, o valor bruto é mantido — errar para
// "PlayStyle desconhecido continua sem nome" é melhor que inventar um.
func mapPlayStyles(n node, l lens) []domain.PlayStyle {
	var out []domain.PlayStyle
	seen := map[string]bool{}

	add := func(name string, plus bool) {
		raw := strings.TrimSpace(name)
		name = strings.TrimSpace(strings.TrimSuffix(resolvePlayStyleName(name, l), "+"))
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		ps := domain.PlayStyle{Name: name, Plus: plus}
		if id, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			ps.EAID = &id
		}
		out = append(out, ps)
	}

	// PlayStyles+ vêm em campo separado na maioria dos formatos.
	for _, s := range n.strs("playstylesPlus", "playStylesPlus", "playstyles_plus") {
		add(s, true)
	}
	for _, s := range n.strs("playstyles", "playStyles", "play_styles") {
		add(s, strings.HasSuffix(strings.TrimSpace(s), "+"))
	}
	// Formato objeto: [{"name":"Rapid","level":2}] — level 2 == PlayStyle+.
	for _, ps := range n.nodes("playstyles", "playStyles", "play_styles") {
		add(ps.str("name", "label", "playstyle"), ps.int("level", "tier") >= 2 || ps.bool_("plus", "isPlus"))
	}
	return out
}

// resolvePlayStyleName traduz um eaId numérico para o nome via a tabela de
// /api/fut/playstyles/ (l.ps). Usada tanto pelo elenco (mapPlayStyles) quanto
// pelos upgrades de evolução (mapUpgrade) — os dois recebem eaId no mesmo
// espaço de ids. Um valor que já é nome, ou um eaId sem correspondência na
// tabela, passa direto: nome errado é pior que número.
func resolvePlayStyleName(raw string, l lens) string {
	raw = strings.TrimSpace(raw)
	if id, err := strconv.Atoi(raw); err == nil {
		if name, ok := l.ps[id]; ok && name != "" {
			return name
		}
	}
	return raw
}

func mapPrice(n node, l lens) domain.Price {
	p := domain.Price{
		Coins:     n.int(l.k("price", "price.current", "currentPrice", "price", "priceNum", "lowestBin", "lowest_bin")...),
		IsSBC:     n.bool_("isSbc", "is_sbc", "sbcOnly", "untradeable"),
		UpdatedAt: time.Now(),
	}
	// Extinct só é lido quando a FONTE mandou a chave — checagem de
	// presença, não n.bool_ direto, porque "ausente" e "presente e false"
	// não podem virar o mesmo caso (mesmo padrão de GGRatingPos acima).
	// Antes disto o código inferia Extinct=true de Coins==0, misturando
	// dois sinais diferentes: "sem oferta de verdade" (a demanda passou do
	// teto que a EA define pra faixa — sinal real de mercado, pesquisa de
	// 22/08/2026) e "não conseguimos ler o preço" (falha de raspagem — a
	// listagem de mercado às vezes não renderiza preço no servidor). O
	// segundo já tem tratamento próprio (report.AllowUnpriced,
	// analyze.Upgrade.Unpriced); tratá-lo como "extinto" escondia os dois
	// atrás de uma métrica só — scan de prices_26.json em 22/08/2026 achou
	// 22,5% dos pontos gravados nesse estado contaminado.
	for _, key := range []string{"price.isExtinct", "isExtinct", "extinct"} {
		v, ok := n.lookup(key)
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case bool:
			p.Extinct = t
		case float64:
			p.Extinct = t != 0
		case string:
			p.Extinct, _ = strconv.ParseBool(t)
		}
		break
	}
	if ts := n.str("price.updatedAt", "priceUpdatedAt", "updatedAt"); ts != "" {
		if t, err := parseTime(ts); err == nil {
			p.UpdatedAt = t
		}
	}
	return p
}

func mapClubPlayer(n node, cycle string, l lens) domain.ClubPlayer {
	// O objeto do clube pode embrulhar a carta em "player"/"item". No GG Club
	// do fut.gg a chave é "playerDef": o registro de fora traz o que é SEU
	// (gols, jogos, número da camisa, se está no elenco ativo) e a carta em
	// si mora um nível abaixo.
	inner := n
	if sub := n.sub("playerDef", "player", "item", "card"); len(sub) > 0 {
		inner = sub
	}

	cp := domain.ClubPlayer{
		Player: mapPlayer(inner, cycle, l),
		// O envelope do GG Club traz um id opaco como
		// "gamertag-eaId". Guardá-lo separadamente do Player.ID evita
		// colapsar duas cópias da mesma carta ao comparar snapshots.
		ClubItemID:   n.str("clubItemId", "club_item_id", "itemId", "item_id", "id"),
		Untradeable:  n.bool_("untradeable", "isUntradeable", "untradable"),
		InSquad:      n.bool_("inSquad", "in_squad", "isStarter", "isInActiveSquad", "inActiveSquad"),
		Chemistry:    n.int("chemistryPoints", "chemistry_points", "chemistry"),
		Contracts:    n.int("contracts", "contract"),
		EvosApplied:  n.strs("evolutions", "evos", "appliedEvolutions"),
		EvoExhausted: n.bool_("evolutionComplete", "isEvolved", "evoExhausted", "hasEvolution"),
	}
	if len(cp.EvosApplied) > 0 {
		cp.EvoExhausted = true
	}
	if slot, err := domain.ParsePosition(n.str("squadPosition", "squad_position", "slotPosition", "slot", "preferredPosition")); err == nil {
		cp.SquadSlot = slot
		cp.OutOfPos = !cp.Player.PlaysAt(slot)
	}
	stats := &domain.ClubStats{}
	set := func(dst **int, keys ...string) {
		if v, ok := optionalInt(n, keys...); ok {
			*dst = &v
		}
	}
	set(&stats.Games, "gamesPlayed", "games_played", "games")
	set(&stats.Goals, "goals")
	set(&stats.Assists, "assists")
	set(&stats.YellowCards, "yellowCards", "yellow_cards")
	set(&stats.RedCards, "redCards", "red_cards")
	set(&stats.PurchasedFor, "purchasedFor", "purchased_for")
	set(&stats.KitNumber, "kitNumber", "kit_number")
	if stats.Games != nil || stats.Goals != nil || stats.Assists != nil || stats.YellowCards != nil || stats.RedCards != nil || stats.PurchasedFor != nil || stats.KitNumber != nil {
		cp.Stats = stats
	}
	return cp
}

func mapEvolution(n node, cycle string, baseURL string, l lens) domain.Evolution {
	e := domain.Evolution{
		ID:                       n.str(l.k("slug", "id", "eaId", "evolutionId")...),
		Slug:                     n.str(l.k("slug", "slug", "path")...),
		Name:                     n.str(l.k("name", "name", "title")...),
		Description:              n.str("description", "subtitle", "summary"),
		CoinCost:                 n.int(l.k("coin_cost", "coinCost", "coin_cost", "priceCoins", "costCoins")...),
		PointCost:                n.int(l.k("point_cost", "pointCost", "point_cost", "pointsCost", "pricePoints", "costPoints")...),
		TokenCost:                n.int(l.k("token_cost", "tokenCost", "token_cost", "costTokens", "priceTokens")...),
		EventTokenID:             n.str(l.k("event_token_id", "eventTokenId", "event_token_id", "tokenId")...),
		RepeatabilityCount:       n.int(l.k("repeatability_count", "repeatabilityCount", "repeatability_count", "repeatableCount")...),
		Repeatable:               n.bool_(l.k("repeatable", "isRepeatable", "repeatable")...),
		AllowedPriorEvolutionIDs: n.strs(l.k("allowed_prior_evolution_ids", "allowedPriorEvolutionIds", "allowed_prior_evolution_ids", "priorEvolutionIds")...),
		Cycle:                    cycle,
	}
	if ts := n.str(l.k("expires_at", "expiresAt", "expires_at", "endDate", "endsAt")...); ts != "" {
		if t, err := parseTime(ts); err == nil {
			e.ExpiresAt = t
		}
	}
	if ts := n.str(l.k("end_submission_at", "endSubmissionTime", "end_submission_time", "endSubmissionAt")...); ts != "" {
		if t, err := parseTime(ts); err == nil {
			e.EndSubmissionAt = t
		}
	}
	e.CategoryID = n.str(l.k("category_id", "categoryId", "category_id")...)
	e.CategoryName = n.str(l.k("category_name", "categoryName", "category_name")...)
	e.CategorySlug = n.str(l.k("category_slug", "categorySlug", "category_slug")...)
	e.SBCName = n.str(l.k("sbc_name", "sbcName", "sbc_name")...)
	e.SBCURL = n.str(l.k("sbc_url", "sbcUrl", "sbc_url")...)
	e.SBCSlug = n.str(l.k("sbc_slug", "sbcSlug", "sbc_slug")...)
	e.ObjectiveGroupName = n.str(l.k("objective_group_name", "objectiveGroupName", "objective_group_name")...)
	e.ObjectiveGroupURL = n.str(l.k("objective_group_url", "objectiveGroupUrl", "objective_group_url")...)
	e.ObjectiveGroupSlug = n.str(l.k("objective_group_slug", "objectiveGroupSlug", "objective_group_slug")...)
	e.TimedCustom = n.bool_(l.k("timed_custom", "timedCustom", "timed_custom")...)
	e.CustomUnlockable = n.bool_(l.k("custom_unlockable", "customUnlockable", "custom_unlockable")...)
	e.IsTimed = n.bool_(l.k("is_timed", "isTimed", "is_timed")...)
	e.TotalTrainingTime = n.int(l.k("total_training_time", "totalTrainingTime", "total_training_time")...)
	e.HideFromFilters = n.bool_(l.k("hide_from_filters", "hideFromFilters", "hide_from_filters")...)
	e.ExcludeFromActivePaths = n.bool_(l.k("exclude_from_active_paths", "excludeFromActivePaths", "exclude_from_active_paths")...)
	e.IsGKEvolution = n.bool_(l.k("is_gk_evolution", "isGkEvolution", "is_gk_evolution")...)
	e.IsNotInEvoLab = n.bool_(l.k("is_not_in_evo_lab", "isNotInEvoLab", "is_not_in_evo_lab")...)
	e.DoesNotUpgradePlayer = n.bool_(l.k("does_not_upgrade_player", "doesNotUpgradePlayer", "does_not_upgrade_player")...)
	e.IsRewardEvolution = n.bool_(l.k("is_reward_evolution", "isRewardEvolution", "is_reward_evolution")...)
	if e.Slug != "" {
		e.URL = strings.TrimRight(baseURL, "/") + "/evolutions/" + strings.Trim(e.Slug, "/") + "/"
	}

	for _, rq := range n.nodes(l.k("requirements", "requirements", "entryRequirements", "requirementGroups")...) {
		e.Requirements = append(e.Requirements, mapRequirement(rq))
	}
	for _, lv := range n.nodes(l.k("levels", "levels", "tiers", "stages")...) {
		level := domain.EvoLevel{
			Number:     lv.int("level", "number", "tier", "idx"),
			Objectives: lv.strs("objectives", "challenges", "tasks"),
		}
		for _, up := range lv.nodes("upgrades", "rewards", "boosts") {
			level.Upgrades = append(level.Upgrades, mapUpgrade(up, l))
		}
		e.Levels = append(e.Levels, level)
	}
	return e.ClassifyEvolution()
}

// mapRequirement traduz o requisito para uma forma que o motor consegue
// avaliar sozinho. Requisito não reconhecido vira kind "unknown", e o
// motor trata isso como bloqueante de propósito.
func mapRequirement(n node) domain.EvoRequirement {
	// Formato real do fut.gg: {"label":"Overall","value":"Max. 97"}. Esse
	// par de campos não existia em nenhum dos formatos que o restante desta
	// função tenta (description/type/operator...), e por isso tem um branch
	// próprio em vez de forçar entrada na lógica por chave abaixo.
	if label := n.str("label"); label != "" {
		if got, ok := mapLabeledRequirement(label, n.str("value")); ok {
			got.Raw = label + ": " + n.str("value")
			return got
		}
	}

	raw := n.str("description", "label", "text", "name")
	req := domain.EvoRequirement{Raw: raw}

	key := strings.ToLower(n.str("type", "kind", "attribute", "field"))
	op := strings.ToLower(n.str("operator", "comparison", "op"))
	val := n.int("value", "amount", "threshold", "max", "min")

	maxOp := strings.Contains(op, "max") || strings.Contains(op, "lte") || op == "<=" || op == "<"
	minOp := strings.Contains(op, "min") || strings.Contains(op, "gte") || op == ">=" || op == ">"

	switch {
	case strings.Contains(key, "overall"), strings.Contains(key, "rating"):
		req.IntValue = val
		if minOp {
			req.Kind = "min_overall"
		} else {
			req.Kind = "max_overall"
		}
	case strings.Contains(key, "position"):
		req.Kind = "position"
		req.Strings = n.strs("values", "positions", "value")
	case strings.Contains(key, "rarity"), strings.Contains(key, "version"):
		req.Kind = "rarity"
		req.Strings = n.strs("values", "rarities", "value")
	case strings.Contains(key, "league"):
		req.Kind = "league"
		req.Strings = n.strs("values", "leagues", "value")
	case strings.Contains(key, "nation"), strings.Contains(key, "country"):
		req.Kind = "nation"
		req.Strings = n.strs("values", "nations", "value")
	case strings.Contains(key, "club"), strings.Contains(key, "team"):
		req.Kind = "club"
		req.Strings = n.strs("values", "clubs", "value")
	case strings.Contains(key, "playstyle"):
		req.IntValue = val
		if strings.Contains(key, "plus") {
			req.Kind = "max_playstyles_plus"
		} else {
			req.Kind = "max_playstyles"
		}
	case strings.Contains(key, "skill"):
		req.IntValue = val
		if minOp {
			req.Kind = "min_skill_moves"
		} else {
			req.Kind = "max_skill_moves"
		}
	case strings.Contains(key, "weak"):
		req.Kind, req.IntValue = "max_weak_foot", val
	case strings.Contains(key, "pace"), key == "pac":
		req.Kind, req.IntValue = "max_pace", val
	case strings.Contains(key, "shoot"), key == "sho":
		req.Kind, req.IntValue = "max_shooting", val
	case strings.Contains(key, "pass"), key == "pas":
		req.Kind, req.IntValue = "max_passing", val
	case strings.Contains(key, "dribbl"), key == "dri":
		req.Kind, req.IntValue = "max_dribbling", val
	case strings.Contains(key, "defend"), key == "def":
		req.Kind, req.IntValue = "max_defending", val
	case strings.Contains(key, "physical"), key == "phy":
		req.Kind, req.IntValue = "max_physical", val
	default:
		// Última tentativa: interpretar o texto livre.
		if k, v, ok := parseRequirementText(raw); ok {
			req.Kind, req.IntValue = k, v
		} else {
			req.Kind = "unknown"
		}
	}
	_ = maxOp
	return req
}

// mapLabeledRequirement traduz o par label/value que o fut.gg usa hoje em
// requirementsText: {"label":"Overall","value":"Max. 97"},
// {"label":"Position","value":"ST, LW, RW"}, {"label":"Excluded
// Position","value":"CB"}, {"label":"Rarity","value":"FUTTIES, ..."},
// {"label":"Max PS+","value":"4"}, {"label":"Min PS+","value":"4"}.
//
// "Max Pos." fica de propósito sem tradução: contra os 30 requisitos reais
// vistos, não deu para confirmar se é "máximo de posições alternativas" ou
// outra coisa, e meets() já trata Kind vazio como bloqueante — um requisito
// mal interpretado recomendaria uma evolução impossível, o que é pior do
// que descartar a evolução.
func mapLabeledRequirement(label, value string) (domain.EvoRequirement, bool) {
	v := strings.TrimSpace(value)
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "overall":
		kind := "max_overall"
		if strings.Contains(strings.ToLower(v), "min") {
			kind = "min_overall"
		}
		return domain.EvoRequirement{Kind: kind, IntValue: extractInt(v)}, true
	case "max ps":
		return domain.EvoRequirement{Kind: "max_playstyles", IntValue: extractInt(v)}, true
	case "max ps+":
		return domain.EvoRequirement{Kind: "max_playstyles_plus", IntValue: extractInt(v)}, true
	case "min ps+":
		return domain.EvoRequirement{Kind: "min_playstyles_plus", IntValue: extractInt(v)}, true
	case "position":
		return domain.EvoRequirement{Kind: "position", Strings: splitList(v)}, true
	case "excluded position":
		return domain.EvoRequirement{Kind: "excluded_position", Strings: splitList(v)}, true
	case "rarity":
		return domain.EvoRequirement{Kind: "rarity", Strings: splitList(v)}, true
	}
	return domain.EvoRequirement{}, false
}

// splitList separa "ST, LW, RW" em três valores, descartando vazios.
func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseRequirementText lê requisitos escritos em inglês, do tipo
// "Max Overall: 84" ou "Min. Skill Moves: 4".
func parseRequirementText(raw string) (kind string, value int, ok bool) {
	l := strings.ToLower(raw)
	if l == "" {
		return "", 0, false
	}
	num := extractInt(l)
	isMax := strings.Contains(l, "max")
	isMin := strings.Contains(l, "min")

	switch {
	case strings.Contains(l, "overall"), strings.Contains(l, "rating"):
		if isMin {
			return "min_overall", num, true
		}
		if isMax {
			return "max_overall", num, true
		}
	case strings.Contains(l, "playstyle"):
		if strings.Contains(l, "plus") || strings.Contains(l, "+") {
			return "max_playstyles_plus", num, true
		}
		return "max_playstyles", num, true
	case strings.Contains(l, "skill move"):
		if isMin {
			return "min_skill_moves", num, true
		}
		return "max_skill_moves", num, true
	case strings.Contains(l, "weak foot"):
		return "max_weak_foot", num, true
	case strings.Contains(l, "pace"):
		return "max_pace", num, true
	}
	return "", 0, false
}

func extractInt(s string) int {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			if start < 0 {
				start = i
			}
		} else if start >= 0 {
			return parseCoinString(s[start:i])
		}
	}
	if start >= 0 {
		return parseCoinString(s[start:])
	}
	return 0
}

// mapUpgrade traduz um ganho de nível de evolução. O formato real do fut.gg
// usa a chave "upgrade" (não "type"/"kind"/"attribute"/"field", que cobrem
// só formatos antigos) com valores como "face_shooting", "play_style_plus",
// "attribute_short_passing" e "overall".
func mapUpgrade(n node, l lens) domain.EvoUpgrade {
	raw := n.str("description", "label", "text", "name")
	up := domain.EvoUpgrade{Raw: raw}
	key := strings.ToLower(n.str("type", "kind", "attribute", "field", "upgrade"))
	amount := n.int("value", "amount", "increase", "boost")
	// maxValue é o teto que o próprio fut.gg declara ("face_shooting +10
	// maxValue 96"): sem ele, um jogador que já está em 90 de finalização
	// ganharia +10 = 100, quando na prática a EA trava em 96. Domain.Apply
	// usa este campo para não superestimar a carta final.
	up.MaxValue = n.int("maxValue", "max_value", "cap")

	switch {
	case strings.Contains(key, "overall"), strings.Contains(key, "rating"):
		up.Kind, up.Amount = "overall", amount
	case strings.Contains(key, "play_style"), strings.Contains(key, "playstyle"):
		up.Kind = "playstyle"
		// "value" carrega o eaId numérico no formato novo, e o nome
		// legível no formato antigo — resolvePlayStyleName aceita os dois.
		name := resolvePlayStyleName(n.str("playstyle", "name", "value"), l)
		up.PlayStyle = domain.PlayStyle{
			Name: strings.TrimSuffix(name, "+"),
			Plus: strings.Contains(key, "plus") || strings.HasSuffix(name, "+") ||
				n.int("level", "tier") >= 2 || n.bool_("plus"),
		}
	case strings.Contains(key, "position"):
		up.Kind = "position"
		if pos, err := domain.ParsePosition(n.str("position", "value", "name")); err == nil {
			up.Position = pos
		}
	case strings.Contains(key, "skill"):
		up.Kind, up.Amount = "skill_moves", amount
	case strings.Contains(key, "weak"):
		up.Kind, up.Amount = "weak_foot", amount
	case strings.HasPrefix(key, "attribute_"):
		// Um dos 35 sub-atributos (attribute_short_passing,
		// attribute_sprint_speed...), não um dos 6 da carta. O payload traz
		// os dois lado a lado para o MESMO nível — "face_passing" e
		// "attribute_short_passing" no mesmo upgrade — e os dois batem no
		// attrKey() por substring ("pass" está em ambos). Contá-los juntos
		// dobraria o ganho de passe. Este case tem que vir ANTES do
		// default/attrKey, e captura só o prefixo "attribute_", deixando
		// "face_*" cair no caminho normal logo abaixo.
		// Mantemos Kind="ignored" por compatibilidade com snapshots e
		// clientes antigos; Attr agora preserva o subatributo para a
		// projeção detalhada (Apply trata ambos os nomes).
		up.Kind = "ignored"
		up.Attr = detailedAttrKey(strings.TrimPrefix(key, "attribute_"))
		if up.Attr == "" {
			up.Kind = "unknown"
		}
	default:
		if attr := attrKey(key); attr != "" {
			up.Kind, up.Attr, up.Amount = "attribute", attr, amount
		} else {
			up.Kind = "unknown"
		}
	}
	return up
}

// detailedAttrKey normaliza o nome explícito que o fut.gg usa no upgrade
// para a chave estável de domain.DetailedAttributes. O mapa é deliberado:
// um campo novo não pode cair em outro subatributo por substring.
func detailedAttrKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.ReplaceAll(key, " ", "_")
	switch key {
	case "acceleration", "sprint_speed", "agility", "balance", "jumping", "stamina", "strength", "reactions", "aggression", "composure", "interceptions", "positioning", "vision", "ball_control", "crossing", "dribbling", "finishing", "fk_accuracy", "heading_accuracy", "long_passing", "short_passing", "defensive_awareness", "shot_power", "long_shots", "standing_tackle", "sliding_tackle", "volleys", "curve", "penalties":
		return key
	case "gk_diving", "gk_handling", "gk_kicking", "gk_reflexes", "gk_speed", "gk_positioning":
		return key
	case "fkaccuracy":
		return "fk_accuracy"
	case "headingaccuracy":
		return "heading_accuracy"
	case "longpassing":
		return "long_passing"
	case "shortpassing":
		return "short_passing"
	case "ballcontrol":
		return "ball_control"
	case "defensiveawareness":
		return "defensive_awareness"
	case "shotpower":
		return "shot_power"
	case "longshots":
		return "long_shots"
	case "standingtackle":
		return "standing_tackle"
	case "slidingtackle":
		return "sliding_tackle"
	case "gkdiving":
		return "gk_diving"
	case "gkhandling":
		return "gk_handling"
	case "gkkicking":
		return "gk_kicking"
	case "gkreflexes":
		return "gk_reflexes"
	case "gkspeed":
		return "gk_speed"
	case "gkpositioning":
		return "gk_positioning"
	}
	return ""
}

func attrKey(key string) string {
	switch {
	case strings.Contains(key, "pace"), key == "pac":
		return "pac"
	case strings.Contains(key, "shoot"), key == "sho":
		return "sho"
	case strings.Contains(key, "pass"), key == "pas":
		return "pas"
	case strings.Contains(key, "dribbl"), key == "dri":
		return "dri"
	case strings.Contains(key, "defend"), key == "def":
		return "def"
	case strings.Contains(key, "physic"), key == "phy":
		return "phy"
	}
	return ""
}

func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339, time.RFC3339Nano,
		"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
	}
	var err error
	for _, f := range formats {
		var t time.Time
		if t, err = time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	// Epoch em segundos ou milissegundos.
	if n := parseCoinString(s); n > 1_000_000_000 {
		if n > 10_000_000_000 {
			return time.UnixMilli(int64(n)), nil
		}
		return time.Unix(int64(n), 0), nil
	}
	return time.Time{}, err
}
