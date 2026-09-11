package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// SquadEditorResponse entrega o clube inteiro ao editor. A listagem comum de
// reservas continua paginada para a tela Meu time; o editor precisa pesquisar
// e posicionar qualquer cópia física, inclusive abaixo de qualquer overall.
type SquadEditorResponse struct {
	GeneratedAt       time.Time                `json:"generated_at"`
	Clube             string                   `json:"clube"`
	Formacao          string                   `json:"formacao"`
	Titulares         []StarterCard            `json:"titulares"`
	Cartas            []RosterCard             `json:"cartas"`
	Alvos             []AlvoEditorElenco       `json:"alvos,omitempty"`
	Funcoes           []FuncaoEditorElenco     `json:"funcoes,omitempty"`
	QuimicaReferencia *chemistry.Resultado     `json:"quimica_referencia,omitempty"`
	Avaliacao         domain.ContextoAvaliacao `json:"avaliacao"`
}

// AlvoEditorElenco é uma carta hipotética que pode ocupar uma vaga sem
// entrar na coleção do clube. Referencia preserva se ela veio do mercado ou
// de uma evolução específica.
type AlvoEditorElenco struct {
	Tipo       string                       `json:"tipo"`
	Player     domain.ClubPlayer            `json:"player"`
	Referencia domain.ReferenciaCartaElenco `json:"referencia"`
	Custo      int                          `json:"custo,omitempty"`
	Descricao  string                       `json:"descricao,omitempty"`
}

// FuncaoEditorElenco Ã© uma opÃ§Ã£o do catÃ¡logo da coleta. O plano armazena o
// nome para continuar legÃ­vel; a posiÃ§Ã£o impede oferecer uma funÃ§Ã£o de LD
// em uma vaga de zagueiro.
type FuncaoEditorElenco struct {
	Nome    string          `json:"nome"`
	Posicao domain.Position `json:"posicao"`
}

// PlanoElencoInput é o corpo de criação e atualização. O servidor completa a
// identidade física, versão e datas; a interface nunca escolhe esses valores.
type PlanoElencoInput struct {
	Nome            string                         `json:"nome"`
	Formacao        string                         `json:"formacao"`
	OrigemFormacao  string                         `json:"origem_formacao"`
	EstiloJogo      string                         `json:"estilo_jogo,omitempty"`
	RevisaoEsperada int                            `json:"revisao_esperada,omitempty"`
	Vagas           []domain.VagaPlanoElenco       `json:"vagas"`
	Banco           []domain.ReferenciaCartaElenco `json:"banco,omitempty"`
	NaoRelacionados []domain.ReferenciaCartaElenco `json:"nao_relacionados,omitempty"`
}

type planoElencoSalvoView struct {
	Plano      domain.PlanoElencoSalvo `json:"plano"`
	Pendencias []string                `json:"pendencias,omitempty"`
}

type planosElencoSalvosResponse struct {
	Value []planoElencoSalvoView `json:"value"`
	Count int                    `json:"@odata.count"`
}

// AvaliacaoEditorVaga é a nota da carta NA vaga física, para a tela nunca
// comparar a nota geral da carta com a posição que ela ocupa.
type AvaliacaoEditorVaga struct {
	Index              int                    `json:"index"`
	Posicao            domain.Position        `json:"posicao"`
	Carta              *domain.ClubPlayer     `json:"carta,omitempty"`
	Nota               float64                `json:"nota,omitempty"`
	NotaDisponivel     bool                   `json:"nota_disponivel"`
	Avaliacao          *domain.AvaliacaoCarta `json:"avaliacao,omitempty"`
	ForaDePosicao      bool                   `json:"fora_de_posicao"`
	Funcao             string                 `json:"funcao,omitempty"`
	EstiloEntrosamento string                 `json:"estilo_entrosamento,omitempty"`
}

type AvaliacaoEditorElencoResponse struct {
	Status       string                   `json:"status"`
	Motivo       string                   `json:"motivo,omitempty"`
	Formacao     string                   `json:"formacao"`
	RevisaoPlano string                   `json:"revisao_plano"`
	Vagas        []AvaliacaoEditorVaga    `json:"vagas"`
	Media        float64                  `json:"media,omitempty"`
	Cobertura    int                      `json:"cobertura"`
	EloMaisFraco *AvaliacaoEditorVaga     `json:"elo_mais_fraco,omitempty"`
	Quimica      *chemistry.Resultado     `json:"quimica,omitempty"`
	Avaliacao    domain.ContextoAvaliacao `json:"avaliacao"`
	Avisos       []string                 `json:"avisos,omitempty"`
}

func (s *Server) handleSquadEditor(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	lookup := newCardSlugLookup(snap.Cards)
	quimicaReferencia := s.currentChemistry(snap)
	quimicaPorVaga := chemistryBySlot(quimicaReferencia)
	starters, formation := buildStarters(snap, quimicaPorVaga)
	cards := make([]RosterCard, 0, len(snap.Club.Players))
	for _, player := range snap.Club.Players {
		cards = append(cards, RosterCard{Player: player, CardSlug: lookup.slug(player)})
	}
	sort.SliceStable(cards, func(i, j int) bool {
		left, right := cards[i].Player.Display(), cards[j].Player.Display()
		if left == right {
			return cards[i].Player.ID < cards[j].Player.ID
		}
		return left < right
	})
	writeJSON(w, SquadEditorResponse{
		GeneratedAt:       snap.GeneratedAt,
		Clube:             squadPlanClubKey(snap.Club),
		Formacao:          formation,
		Titulares:         starters,
		Cartas:            cards,
		Alvos:             alvosDoEditor(snap),
		Funcoes:           funcoesDoEditor(snap.RoleCatalog),
		QuimicaReferencia: quimicaReferencia,
		Avaliacao:         s.resolveEvaluationContext(),
	})
}

func alvosDoEditor(snap store.Snapshot) []AlvoEditorElenco {
	result := make([]AlvoEditorElenco, 0, len(snap.Upgrades)+len(snap.EvoMatches))
	seen := make(map[string]bool)
	for _, upgrade := range snap.Upgrades {
		key := fmt.Sprintf("mercado:%d", upgrade.Candidate.ID)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, AlvoEditorElenco{
			Tipo: "mercado", Player: domain.ClubPlayer{Player: upgrade.Candidate, AlvoPlano: true},
			Referencia: domain.ReferenciaCartaElenco{Origem: "mercado", PlayerID: upgrade.Candidate.ID},
			Custo:      upgrade.GrossCost, Descricao: fmt.Sprintf("alvo de compra para %s", upgrade.Slot),
		})
	}
	for _, match := range snap.EvoMatches {
		base := domain.ReferenciaCartaElenco{Origem: "evolucao", ClubItemID: match.Player.ClubItemID, PlayerID: match.Player.ID, EvolucaoID: match.Evolution.ID}
		key := fmt.Sprintf("evolucao:%s:%s:%d", base.EvolucaoID, base.ClubItemID, base.PlayerID)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, AlvoEditorElenco{
			Tipo: "evolucao", Player: domain.ClubPlayer{Player: match.Result, AlvoPlano: true}, Referencia: base,
			Custo: match.Cost, Descricao: match.Evolution.Name,
		})
	}
	return result
}

func funcoesDoEditor(catalogo futgg.RolesTable) []FuncaoEditorElenco {
	seen := make(map[string]bool)
	result := make([]FuncaoEditorElenco, 0, len(catalogo.Plus)+len(catalogo.PlusPlus))
	add := func(role futgg.Role) {
		key := string(role.Position) + "\x00" + role.Name
		if role.Name == "" || seen[key] {
			return
		}
		seen[key] = true
		result = append(result, FuncaoEditorElenco{Nome: role.Name, Posicao: role.Position})
	}
	for _, role := range catalogo.Plus {
		add(role)
	}
	for _, role := range catalogo.PlusPlus {
		add(role)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Posicao != result[j].Posicao {
			return result[i].Posicao < result[j].Posicao
		}
		return result[i].Nome < result[j].Nome
	})
	return result
}

func (s *Server) handleSavedSquadPlans(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedSquadPlanStore)
	if !ok {
		http.Error(w, "planos salvos indisponíveis neste armazenamento", http.StatusNotImplemented)
		return
	}
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	plans, err := backend.ListSavedSquadPlans(r.Context(), squadPlanCycle(s, snap), squadPlanClubKey(snap.Club))
	if err != nil {
		http.Error(w, "lendo planos salvos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	value := make([]planoElencoSalvoView, 0, len(plans))
	for _, plan := range plans {
		value = append(value, planoElencoSalvoView{Plano: plan, Pendencias: pendenciasDoPlano(snap, plan)})
	}
	writeJSON(w, planosElencoSalvosResponse{Value: value, Count: len(value)})
}

func (s *Server) handleSavedSquadPlanCreate(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedSquadPlanStore)
	if !ok {
		http.Error(w, "planos salvos indisponíveis neste armazenamento", http.StatusNotImplemented)
		return
	}
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	input, ok := readPlanoElencoInput(w, r)
	if !ok {
		return
	}
	id, err := newSquadPlanID()
	if err != nil {
		http.Error(w, "gerando id do plano: "+err.Error(), http.StatusInternalServerError)
		return
	}
	now := time.Now()
	plan := planoDoInput(id, squadPlanCycle(s, snap), squadPlanClubKey(snap.Club), input, 1, false, now, now)
	if err := normalizarPlanoContraSnapshot(&plan, snap); err != nil {
		http.Error(w, "validando plano: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := backend.SaveSquadPlan(r.Context(), plan); err != nil {
		http.Error(w, "gravando plano salvo: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, planoElencoSalvoView{Plano: plan})
}

func (s *Server) handleSavedSquadPlanUpdate(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedSquadPlanStore)
	if !ok {
		http.Error(w, "planos salvos indisponíveis neste armazenamento", http.StatusNotImplemented)
		return
	}
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	input, ok := readPlanoElencoInput(w, r)
	if !ok {
		return
	}
	cycle, club := squadPlanCycle(s, snap), squadPlanClubKey(snap.Club)
	previous, found, err := savedSquadPlanByID(r.Context(), backend, cycle, club, r.PathValue("id"))
	if err != nil {
		http.Error(w, "lendo plano salvo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(w, "plano salvo não encontrado neste clube", http.StatusNotFound)
		return
	}
	// RevisaoEsperada preserva compatibilidade com clientes antigos quando
	// omitida. O editor atual sempre a envia, impedindo que uma aba antiga
	// sobrescreva uma revisão mais nova.
	if input.RevisaoEsperada > 0 && input.RevisaoEsperada != previous.Revisao {
		http.Error(w, fmt.Sprintf("conflito de edição: o plano está na revisão %d, mas esta aba editou a revisão %d; abra o plano novamente antes de salvar", previous.Revisao, input.RevisaoEsperada), http.StatusConflict)
		return
	}
	plan := planoDoInput(previous.ID, cycle, club, input, previous.Revisao+1, previous.Referencia, previous.CriadoEm, time.Now())
	if err := normalizarPlanoContraSnapshot(&plan, snap); err != nil {
		http.Error(w, "validando plano: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := backend.SaveSquadPlan(r.Context(), plan); err != nil {
		http.Error(w, "gravando plano salvo: "+err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, planoElencoSalvoView{Plano: plan})
}

func (s *Server) handleSavedSquadPlanDelete(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedSquadPlanStore)
	if !ok {
		http.Error(w, "planos salvos indisponíveis neste armazenamento", http.StatusNotImplemented)
		return
	}
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	if err := backend.DeleteSavedSquadPlan(r.Context(), squadPlanCycle(s, snap), squadPlanClubKey(snap.Club), r.PathValue("id")); err != nil {
		http.Error(w, "apagando plano salvo: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSavedSquadPlanReference(w http.ResponseWriter, r *http.Request) {
	backend, ok := s.Store.(store.SavedSquadPlanStore)
	if !ok {
		http.Error(w, "planos salvos indisponíveis neste armazenamento", http.StatusNotImplemented)
		return
	}
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	cycle, club, targetID := squadPlanCycle(s, snap), squadPlanClubKey(snap.Club), r.PathValue("id")
	plans, err := backend.ListSavedSquadPlans(r.Context(), cycle, club)
	if err != nil {
		http.Error(w, "lendo planos salvos: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var target *domain.PlanoElencoSalvo
	for i := range plans {
		if plans[i].ID == targetID {
			target = &plans[i]
			break
		}
	}
	if target == nil {
		http.Error(w, "plano salvo não encontrado neste clube", http.StatusNotFound)
		return
	}
	now := time.Now()
	for i := range plans {
		wantReference := plans[i].ID == targetID
		if plans[i].Referencia == wantReference {
			continue
		}
		plans[i].Referencia = wantReference
		plans[i].AtualizadoEm = now
		if wantReference {
			plans[i].Revisao++
		}
		if err := backend.SaveSquadPlan(r.Context(), plans[i]); err != nil {
			http.Error(w, "aplicando plano de referência: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if wantReference {
			target = &plans[i]
		}
	}
	writeJSON(w, planoElencoSalvoView{Plano: *target, Pendencias: pendenciasDoPlano(snap, *target)})
}

func (s *Server) handleSquadEditorEvaluate(w http.ResponseWriter, r *http.Request) {
	snap, loaded := s.load(w, r)
	if !loaded {
		return
	}
	var input struct {
		Formacao   string                   `json:"formacao"`
		EstiloJogo string                   `json:"estilo_jogo,omitempty"`
		Vagas      []domain.VagaPlanoElenco `json:"vagas"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "lendo avaliação do editor: "+err.Error(), http.StatusBadRequest)
		return
	}
	plan := domain.PlanoElencoSalvo{
		ID: "rascunho", Ciclo: squadPlanCycle(s, snap), Clube: squadPlanClubKey(snap.Club), Nome: "Rascunho do editor",
		Formacao: input.Formacao, OrigemFormacao: "manual", EstiloJogo: strings.TrimSpace(input.EstiloJogo), Vagas: input.Vagas, Revisao: 1,
	}
	if err := normalizarPlanoContraSnapshot(&plan, snap); err != nil {
		http.Error(w, "validando rascunho: "+err.Error(), http.StatusBadRequest)
		return
	}
	plan.ID = revisaoDoRascunho(plan)
	writeJSON(w, s.avaliarPlanoDoEditor(snap, plan))
}

func revisaoDoRascunho(plan domain.PlanoElencoSalvo) string {
	payload, _ := json.Marshal(struct {
		Formacao   string                   `json:"formacao"`
		EstiloJogo string                   `json:"estilo_jogo"`
		Vagas      []domain.VagaPlanoElenco `json:"vagas"`
	}{Formacao: plan.Formacao, EstiloJogo: plan.EstiloJogo, Vagas: plan.Vagas})
	sum := sha256.Sum256(payload)
	return "rascunho-" + hex.EncodeToString(sum[:6])
}

func (s *Server) avaliarPlanoDoEditor(snap store.Snapshot, plan domain.PlanoElencoSalvo) AvaliacaoEditorElencoResponse {
	club, jogadores, err := clubeParaPlanoSnapshot(snap, plan)
	contexto := s.resolveEvaluationContext()
	contexto.EstiloJogo = plan.EstiloJogo
	contexto.RevisaoPlano = fmt.Sprintf("%s:%d", plan.ID, plan.Revisao)
	contexto.Snapshot = snap.GeneratedAt.UTC().Format(time.RFC3339)
	response := AvaliacaoEditorElencoResponse{Formacao: plan.Formacao, RevisaoPlano: contexto.RevisaoPlano, Avaliacao: contexto, Vagas: make([]AvaliacaoEditorVaga, 0, len(plan.Vagas))}
	if err != nil {
		response.Status, response.Motivo = "indisponivel", err.Error()
		return response
	}
	quimica := chemistry.Avaliar(s.resolveChemistryModel(), club)
	club = aplicarQuimicaSimulada(club, quimica)
	if len(jogadores) != 11 {
		response.Status = "incompleto"
		response.Avisos = append(response.Avisos, fmt.Sprintf("Faltam %d vagas para avaliar o XI completo.", 11-len(jogadores)))
	} else {
		response.Status = "ok"
	}
	if quimica != nil {
		response.Quimica = quimica
	}
	for _, vaga := range plan.Vagas {
		view := AvaliacaoEditorVaga{Index: vaga.Index, Posicao: vaga.Posicao, Funcao: vaga.Funcao, EstiloEntrosamento: vaga.EstiloEntrosamento}
		if vaga.Carta.Vazia() {
			response.Vagas = append(response.Vagas, view)
			continue
		}
		player, err := jogadorDaReferencia(club, vaga.Carta)
		if err != nil {
			response.Avisos = append(response.Avisos, fmt.Sprintf("A carta da vaga %d não está na coleta atual.", vaga.Index))
			response.Vagas = append(response.Vagas, view)
			continue
		}
		view.Carta = &player
		view.ForaDePosicao = !player.PlaysAt(vaga.Posicao)
		contextoDaVaga := response.Avaliacao
		contextoDaVaga.Posicao = vaga.Posicao
		contextoDaVaga.Funcao = vaga.Funcao
		contextoDaVaga.EstiloEntrosamento = vaga.EstiloEntrosamento
		chem := player.Chemistry
		contextoDaVaga.Quimica = &chem
		resultado := s.resolveEvaluator().Avaliar(player.Player, vaga.Posicao, contextoDaVaga)
		view.Avaliacao = &resultado
		view.Nota, view.NotaDisponivel = resultado.Nota, resultado.Disponivel
		if view.NotaDisponivel {
			response.Cobertura++
			response.Media += view.Nota
			if response.EloMaisFraco == nil || view.Nota < response.EloMaisFraco.Nota {
				weakest := view
				response.EloMaisFraco = &weakest
			}
		}
		response.Vagas = append(response.Vagas, view)
	}
	if response.Cobertura > 0 {
		response.Media /= float64(response.Cobertura)
	}
	if response.Cobertura < len(jogadores) {
		response.Avisos = append(response.Avisos, "Há cartas sem nota disponível neste avaliador e contexto; elas não entram na média.")
	}
	return response
}

func readPlanoElencoInput(w http.ResponseWriter, r *http.Request) (PlanoElencoInput, bool) {
	var input PlanoElencoInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "lendo plano de elenco: "+err.Error(), http.StatusBadRequest)
		return PlanoElencoInput{}, false
	}
	return input, true
}

func planoDoInput(id, ciclo, clube string, input PlanoElencoInput, revisao int, referencia bool, criadoEm, atualizadoEm time.Time) domain.PlanoElencoSalvo {
	return domain.PlanoElencoSalvo{
		ID: id, Ciclo: ciclo, Clube: clube, Nome: strings.TrimSpace(input.Nome), Formacao: strings.TrimSpace(input.Formacao),
		OrigemFormacao: strings.TrimSpace(input.OrigemFormacao), EstiloJogo: strings.TrimSpace(input.EstiloJogo),
		Vagas: input.Vagas, Banco: input.Banco, NaoRelacionados: input.NaoRelacionados,
		Revisao: revisao, Referencia: referencia, CriadoEm: criadoEm, AtualizadoEm: atualizadoEm,
	}
}

func normalizarPlanoContraSnapshot(plan *domain.PlanoElencoSalvo, snap store.Snapshot) error {
	for i := range plan.Vagas {
		if plan.Vagas[i].Carta.Vazia() {
			continue
		}
		ref, err := referenciaDoSnapshot(snap, plan.Vagas[i].Carta)
		if err != nil {
			return fmt.Errorf("vaga %d: %w", plan.Vagas[i].Index, err)
		}
		plan.Vagas[i].Carta = ref
	}
	for i := range plan.Banco {
		ref, err := referenciaDoSnapshot(snap, plan.Banco[i])
		if err != nil {
			return fmt.Errorf("banco: %w", err)
		}
		plan.Banco[i] = ref
	}
	for i := range plan.NaoRelacionados {
		ref, err := referenciaDoSnapshot(snap, plan.NaoRelacionados[i])
		if err != nil {
			return fmt.Errorf("não relacionados: %w", err)
		}
		plan.NaoRelacionados[i] = ref
	}
	return plan.Validate()
}

func referenciaDoSnapshot(snap store.Snapshot, ref domain.ReferenciaCartaElenco) (domain.ReferenciaCartaElenco, error) {
	switch ref.Origem {
	case "", "clube":
		normalized, err := referenciaDoClube(snap.Club, ref)
		if err == nil && ref.Origem == "clube" {
			normalized.Origem = "clube"
		}
		return normalized, err
	case "mercado":
		for _, player := range snap.Market {
			if player.ID == ref.PlayerID {
				return domain.ReferenciaCartaElenco{Origem: "mercado", PlayerID: player.ID}, nil
			}
		}
		return ref, fmt.Errorf("alvo de mercado %d não está na coleta atual", ref.PlayerID)
	case "evolucao":
		baseRef, err := referenciaDoClube(snap.Club, domain.ReferenciaCartaElenco{ClubItemID: ref.ClubItemID, PlayerID: ref.PlayerID})
		if err != nil {
			return ref, err
		}
		for _, match := range snap.EvoMatches {
			if match.Evolution.ID == ref.EvolucaoID && match.Player.ID == baseRef.PlayerID && (baseRef.ClubItemID == "" || match.Player.ClubItemID == baseRef.ClubItemID) {
				return domain.ReferenciaCartaElenco{Origem: "evolucao", ClubItemID: baseRef.ClubItemID, PlayerID: baseRef.PlayerID, EvolucaoID: ref.EvolucaoID}, nil
			}
		}
		return ref, fmt.Errorf("evolução %q para a carta %d não está na coleta atual", ref.EvolucaoID, ref.PlayerID)
	default:
		return ref, fmt.Errorf("origem de carta %q desconhecida", ref.Origem)
	}
}

func referenciaDoClube(club domain.Club, ref domain.ReferenciaCartaElenco) (domain.ReferenciaCartaElenco, error) {
	if ref.Vazia() {
		return ref, fmt.Errorf("informe club_item_id ou player_id")
	}
	if ref.ClubItemID != "" {
		for _, player := range club.Players {
			if player.ClubItemID != ref.ClubItemID {
				continue
			}
			if ref.PlayerID != 0 && ref.PlayerID != player.ID {
				return ref, fmt.Errorf("club_item_id %q não pertence ao player_id %d", ref.ClubItemID, ref.PlayerID)
			}
			return domain.ReferenciaCartaElenco{ClubItemID: player.ClubItemID, PlayerID: player.ID}, nil
		}
		return ref, fmt.Errorf("carta %q não está na coleta atual", ref.ClubItemID)
	}
	var matches []domain.ClubPlayer
	for _, player := range club.Players {
		if player.ID == ref.PlayerID {
			matches = append(matches, player)
		}
	}
	if len(matches) == 0 {
		return ref, fmt.Errorf("player_id %d não está na coleta atual", ref.PlayerID)
	}
	if len(matches) > 1 {
		return ref, fmt.Errorf("player_id %d tem %d cópias; informe club_item_id", ref.PlayerID, len(matches))
	}
	return domain.ReferenciaCartaElenco{ClubItemID: matches[0].ClubItemID, PlayerID: matches[0].ID}, nil
}

func clubeParaPlano(base domain.Club, plan domain.PlanoElencoSalvo) (domain.Club, []domain.ClubPlayer, error) {
	return clubeParaPlanoResolvido(base, plan, func(ref domain.ReferenciaCartaElenco) (domain.ClubPlayer, error) {
		return jogadorDaReferencia(base, ref)
	})
}

func clubeParaPlanoSnapshot(snap store.Snapshot, plan domain.PlanoElencoSalvo) (domain.Club, []domain.ClubPlayer, error) {
	return clubeParaPlanoResolvido(snap.Club, plan, func(ref domain.ReferenciaCartaElenco) (domain.ClubPlayer, error) {
		return jogadorDoSnapshot(snap, ref)
	})
}

func clubeParaPlanoResolvido(base domain.Club, plan domain.PlanoElencoSalvo, resolver func(domain.ReferenciaCartaElenco) (domain.ClubPlayer, error)) (domain.Club, []domain.ClubPlayer, error) {
	selected := make([]domain.ClubPlayer, 0, len(plan.Vagas))
	selectedCards := make(map[string]bool)
	for _, vaga := range plan.Vagas {
		if vaga.Carta.Vazia() {
			continue
		}
		player, err := resolver(vaga.Carta)
		if err != nil {
			return domain.Club{}, nil, err
		}
		key := chaveCartaDoClube(player)
		if selectedCards[key] {
			return domain.Club{}, nil, fmt.Errorf("a carta %s foi escalada em mais de uma vaga", player.Display())
		}
		selectedCards[key] = true
		selected = append(selected, player)
	}
	club := base
	club.Players = append([]domain.ClubPlayer(nil), selected...)
	for i := range club.Players {
		club.Players[i].InSquad = true
	}
	for _, player := range base.Players {
		if !selectedCards[chaveCartaDoClube(player)] {
			player.InSquad = false
			club.Players = append(club.Players, player)
		}
	}
	club.ProtectedCards = make(map[string]bool, len(selected)+len(plan.Banco))
	for _, player := range selected {
		club.ProtectedCards[player.IdentityKey()] = true
	}
	for _, ref := range plan.Banco {
		player, err := resolver(ref)
		if err != nil {
			return domain.Club{}, nil, err
		}
		club.ProtectedCards[player.IdentityKey()] = true
	}
	club.Squad.Formation = plan.Formacao
	club.Squad.Chemistry = 0
	club.Squad.ChemistrySynced = false
	club.Squad.SyncedAt = time.Time{}
	club.Squad.Starters = make([]domain.SquadSlot, 0, len(plan.Vagas))
	for _, vaga := range plan.Vagas {
		if vaga.Carta.Vazia() {
			continue
		}
		club.Squad.Starters = append(club.Squad.Starters, domain.SquadSlot{Index: vaga.Index, Position: vaga.Posicao, PlayerID: vaga.Carta.PlayerID, ClubItemID: vaga.Carta.ClubItemID})
	}
	return club, selected, nil
}

func jogadorDoSnapshot(snap store.Snapshot, ref domain.ReferenciaCartaElenco) (domain.ClubPlayer, error) {
	switch ref.Origem {
	case "", "clube":
		return jogadorDaReferencia(snap.Club, ref)
	case "mercado":
		for _, player := range snap.Market {
			if player.ID == ref.PlayerID {
				return domain.ClubPlayer{Player: player, AlvoPlano: true}, nil
			}
		}
	case "evolucao":
		for _, match := range snap.EvoMatches {
			if match.Evolution.ID != ref.EvolucaoID || match.Player.ID != ref.PlayerID || (ref.ClubItemID != "" && match.Player.ClubItemID != ref.ClubItemID) {
				continue
			}
			result := match.Player
			result.Player = match.Result
			result.AlvoPlano = true
			return result, nil
		}
	}
	return domain.ClubPlayer{}, fmt.Errorf("o alvo %s:%d não está na coleta atual", ref.Origem, ref.PlayerID)
}

func chaveCartaDoClube(player domain.ClubPlayer) string {
	return player.IdentityKey()
}

func jogadorDaReferencia(club domain.Club, ref domain.ReferenciaCartaElenco) (domain.ClubPlayer, error) {
	for _, player := range club.Players {
		if ref.ClubItemID != "" && player.ClubItemID == ref.ClubItemID {
			return player, nil
		}
	}
	if ref.ClubItemID == "" && ref.PlayerID != 0 {
		var found []domain.ClubPlayer
		for _, player := range club.Players {
			if player.ID == ref.PlayerID {
				found = append(found, player)
			}
		}
		if len(found) == 1 {
			return found[0], nil
		}
		if len(found) > 1 {
			return domain.ClubPlayer{}, fmt.Errorf("a carta %d tem cópias ambíguas na coleta atual", ref.PlayerID)
		}
	}
	return domain.ClubPlayer{}, fmt.Errorf("a carta %q não está na coleta atual", ref.ClubItemID)
}

func pendenciasDoPlano(snap store.Snapshot, plan domain.PlanoElencoSalvo) []string {
	var pending []string
	for _, vaga := range plan.Vagas {
		if vaga.Carta.Vazia() {
			continue
		}
		if _, err := jogadorDoSnapshot(snap, vaga.Carta); err != nil {
			pending = append(pending, fmt.Sprintf("Vaga %d: %v.", vaga.Index, err))
		}
	}
	for _, carta := range append(append([]domain.ReferenciaCartaElenco(nil), plan.Banco...), plan.NaoRelacionados...) {
		if _, err := jogadorDoSnapshot(snap, carta); err != nil {
			pending = append(pending, err.Error()+".")
		}
	}
	return pending
}

func savedSquadPlanByID(ctx context.Context, backend store.SavedSquadPlanStore, cycle, club, id string) (domain.PlanoElencoSalvo, bool, error) {
	plans, err := backend.ListSavedSquadPlans(ctx, cycle, club)
	if err != nil {
		return domain.PlanoElencoSalvo{}, false, err
	}
	for _, plan := range plans {
		if plan.ID == id {
			return plan, true, nil
		}
	}
	return domain.PlanoElencoSalvo{}, false, nil
}

func squadPlanCycle(s *Server, snap store.Snapshot) string {
	if s.Cycle != "" {
		return s.Cycle
	}
	if snap.Cycle != "" {
		return snap.Cycle
	}
	return snap.Club.Cycle
}

func squadPlanClubKey(club domain.Club) string {
	if key := strings.ToLower(strings.TrimSpace(club.GamerTag)); key != "" {
		return key
	}
	return "clube-local"
}

func newSquadPlanID() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "plano-" + hex.EncodeToString(raw[:]), nil
}
