package analyze

import (
	"embed"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

//go:embed profiles/*.json
var profileFiles embed.FS

const versaoMotorAvaliacao = "avaliador_detalhado_v2"

// Avaliador é a única entrada para uma nota ativa. Os analisadores recebem
// esta interface, portanto não precisam saber se a escala veio do FUT.GG ou
// de um perfil local aprovado.
type Avaliador interface {
	Avaliar(domain.Player, domain.Position, domain.ContextoAvaliacao) domain.AvaliacaoCarta
	Perfis() []domain.PerfilMeta
}

type perfilArquivo struct {
	ID                    string                              `json:"id"`
	Versao                string                              `json:"versao"`
	Nome                  string                              `json:"nome"`
	Status                string                              `json:"status"`
	Descricao             string                              `json:"descricao"`
	Plataformas           []string                            `json:"plataformas"`
	Pesos                 map[string]map[string]float64       `json:"pesos"`
	PlayStyles            map[string]float64                  `json:"playstyles"`
	PernaFraca            float64                             `json:"perna_fraca"`
	DribleEstrelas        float64                             `json:"drible_estrelas"`
	QuimicaPorPonto       float64                             `json:"quimica_por_ponto"`
	PenalidadeForaPos     float64                             `json:"penalidade_fora_posicao"`
	PeDominanteNoCorredor float64                             `json:"pe_dominante_no_corredor"`
	FamiliaridadeFuncao   perfilFamiliaridadeFuncaoArquivo    `json:"familiaridade_funcao"`
	PorteFisico           map[string]perfilPorteFisicoArquivo `json:"porte_fisico"`
}

type perfilFamiliaridadeFuncaoArquivo struct {
	Plus     float64 `json:"plus"`
	PlusPlus float64 `json:"plus_plus"`
}

// perfilPorteFisicoArquivo deixa altura, peso, tipo corporal e AcceleRATE
// versionados no perfil: estes dados só têm significado no meta declarado.
type perfilPorteFisicoArquivo struct {
	AlturaIdeal           int                `json:"altura_ideal"`
	PesoIdeal             int                `json:"peso_ideal"`
	PenalidadeAlturaPorCM float64            `json:"penalidade_altura_por_cm"`
	PenalidadePesoPorKG   float64            `json:"penalidade_peso_por_kg"`
	Limite                float64            `json:"limite"`
	TiposCorpo            map[string]float64 `json:"tipos_corpo"`
	AceleraRATE           map[string]float64 `json:"accelerate_rate"`
}

// RegistroAvaliadores carrega os perfis versionados compilados junto do
// binário. O cálculo não depende de IA, rede nem de uma conta externa.
type RegistroAvaliadores struct {
	perfis  map[string]perfilArquivo
	estilos *RegistroEstilosEntrosamento
}

func NovoRegistroAvaliadores() (*RegistroAvaliadores, error) {
	entries, err := profileFiles.ReadDir("profiles")
	if err != nil {
		return nil, fmt.Errorf("lendo perfis de avaliação: %w", err)
	}
	estilos, err := novoRegistroEstilosEntrosamento()
	if err != nil {
		return nil, err
	}
	r := &RegistroAvaliadores{perfis: make(map[string]perfilArquivo, len(entries)), estilos: estilos}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := profileFiles.ReadFile("profiles/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("lendo perfil %s: %w", entry.Name(), err)
		}
		var p perfilArquivo
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, fmt.Errorf("decodificando perfil %s: %w", entry.Name(), err)
		}
		if err := validarPerfil(p); err != nil {
			return nil, fmt.Errorf("perfil %s: %w", entry.Name(), err)
		}
		r.perfis[p.ID] = p
	}
	if len(r.perfis) == 0 {
		return nil, fmt.Errorf("nenhum perfil de avaliação foi encontrado")
	}
	return r, nil
}

func validarPerfil(p perfilArquivo) error {
	if p.ID == "" || p.Versao == "" || p.Nome == "" || len(p.Pesos) == 0 {
		return fmt.Errorf("id, versão, nome e pesos são obrigatórios")
	}
	for _, grupo := range []string{"goleiro", "defensor", "lateral", "meio", "ataque"} {
		weights := p.Pesos[grupo]
		if len(weights) == 0 {
			return fmt.Errorf("grupo %s sem pesos", grupo)
		}
		total := 0.0
		for attr, w := range weights {
			if _, ok := detailedAttributeNames[attr]; !ok || w < 0 {
				return fmt.Errorf("peso inválido de %s", attr)
			}
			total += w
		}
		if total <= 0 {
			return fmt.Errorf("pesos de %s não têm massa", grupo)
		}
		porte, ok := p.PorteFisico[grupo]
		if !ok || porte.AlturaIdeal <= 0 || porte.PesoIdeal <= 0 || porte.Limite < 0 ||
			porte.PenalidadeAlturaPorCM < 0 || porte.PenalidadePesoPorKG < 0 ||
			len(porte.TiposCorpo) == 0 || len(porte.AceleraRATE) == 0 {
			return fmt.Errorf("perfil de porte fisico incompleto para %s", grupo)
		}
	}
	return nil
}

func (r *RegistroAvaliadores) Perfis() []domain.PerfilMeta {
	out := make([]domain.PerfilMeta, 0, len(r.perfis))
	for _, p := range r.perfis {
		out = append(out, domain.PerfilMeta{ID: p.ID, Versao: p.Versao, Nome: p.Nome, Status: p.Status, Descricao: p.Descricao, Plataformas: append([]string(nil), p.Plataformas...)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nome < out[j].Nome })
	return out
}

func (r *RegistroAvaliadores) Avaliar(card domain.Player, pos domain.Position, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	if ctx.Fonte == "" || ctx.Fonte == domain.FonteFutGG {
		return avaliarFutGG(card, pos, ctx)
	}
	if ctx.Fonte == domain.FonteFUTBIN || ctx.Fonte == domain.FonteFUTWIZ {
		return avaliarFonteExterna(card, pos, ctx)
	}
	if ctx.Fonte != domain.FonteBot {
		return domain.AvaliacaoCarta{Contexto: ctx, Motivo: fmt.Sprintf("fonte de avaliação %q ainda não está disponível", ctx.Fonte)}
	}
	profileID := ctx.Perfil
	if profileID == "" {
		profileID = "meta_competitivo"
	}
	p, ok := r.perfis[profileID]
	if !ok {
		return domain.AvaliacaoCarta{Contexto: ctx, Motivo: fmt.Sprintf("perfil %q não foi encontrado", profileID)}
	}
	ctx.Perfil, ctx.VersaoPerfil, ctx.VersaoMotor, ctx.Fonte = p.ID, p.Versao, versaoMotorAvaliacao, domain.FonteBot
	if ctx.Ciclo == "" {
		ctx.Ciclo = card.Cycle
	}
	return r.avaliarPerfil(card, pos, ctx, p)
}

func avaliarFonteExterna(card domain.Player, pos domain.Position, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	if ctx.Ciclo == "" {
		ctx.Ciclo = card.Cycle
	}
	nota, found := card.ExternalRatings[ctx.Fonte]
	if !found {
		return domain.AvaliacaoCarta{Contexto: ctx, Motivo: string(ctx.Fonte) + " não publicou uma nota importada para esta carta", Cobertura: []string{"nota externa ausente"}}
	}
	value, available := nota.RatingAt(pos, ctx)
	if !available {
		return domain.AvaliacaoCarta{Contexto: ctx, Motivo: string(ctx.Fonte) + " não confirmou uma nota compatível para esta vaga e contexto", Cobertura: []string{"nota externa sem cobertura compatível"}}
	}
	return domain.AvaliacaoCarta{Disponivel: true, Nota: value, Contexto: ctx,
		Cobertura:   []string{"nota posicional " + string(ctx.Fonte), "métrica: " + nota.Metrica, "escala declarada pela fonte"},
		Componentes: []domain.ComponenteAvaliacao{{Chave: string(ctx.Fonte) + "_rating", Rotulo: string(ctx.Fonte) + " " + nota.Metrica, Valor: value}},
	}
}

func avaliarFutGG(card domain.Player, pos domain.Position, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	ctx.Fonte = domain.FonteFutGG
	if ctx.Ciclo == "" {
		ctx.Ciclo = card.Cycle
	}
	ratingPos := card.GGRatingPos
	if ratingPos == "" {
		ratingPos = card.Position
	}
	if card.GGRating > 0 && ratingPos == pos {
		return domain.AvaliacaoCarta{Disponivel: true, Nota: card.GGRating, Contexto: ctx,
			Cobertura:   []string{"GG Rating da própria carta publicado pelo FUT.GG"},
			Componentes: []domain.ComponenteAvaliacao{{Chave: "gg_rating_carta", Rotulo: "GG Rating da carta", Valor: card.GGRating}},
		}
	}
	return domain.AvaliacaoCarta{Contexto: ctx, Motivo: "GG Rating do FUT.GG ausente para esta carta nesta posição", Cobertura: []string{"GG Rating da carta sem cobertura nesta vaga"}}
}

// motivoAvaliacaoIndisponivel mantém a lacuna na fonte rastreável até a vaga
// física e a carta afetadas. Um resumo genérico não permite ao usuário saber
// se deve sincronizar o XI, aguardar a fonte ou simplesmente trocar de vaga.
func motivoAvaliacaoIndisponivel(slot domain.SquadSlot, player domain.ClubPlayer, avaliacao domain.AvaliacaoCarta) string {
	motivo := avaliacao.Motivo
	if motivo == "" {
		motivo = "a fonte ativa não confirmou nota para esta vaga"
	}
	return fmt.Sprintf("%s %s: %s", slot.Position, player.Display(), motivo)
}

func (r *RegistroAvaliadores) avaliarPerfil(card domain.Player, pos domain.Position, ctx domain.ContextoAvaliacao, profile perfilArquivo) domain.AvaliacaoCarta {
	out := domain.AvaliacaoCarta{Contexto: ctx, Cobertura: []string{"subatributos", "PlayStyles", "posição", "química contextual"}}
	if card.DetailedAttributes == nil {
		out.Motivo = "subatributos essenciais ausentes; o perfil não usa atributos resumidos como substituto"
		out.DadosAusentes = []string{"detailed_attributes"}
		return out
	}
	grupo := grupoPosicao(pos)
	weights := profile.Pesos[grupo]
	styleGains, styleLimitation := r.estilos.incrementos(ctx)
	if styleLimitation != "" {
		out.Parcial, out.Limitacoes = true, append(out.Limitacoes, styleLimitation)
	}
	missing := make([]string, 0)
	base := 0.0
	weightTotal := 0.0
	styleContribution := 0.0
	for _, attr := range sortedWeightKeys(weights) {
		v, ok := detailedValue(card.DetailedAttributes, attr)
		if !ok {
			missing = append(missing, attr)
			continue
		}
		boosted := v + styleGains[attr]
		if boosted > 99 {
			boosted = 99
		}
		base += float64(boosted) * weights[attr]
		styleContribution += float64(boosted-v) * weights[attr]
		weightTotal += weights[attr]
	}
	// Esses são os atributos sem os quais a função não existe em campo. Uma
	// avaliação parcial não pode inventar o valor de um fundamento principal.
	if len(missing) > 0 {
		out.Motivo = "subatributos exigidos pelo perfil estão ausentes"
		out.DadosAusentes = missing
		return out
	}
	base /= weightTotal
	styleContribution /= weightTotal
	out.Disponivel = true
	out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "subatributos", Rotulo: "subatributos da função", Valor: round1(base)})
	if len(styleGains) > 0 {
		coverage := "estilo de entrosamento confirmado: " + ctx.EstiloEntrosamento + " (" + strings.Join(nomesIncrementos(styleGains), ", ") + ")"
		out.Cobertura = append(out.Cobertura, coverage)
		if styleContribution != 0 {
			out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "estilo_entrosamento", Rotulo: "estilo de entrosamento " + ctx.EstiloEntrosamento, Valor: round1(styleContribution)})
		}
	}
	total := base

	traitBonus := 0.0
	for _, ps := range card.PlayStyles {
		if weight, ok := profile.PlayStyles[ps.Name]; ok {
			v := weight
			if ps.Plus {
				v *= 2
			}
			traitBonus += v
			out.PontosFortes = append(out.PontosFortes, ps.String())
		}
	}
	traitBonus = math.Min(traitBonus, 6)
	if traitBonus > 0 {
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "playstyles", Rotulo: "PlayStyles da função", Valor: round1(traitBonus)})
		total += traitBonus
	}
	if card.WeakFoot > 0 {
		v := float64(card.WeakFoot-3) * profile.PernaFraca
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "perna_fraca", Rotulo: "perna fraca", Valor: round1(v)})
		total += v
	} else {
		out.Parcial, out.DadosAusentes = true, append(out.DadosAusentes, "weak_foot")
	}
	if card.SkillMoves > 0 && (grupo == "ataque" || grupo == "meio" || grupo == "lateral") {
		v := float64(card.SkillMoves-3) * profile.DribleEstrelas
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "drible", Rotulo: "estrelas de drible", Valor: round1(v)})
		total += v
	} else if card.SkillMoves == 0 {
		out.Parcial, out.DadosAusentes = true, append(out.DadosAusentes, "skill_moves")
	}
	if ctx.Quimica != nil {
		chem := *ctx.Quimica
		if chem < 0 {
			chem = 0
		}
		if chem > 3 {
			chem = 3
		}
		v := float64(chem-3) * profile.QuimicaPorPonto
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "quimica", Rotulo: "química simulada", Valor: round1(v)})
		total += v
	} else {
		out.Parcial, out.DadosAusentes = true, append(out.DadosAusentes, "química do contexto")
	}
	porte, faltantesPorte := avaliarPorteFisico(card, profile.PorteFisico[grupo])
	if len(faltantesPorte) > 0 {
		out.Parcial = true
		out.DadosAusentes = append(out.DadosAusentes, faltantesPorte...)
	} else {
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "porte_fisico", Rotulo: "altura, peso e tipo corporal", Valor: round1(porte)})
		total += porte
	}
	if bonus, ok := valorNormalizado(profile.PorteFisico[grupo].AceleraRATE, card.AccelerateType); ok {
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "acelerate_rate", Rotulo: "AcceleRATE", Valor: round1(bonus)})
		total += bonus
	} else {
		out.Parcial, out.DadosAusentes = true, append(out.DadosAusentes, "AcceleRATE")
	}
	if bonus, relevante, faltando := avaliarPeDominante(card.Foot, pos, profile.PeDominanteNoCorredor); relevante {
		if faltando {
			out.Parcial, out.DadosAusentes = true, append(out.DadosAusentes, "pe dominante")
		} else {
			out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "pe_dominante", Rotulo: "pe dominante no corredor", Valor: round1(bonus)})
			total += bonus
		}
	}
	if ctx.Funcao != "" {
		if role, ok := card.FamiliaridadeNaFuncao(pos, ctx.Funcao); ok {
			bonus := profile.FamiliaridadeFuncao.Plus
			if role.Nivel == "plus_plus" {
				bonus = profile.FamiliaridadeFuncao.PlusPlus
			}
			out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "familiaridade_funcao", Rotulo: "familiaridade " + role.Nome, Valor: round1(bonus)})
			total += bonus
		} else if card.FamiliaridadesFuncao == nil {
			out.Parcial, out.DadosAusentes = true, append(out.DadosAusentes, "catalogo de funcoes")
		} else {
			out.Limitacoes = append(out.Limitacoes, "sem familiaridade confirmada para a funcao "+ctx.Funcao)
		}
	}
	if !card.PlaysAt(pos) {
		out.Componentes = append(out.Componentes, domain.ComponenteAvaliacao{Chave: "fora_posicao", Rotulo: "fora de posição", Valor: -profile.PenalidadeForaPos})
		out.Limitacoes = append(out.Limitacoes, "fora de posição")
		total -= profile.PenalidadeForaPos
	}
	if ctx.Patch == "" || ctx.Patch == "não_validado" {
		out.Parcial, out.Limitacoes = true, append(out.Limitacoes, "patch sem validação independente")
	}
	if !contains(profile.Plataformas, strings.ToLower(ctx.Plataforma)) && ctx.Plataforma != "" {
		out.Parcial, out.Limitacoes = true, append(out.Limitacoes, "plataforma fora da cobertura do perfil")
	}
	if profile.Status != "aprovado" {
		out.Parcial, out.Limitacoes = true, append(out.Limitacoes, "perfil experimental: faltam evidências independentes")
	}
	if total < 0 {
		total = 0
	}
	if total > 100 {
		total = 100
	}
	out.Nota = round1(total)
	if len(out.PontosFortes) > 3 {
		out.PontosFortes = out.PontosFortes[:3]
	}
	if len(out.PontosFortes) == 0 {
		out.PontosFortes = []string{"sem PlayStyle relevante confirmado"}
	}
	return out
}

func (r *RegistroAvaliadores) Perfil(id string) (domain.PerfilMeta, bool) {
	p, ok := r.perfis[id]
	return domain.PerfilMeta{ID: p.ID, Versao: p.Versao, Nome: p.Nome, Status: p.Status, Descricao: p.Descricao, Plataformas: append([]string(nil), p.Plataformas...)}, ok
}

// ClubeNaRegua traduz o retrato para uma régua única antes de algoritmos que
// historicamente liam GGRatingAt. Não há fallback: se o perfil não consegue
// avaliar uma carta, a nota posicional é removida e a análise informa a
// cobertura faltante em vez de misturar fontes na mesma escala.
func ClubeNaRegua(club domain.Club, evaluator Avaliador, ctx domain.ContextoAvaliacao) domain.Club {
	if evaluator == nil {
		return club
	}
	out := club
	out.Players = append([]domain.ClubPlayer(nil), club.Players...)
	for i := range out.Players {
		player := &out.Players[i]
		original := player.Player
		player.GGRating = 0
		player.GGRatingPos = ""
		player.GGRatings = nil
		player.NotasNaRegua = make(map[domain.Position]float64)
		positions := append([]domain.Position{player.Position}, player.AltPositions...)
		for _, pos := range positions {
			cardCtx := ctx
			if player.Chemistry >= 0 {
				chem := player.Chemistry
				cardCtx.Quimica = &chem
			}
			result := evaluator.Avaliar(original, pos, cardCtx)
			if !result.Disponivel {
				continue
			}
			player.NotasNaRegua[pos] = result.Nota
			if result.Nota > player.GGRating {
				player.GGRating, player.GGRatingPos = result.Nota, pos
			}
		}
	}
	return out
}

// avaliarOpcional conserva a API histórica enquanto cada chamador migra. Ao
// receber um avaliador, porém, não recorre ao Score antigo: ausência continua
// ausência e não vira uma escala diferente no meio do ranking.
func avaliarOpcional(card domain.Player, pos domain.Position, evaluator Avaliador, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	if evaluator != nil {
		return evaluator.Avaliar(card, pos, ctx)
	}
	legacy := EvaluateBotScore(card, pos, DefaultBotScoreProfile)
	return domain.AvaliacaoCarta{Disponivel: true, Nota: legacy.Total, Contexto: domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: string(legacy.Profile), VersaoPerfil: legacy.Version, Ciclo: legacy.Cycle, Posicao: pos}, Componentes: legacyComponents(legacy)}
}

func legacyComponents(score BotScore) []domain.ComponenteAvaliacao {
	out := make([]domain.ComponenteAvaliacao, 0, len(score.Components))
	for _, c := range score.Components {
		out = append(out, domain.ComponenteAvaliacao{Chave: c.Key, Rotulo: c.Label, Valor: c.Value})
	}
	return out
}

func grupoPosicao(pos domain.Position) string {
	switch pos {
	case domain.GK:
		return "goleiro"
	case domain.CB:
		return "defensor"
	case domain.RB, domain.LB, domain.RWB, domain.LWB:
		return "lateral"
	case domain.CDM, domain.CM, domain.CAM:
		return "meio"
	default:
		return "ataque"
	}
}

func sortedWeightKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
func contains(values []string, want string) bool {
	for _, v := range values {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}
func round1(v float64) float64 { return math.Round(v*10) / 10 }

func avaliarPorteFisico(card domain.Player, profile perfilPorteFisicoArquivo) (float64, []string) {
	var missing []string
	if card.Height == 0 {
		missing = append(missing, "altura")
	}
	if card.WeightKg == nil {
		missing = append(missing, "peso")
	}
	if strings.TrimSpace(card.BodyType) == "" {
		missing = append(missing, "tipo corporal")
	}
	if len(missing) > 0 {
		return 0, missing
	}
	value := -math.Abs(float64(card.Height-profile.AlturaIdeal))*profile.PenalidadeAlturaPorCM -
		math.Abs(float64(*card.WeightKg-profile.PesoIdeal))*profile.PenalidadePesoPorKG
	bodyBonus, ok := valorNormalizado(profile.TiposCorpo, card.BodyType)
	if !ok {
		return 0, []string{"tipo corporal " + card.BodyType + " sem curva no perfil"}
	}
	value += bodyBonus
	if value < -profile.Limite {
		value = -profile.Limite
	}
	if value > profile.Limite {
		value = profile.Limite
	}
	return value, nil
}

func avaliarPeDominante(foot string, pos domain.Position, weight float64) (bonus float64, relevante, faltando bool) {
	if weight == 0 {
		return 0, false, false
	}
	want := ""
	switch pos {
	case domain.RB, domain.RWB, domain.RM, domain.RW:
		want = "Direito"
	case domain.LB, domain.LWB, domain.LM, domain.LW:
		want = "Esquerdo"
	default:
		return 0, false, false
	}
	foot = domain.NormalizeFoot(foot)
	if foot == "" {
		return 0, true, true
	}
	if foot == want {
		return weight, true, false
	}
	return -weight / 2, true, false
}

func valorNormalizado(values map[string]float64, raw string) (float64, bool) {
	want := normalizarValorPerfil(raw)
	for name, value := range values {
		if normalizarValorPerfil(name) == want {
			return value, true
		}
	}
	return 0, false
}

func normalizarValorPerfil(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", " ", "_", " ").Replace(value)
	return strings.Join(strings.Fields(value), " ")
}

var detailedAttributeNames = map[string]struct{}{
	"acceleration": {}, "sprint_speed": {}, "agility": {}, "balance": {}, "jumping": {}, "stamina": {}, "strength": {}, "reactions": {}, "aggression": {}, "composure": {}, "interceptions": {}, "positioning": {}, "vision": {}, "ball_control": {}, "crossing": {}, "dribbling": {}, "finishing": {}, "fk_accuracy": {}, "heading_accuracy": {}, "long_passing": {}, "short_passing": {}, "defensive_awareness": {}, "shot_power": {}, "long_shots": {}, "standing_tackle": {}, "sliding_tackle": {}, "volleys": {}, "curve": {}, "penalties": {}, "gk_diving": {}, "gk_handling": {}, "gk_kicking": {}, "gk_reflexes": {}, "gk_speed": {}, "gk_positioning": {},
}

func detailedValue(a *domain.DetailedAttributes, key string) (int, bool) {
	if a == nil {
		return 0, false
	}
	var v *int
	switch key {
	case "acceleration":
		v = a.Acceleration
	case "sprint_speed":
		v = a.SprintSpeed
	case "agility":
		v = a.Agility
	case "balance":
		v = a.Balance
	case "jumping":
		v = a.Jumping
	case "stamina":
		v = a.Stamina
	case "strength":
		v = a.Strength
	case "reactions":
		v = a.Reactions
	case "aggression":
		v = a.Aggression
	case "composure":
		v = a.Composure
	case "interceptions":
		v = a.Interceptions
	case "positioning":
		v = a.Positioning
	case "vision":
		v = a.Vision
	case "ball_control":
		v = a.BallControl
	case "crossing":
		v = a.Crossing
	case "dribbling":
		v = a.Dribbling
	case "finishing":
		v = a.Finishing
	case "fk_accuracy":
		v = a.FKAccuracy
	case "heading_accuracy":
		v = a.HeadingAccuracy
	case "long_passing":
		v = a.LongPassing
	case "short_passing":
		v = a.ShortPassing
	case "defensive_awareness":
		v = a.DefensiveAwareness
	case "shot_power":
		v = a.ShotPower
	case "long_shots":
		v = a.LongShots
	case "standing_tackle":
		v = a.StandingTackle
	case "sliding_tackle":
		v = a.SlidingTackle
	case "volleys":
		v = a.Volleys
	case "curve":
		v = a.Curve
	case "penalties":
		v = a.Penalties
	case "gk_diving":
		v = a.GKDiving
	case "gk_handling":
		v = a.GKHandling
	case "gk_kicking":
		v = a.GKKicking
	case "gk_reflexes":
		v = a.GKReflexes
	case "gk_speed":
		v = a.GKSpeed
	case "gk_positioning":
		v = a.GKPositioning
	}
	if v == nil {
		return 0, false
	}
	return *v, true
}
