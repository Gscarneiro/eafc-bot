package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// profileEvent é uma trilha local de ativação. O motor e os arquivos de
// perfil continuam imutáveis: ativar só muda a preferência explícita e deixa
// o estado anterior disponível para reversão.
type profileEvent struct {
	Tipo     string                      `json:"tipo"`
	Em       time.Time                   `json:"em"`
	Proposta string                      `json:"proposta,omitempty"`
	Anterior config.UISettingsEvaluation `json:"anterior"`
	Novo     config.UISettingsEvaluation `json:"novo"`
}

type profileHistory struct {
	Eventos []profileEvent                `json:"eventos"`
	Pilha   []config.UISettingsEvaluation `json:"pilha"`
}

func cmdPerfil(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("informe validar, comparar, propor, propostas, aprovar, rejeitar, ativar ou reverter (ex.: `eafcbot perfil validar`)")
	}
	switch args[0] {
	case "validar":
		return cmdPerfilValidar(args[1:])
	case "comparar":
		return cmdPerfilComparar(ctx, args[1:])
	case "ativar":
		return cmdPerfilAtivar(ctx, args[1:])
	case "reverter":
		return cmdPerfilReverter(ctx, args[1:])
	case "propor":
		return cmdPerfilPropor(ctx, args[1:])
	case "propostas":
		return cmdPerfilPropostas(ctx, args[1:])
	case "aprovar":
		return cmdPerfilDecidir(ctx, args[1:], "aprovada")
	case "rejeitar":
		return cmdPerfilDecidir(ctx, args[1:], "rejeitada")
	default:
		return fmt.Errorf("subcomando de perfil desconhecido: %q (use validar, comparar, propor, propostas, aprovar, rejeitar, ativar ou reverter)", args[0])
	}
}

type flagsRepetidas []string

func (values *flagsRepetidas) String() string { return strings.Join(*values, ",") }
func (values *flagsRepetidas) Set(value string) error {
	*values = append(*values, strings.TrimSpace(value))
	return nil
}

func cmdPerfilPropor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("perfil propor", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	profileID := fs.String("perfil", "", "perfil candidato")
	patch := fs.String("patch", "", "patch investigado")
	plataformas := fs.String("plataformas", "ps5,xbox_series,pc", "plataformas separadas por vírgula")
	mecanicas := fs.String("mecanicas", "", "mecânicas afetadas separadas por vírgula")
	impacto := fs.String("impacto", "", "impacto esperado no ranking")
	resultado := fs.String("resultado", "", "resultado das regressões e avaliação")
	var fatos, hipoteses, limitacoes, mudancas flagsRepetidas
	fs.Var(&fatos, "fato", "descricao|fonte|data|aplicabilidade (repetível)")
	fs.Var(&hipoteses, "hipotese", "descricao|fonte|data|aplicabilidade (repetível)")
	fs.Var(&limitacoes, "limitacao", "limitação conhecida (repetível)")
	fs.Var(&mudancas, "mudanca-codigo", "descricao|ramo|arquivo,arquivo (repetível)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profileID == "" || *patch == "" {
		return fmt.Errorf("informe -perfil e -patch")
	}
	registry, err := analyze.NovoRegistroAvaliadores()
	if err != nil {
		return err
	}
	profile, found := registry.Perfil(*profileID)
	if !found {
		return fmt.Errorf("perfil %q não existe; rode `eafcbot perfil validar`", *profileID)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	proposal := domain.PropostaMeta{
		Ciclo: cfg.FutGG.Cycle, Patch: strings.TrimSpace(*patch), Plataformas: dividirLista(*plataformas),
		Perfil: profile.ID, VersaoCandidata: profile.Versao, Status: "proposta",
		MecanicasAfetadas: dividirLista(*mecanicas), Impacto: strings.TrimSpace(*impacto),
		ResultadoAvaliacao: strings.TrimSpace(*resultado), Limitacoes: limitacoes,
	}
	for _, raw := range fatos {
		evidence, err := lerEvidenciaMeta(raw)
		if err != nil {
			return fmt.Errorf("fato: %w", err)
		}
		proposal.Fatos = append(proposal.Fatos, evidence)
	}
	for _, raw := range hipoteses {
		evidence, err := lerEvidenciaMeta(raw)
		if err != nil {
			return fmt.Errorf("hipótese: %w", err)
		}
		proposal.Hipoteses = append(proposal.Hipoteses, evidence)
	}
	for _, raw := range mudancas {
		change, err := lerMudancaMeta(raw)
		if err != nil {
			return fmt.Errorf("mudança de código: %w", err)
		}
		proposal.MudancasCodigo = append(proposal.MudancasCodigo, change)
	}
	canonical, _ := json.Marshal(proposal)
	hash := sha256.Sum256(canonical)
	proposal.Pacote = fmt.Sprintf("sha256:%x", hash[:])
	proposal.ID = fmt.Sprintf("meta-%x", hash[:8])
	now := time.Now().UTC().Format(time.RFC3339)
	proposal.CriadaEm, proposal.AtualizadaEm = now, now
	if err := proposal.Validate(); err != nil {
		return err
	}
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	backend, ok := st.(store.MetaProposalStore)
	if !ok {
		return fmt.Errorf("armazenamento não suporta propostas de meta")
	}
	if err := backend.SaveMetaProposal(ctx, proposal); err != nil {
		return err
	}
	fmt.Printf("proposta %s criada para %s %s; pacote %s\n", proposal.ID, proposal.Perfil, proposal.VersaoCandidata, proposal.Pacote)
	return nil
}

func cmdPerfilPropostas(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("perfil propostas", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	backend, ok := st.(store.MetaProposalStore)
	if !ok {
		return fmt.Errorf("armazenamento não suporta propostas de meta")
	}
	entries, err := backend.ListMetaProposals(ctx, cfg.FutGG.Cycle)
	if err != nil {
		return err
	}
	for _, proposal := range entries {
		fmt.Printf("%s  %-10s  %s %s  patch %s\n", proposal.ID, proposal.Status, proposal.Perfil, proposal.VersaoCandidata, proposal.Patch)
	}
	return nil
}

func cmdPerfilDecidir(ctx context.Context, args []string, status string) error {
	fs := flag.NewFlagSet("perfil decidir", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	id := fs.String("proposta", "", "id da proposta")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("informe -proposta")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	backend, ok := st.(store.MetaProposalStore)
	if !ok {
		return fmt.Errorf("armazenamento não suporta propostas de meta")
	}
	entries, err := backend.ListMetaProposals(ctx, cfg.FutGG.Cycle)
	if err != nil {
		return err
	}
	for _, proposal := range entries {
		if proposal.ID != *id {
			continue
		}
		if proposal.Status != "proposta" {
			return fmt.Errorf("proposta %s está em %s e não aceita nova decisão", proposal.ID, proposal.Status)
		}
		proposal.Status, proposal.AtualizadaEm = status, time.Now().UTC().Format(time.RFC3339)
		if err := backend.SaveMetaProposal(ctx, proposal); err != nil {
			return err
		}
		fmt.Printf("proposta %s marcada como %s\n", proposal.ID, status)
		return nil
	}
	return fmt.Errorf("proposta %q não encontrada no ciclo %s", *id, cfg.FutGG.Cycle)
}

func lerEvidenciaMeta(raw string) (domain.EvidenciaMeta, error) {
	parts := strings.Split(raw, "|")
	if len(parts) < 3 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
		return domain.EvidenciaMeta{}, fmt.Errorf("use descricao|fonte|data|aplicabilidade")
	}
	out := domain.EvidenciaMeta{Descricao: strings.TrimSpace(parts[0]), Fonte: strings.TrimSpace(parts[1]), Data: strings.TrimSpace(parts[2])}
	if len(parts) > 3 {
		out.Aplicabilidade = strings.TrimSpace(strings.Join(parts[3:], "|"))
	}
	return out, nil
}

func lerMudancaMeta(raw string) (domain.MudancaMeta, error) {
	parts := strings.Split(raw, "|")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return domain.MudancaMeta{}, fmt.Errorf("use descricao|ramo|arquivo,arquivo")
	}
	change := domain.MudancaMeta{Descricao: strings.TrimSpace(parts[0])}
	if len(parts) > 1 {
		change.Ramo = strings.TrimSpace(parts[1])
	}
	if len(parts) > 2 {
		change.Arquivos = dividirLista(parts[2])
	}
	return change, nil
}

func dividirLista(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func cmdPerfilValidar(args []string) error {
	fs := flag.NewFlagSet("perfil validar", flag.ContinueOnError)
	id := fs.String("perfil", "", "id de um perfil específico")
	if err := fs.Parse(args); err != nil {
		return err
	}
	registry, err := analyze.NovoRegistroAvaliadores()
	if err != nil {
		return fmt.Errorf("validando perfis: %w", err)
	}
	profiles := registry.Perfis()
	for _, profile := range profiles {
		if *id != "" && *id != profile.ID {
			continue
		}
		fmt.Printf("válido: %s (%s, %s)\n", profile.ID, profile.Versao, profile.Status)
	}
	if *id != "" {
		if _, found := registry.Perfil(*id); !found {
			return fmt.Errorf("perfil %q não existe; rode `eafcbot perfil validar` para listar os perfis", *id)
		}
	}
	return nil
}

func cmdPerfilComparar(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("perfil comparar", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	candidateID := fs.String("perfil", "", "perfil candidato")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *candidateID == "" {
		return fmt.Errorf("informe -perfil com o perfil candidato (ex.: `eafcbot perfil comparar -perfil posse`)")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	registry, err := analyze.NovoRegistroAvaliadores()
	if err != nil {
		return err
	}
	if _, found := registry.Perfil(*candidateID); !found {
		return fmt.Errorf("perfil candidato %q não existe; rode `eafcbot perfil validar`", *candidateID)
	}
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	snap, found, err := st.LatestSnapshot(ctx, cfg.FutGG.Cycle)
	if err != nil {
		return fmt.Errorf("lendo último snapshot: %w", err)
	}
	if !found {
		return fmt.Errorf("não há snapshot do ciclo %s; rode `eafcbot run` antes de comparar perfis", cfg.FutGG.Cycle)
	}
	activeID := cfg.Evaluation.Profile
	if activeID == "" {
		activeID = "meta_competitivo"
	}
	if _, found := registry.Perfil(activeID); !found {
		return fmt.Errorf("perfil ativo %q não existe; rode `eafcbot perfil validar`", activeID)
	}
	base := domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: activeID, Ciclo: cfg.FutGG.Cycle, Patch: cfg.Evaluation.Patch, Plataforma: cfg.Platform, EstiloJogo: cfg.Evaluation.PlayStyle}
	contextos := contextosDeComparacaoPerfil(ctx, st, cfg, snap.Club, base)
	rows := make([]profileComparisonRow, 0, len(snap.Club.Squad.Starters))
	for _, slot := range snap.Club.Squad.Starters {
		card, found := snap.Club.PlayerForSlot(slot)
		if !found {
			continue
		}
		beforeCtx := base
		if specific, found := contextos[slot.Index]; found {
			beforeCtx = specific
		}
		beforeCtx.Posicao = slot.Position
		chem := card.Chemistry
		beforeCtx.Quimica = &chem
		afterCtx := beforeCtx
		afterCtx.Perfil = *candidateID
		before, after := registry.Avaliar(card.Player, slot.Position, beforeCtx), registry.Avaliar(card.Player, slot.Position, afterCtx)
		rows = append(rows, profileComparisonRow{Index: slot.Index, Position: slot.Position, Funcao: beforeCtx.Funcao, Name: card.Display(), Before: before.Nota, After: after.Nota, Available: before.Disponivel && after.Disponivel})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Index < rows[j].Index })
	fmt.Printf("comparação: %s → %s | snapshot %s\n", activeID, *candidateID, snap.GeneratedAt.Format(time.RFC3339))
	for _, row := range rows {
		if !row.Available {
			fmt.Printf("vaga %d %s %-24s sem cobertura nas duas réguas\n", row.Index+1, row.Position, row.Name)
			continue
		}
		role := row.Funcao
		if role == "" {
			role = "sem função"
		}
		fmt.Printf("vaga %d %s %-14s %-24s %.1f → %.1f (%+.1f)\n", row.Index+1, row.Position, role, row.Name, row.Before, row.After, row.After-row.Before)
	}
	imprimirResumoPorFuncao(rows)
	return nil
}

type profileComparisonRow struct {
	Index     int
	Position  domain.Position
	Funcao    string
	Name      string
	Before    float64
	After     float64
	Available bool
}

// contextosDeComparacaoPerfil lê somente a referência salva para preservar
// a função e o estilo escolhidos no editor. Se ela não estiver disponível,
// a comparação ainda funciona, declarando a função como ausente em vez de
// inventar uma para cada posição.
func contextosDeComparacaoPerfil(ctx context.Context, st store.Store, cfg config.Config, club domain.Club, base domain.ContextoAvaliacao) map[int]domain.ContextoAvaliacao {
	backend, ok := st.(store.SavedSquadPlanStore)
	if !ok {
		return nil
	}
	plans, err := backend.ListSavedSquadPlans(ctx, cfg.FutGG.Cycle, profileClubKey(club))
	if err != nil {
		return nil
	}
	for _, plan := range plans {
		if !plan.Referencia {
			continue
		}
		out := make(map[int]domain.ContextoAvaliacao, len(plan.Vagas))
		for _, slot := range plan.Vagas {
			item := base
			item.Posicao, item.Funcao, item.EstiloEntrosamento = slot.Posicao, slot.Funcao, slot.EstiloEntrosamento
			out[slot.Index] = item
		}
		return out
	}
	return nil
}

func profileClubKey(club domain.Club) string {
	if key := strings.ToLower(strings.TrimSpace(club.GamerTag)); key != "" {
		return key
	}
	return "clube-local"
}

func imprimirResumoPorFuncao(rows []profileComparisonRow) {
	type aggregate struct {
		count int
		delta float64
	}
	byRole := make(map[string]aggregate)
	for _, row := range rows {
		if !row.Available {
			continue
		}
		role := row.Funcao
		if role == "" {
			role = string(row.Position)
		}
		value := byRole[role]
		value.count++
		value.delta += row.After - row.Before
		byRole[role] = value
	}
	if len(byRole) == 0 {
		return
	}
	roles := make([]string, 0, len(byRole))
	for role := range byRole {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	fmt.Println("resumo por função:")
	for _, role := range roles {
		value := byRole[role]
		fmt.Printf("  %-18s %d vagas, variação média %+.1f\n", role, value.count, value.delta/float64(value.count))
	}
}

func cmdPerfilAtivar(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("perfil ativar", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	id := fs.String("perfil", "", "id do perfil aprovado ou experimental")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return fmt.Errorf("informe -perfil (ex.: `eafcbot perfil ativar -perfil posse`)")
	}
	registry, err := analyze.NovoRegistroAvaliadores()
	if err != nil {
		return err
	}
	profile, found := registry.Perfil(*id)
	if !found {
		return fmt.Errorf("perfil %q não existe; rode `eafcbot perfil validar`", *id)
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	backend, ok := st.(store.MetaProposalStore)
	if !ok {
		return fmt.Errorf("armazenamento não suporta propostas de meta")
	}
	proposals, err := backend.ListMetaProposals(ctx, cfg.FutGG.Cycle)
	if err != nil {
		return err
	}
	var approved *domain.PropostaMeta
	for i := range proposals {
		proposal := &proposals[i]
		if proposal.Perfil == profile.ID && proposal.VersaoCandidata == profile.Versao && proposal.Status == "aprovada" {
			approved = proposal
			break
		}
	}
	if approved == nil {
		return fmt.Errorf("perfil %s %s não tem proposta aprovada; rode `eafcbot perfil propor`, compare e depois `eafcbot perfil aprovar`", profile.ID, profile.Versao)
	}
	before := cfg.Editable().Evaluation
	after := before
	after.UseBot, after.Profile = true, profile.ID
	settings := cfg.Editable()
	settings.Evaluation = after
	if err := cfg.SaveEditable(*configPath, settings); err != nil {
		return fmt.Errorf("ativando perfil: %w", err)
	}
	history, err := loadProfileHistory(profileHistoryPath(*configPath))
	if err != nil {
		return err
	}
	history.Pilha = append(history.Pilha, before)
	history.Eventos = append(history.Eventos, profileEvent{Tipo: "ativar", Em: time.Now().UTC(), Proposta: approved.ID, Anterior: before, Novo: after})
	if err := saveProfileHistory(profileHistoryPath(*configPath), history); err != nil {
		return err
	}
	approved.Status, approved.AtualizadaEm = "ativada", time.Now().UTC().Format(time.RFC3339)
	if err := backend.SaveMetaProposal(ctx, *approved); err != nil {
		return fmt.Errorf("marcando proposta ativada: %w", err)
	}
	fmt.Printf("perfil %s (%s) ativado; a próxima resposta da API usa a nova régua\n", profile.ID, profile.Status)
	return nil
}

func cmdPerfilReverter(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("perfil reverter", flag.ContinueOnError)
	configPath := fs.String("config", config.DefaultPath(), "arquivo de configuração")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := profileHistoryPath(*configPath)
	history, err := loadProfileHistory(path)
	if err != nil {
		return err
	}
	if len(history.Pilha) == 0 {
		return fmt.Errorf("não há ativação de perfil para reverter")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	before := cfg.Editable().Evaluation
	proposalID := ""
	for i := len(history.Eventos) - 1; i >= 0; i-- {
		if history.Eventos[i].Tipo == "ativar" && history.Eventos[i].Proposta != "" {
			proposalID = history.Eventos[i].Proposta
			break
		}
	}
	previous := history.Pilha[len(history.Pilha)-1]
	settings := cfg.Editable()
	settings.Evaluation = previous
	if err := cfg.SaveEditable(*configPath, settings); err != nil {
		return fmt.Errorf("revertendo perfil: %w", err)
	}
	history.Pilha = history.Pilha[:len(history.Pilha)-1]
	history.Eventos = append(history.Eventos, profileEvent{Tipo: "reverter", Em: time.Now().UTC(), Proposta: proposalID, Anterior: before, Novo: previous})
	if err := saveProfileHistory(path, history); err != nil {
		return err
	}
	if proposalID != "" {
		st, err := openStore(ctx, cfg)
		if err != nil {
			return err
		}
		defer st.Close()
		if backend, ok := st.(store.MetaProposalStore); ok {
			entries, err := backend.ListMetaProposals(ctx, cfg.FutGG.Cycle)
			if err != nil {
				return err
			}
			for _, proposal := range entries {
				if proposal.ID == proposalID && proposal.Status == "ativada" {
					proposal.Status, proposal.AtualizadaEm = "revertida", time.Now().UTC().Format(time.RFC3339)
					if err := backend.SaveMetaProposal(ctx, proposal); err != nil {
						return err
					}
					break
				}
			}
		}
	}
	label := previous.ExternalSource
	if previous.UseBot {
		label = previous.Profile
	}
	fmt.Printf("régua anterior restaurada: %s\n", label)
	return nil
}

func profileHistoryPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "historico-perfis.json")
}

func loadProfileHistory(path string) (profileHistory, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return profileHistory{}, nil
	}
	if err != nil {
		return profileHistory{}, fmt.Errorf("lendo histórico de perfis: %w", err)
	}
	var history profileHistory
	if err := json.Unmarshal(b, &history); err != nil {
		return profileHistory{}, fmt.Errorf("lendo histórico de perfis: %w", err)
	}
	return history, nil
}

func saveProfileHistory(path string, history profileHistory) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("codificando histórico de perfis: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("gravando histórico de perfis: %w", err)
	}
	return nil
}
