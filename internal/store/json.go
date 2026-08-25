package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// JSONStore guarda tudo em arquivos JSON num diretório. É o padrão: o bot
// funciona no primeiro dia sem Postgres, sem Docker, sem nada instalado.
// Para histórico longo e consulta, use PostgresStore.
type JSONStore struct {
	dir string
	mu  sync.Mutex
}

// NewJSON abre (ou cria) o diretório de dados.
func NewJSON(dir string) (*JSONStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando diretório de dados: %w", err)
	}
	return &JSONStore{dir: dir}, nil
}

func (s *JSONStore) path(name string) string { return filepath.Join(s.dir, name) }

func (s *JSONStore) readJSON(name string, dst any) error {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}

// writeJSON grava de forma atômica: escreve num temporário e renomeia,
// para uma execução interrompida nunca deixar arquivo pela metade.
func (s *JSONStore) writeJSON(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path(name) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path(name))
}

func pricesFile(cycle string) string { return "prices_" + cycle + ".json" }
func clubFile(cycle string) string   { return "club_" + cycle + ".json" }
func seenFile(cycle string) string   { return "seen_" + cycle + ".json" }

// snapshotRetention é quantos dias de snapshot ficam guardados — o bastante
// para o gráfico de tendência de 30 dias do status diário, sem deixar o
// diretório de dados crescer para sempre.
const snapshotRetention = 30

func snapshotsDir(cycle string) string { return filepath.Join("snapshots", cycle) }
func snapshotFile(cycle string, day time.Time) string {
	return filepath.Join(snapshotsDir(cycle), day.Format("2006-01-02")+".json")
}
func historyFile(cycle string) string { return filepath.Join(snapshotsDir(cycle), "history.json") }

type priceHistory struct {
	Points map[string][]PricePoint `json:"points"` // ea_id -> série
}

func (s *JSONStore) SavePrices(ctx context.Context, cycle string, players []domain.Player) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hist := priceHistory{Points: map[string][]PricePoint{}}
	_ = s.readJSON(pricesFile(cycle), &hist)
	if hist.Points == nil {
		hist.Points = map[string][]PricePoint{}
	}

	now := time.Now()
	cutoff := now.Add(-60 * 24 * time.Hour) // guarda 60 dias

	for _, p := range players {
		if p.Price.Coins == 0 && !p.Price.Extinct {
			continue
		}
		key := fmt.Sprint(p.ID)
		series := hist.Points[key]

		// Não grava dois pontos no mesmo período de 1h: a coleta diária
		// pode rodar mais de uma vez sem inflar o arquivo.
		if n := len(series); n > 0 && now.Sub(series[n-1].ObservedAt) < time.Hour {
			continue
		}
		series = append(series, PricePoint{
			EAID: p.ID, Coins: p.Price.Coins, Extinct: p.Price.Extinct, ObservedAt: now,
		})

		// Poda pontos antigos.
		keep := series[:0]
		for _, pt := range series {
			if pt.ObservedAt.After(cutoff) {
				keep = append(keep, pt)
			}
		}
		hist.Points[key] = keep
	}
	return s.writeJSON(pricesFile(cycle), hist)
}

func (s *JSONStore) Trends(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64]PriceTrend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var hist priceHistory
	if err := s.readJSON(pricesFile(cycle), &hist); err != nil {
		return map[int64]PriceTrend{}, nil // sem histórico ainda não é erro
	}

	want := make(map[int64]bool, len(eaIDs))
	for _, id := range eaIDs {
		want[id] = true
	}
	cutoff := time.Now().Add(-since)
	out := make(map[int64]PriceTrend, len(eaIDs))

	for key, series := range hist.Points {
		var id int64
		if _, err := fmt.Sscan(key, &id); err != nil {
			continue
		}
		if len(want) > 0 && !want[id] {
			continue
		}

		var inWindow []PricePoint
		for _, pt := range series {
			if pt.ObservedAt.After(cutoff) && pt.Coins > 0 {
				inWindow = append(inWindow, pt)
			}
		}
		if len(inWindow) < 2 {
			continue
		}
		sort.Slice(inWindow, func(i, j int) bool {
			return inWindow[i].ObservedAt.Before(inWindow[j].ObservedAt)
		})

		t := PriceTrend{
			EAID:    id,
			First:   inWindow[0].Coins,
			Last:    inWindow[len(inWindow)-1].Coins,
			Min:     inWindow[0].Coins,
			Max:     inWindow[0].Coins,
			Samples: len(inWindow),
		}
		for _, pt := range inWindow {
			if pt.Coins < t.Min {
				t.Min = pt.Coins
			}
			if pt.Coins > t.Max {
				t.Max = pt.Coins
			}
		}
		if t.First > 0 {
			t.ChangePct = (float64(t.Last-t.First) / float64(t.First)) * 100
		}
		out[id] = t
	}
	return out, nil
}

func (s *JSONStore) SaveClub(ctx context.Context, club domain.Club) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Preserva o retrato anterior antes de sobrescrever, para o diff.
	if prev, err := os.ReadFile(s.path(clubFile(club.Cycle))); err == nil {
		_ = os.WriteFile(s.path("prev_"+clubFile(club.Cycle)), prev, 0o644)
	}
	return s.writeJSON(clubFile(club.Cycle), club)
}

func (s *JSONStore) PreviousClub(ctx context.Context, gamerTag, cycle string) (domain.Club, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var club domain.Club
	if err := s.readJSON("prev_"+clubFile(cycle), &club); err != nil {
		return domain.Club{}, false, nil
	}
	return club, true, nil
}

type seenState struct {
	Players map[string]time.Time `json:"players"`
	News    map[string]time.Time `json:"news"`
}

func (s *JSONStore) loadSeen(cycle string) seenState {
	st := seenState{Players: map[string]time.Time{}, News: map[string]time.Time{}}
	_ = s.readJSON(seenFile(cycle), &st)
	if st.Players == nil {
		st.Players = map[string]time.Time{}
	}
	if st.News == nil {
		st.News = map[string]time.Time{}
	}
	return st
}

func (s *JSONStore) NewPlayers(ctx context.Context, cycle string, players []domain.Player) ([]domain.Player, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.loadSeen(cycle)
	firstRun := len(st.Players) == 0
	now := time.Now()

	var fresh []domain.Player
	for _, p := range players {
		key := fmt.Sprint(p.ID)
		if _, seen := st.Players[key]; !seen {
			st.Players[key] = now
			if !firstRun {
				fresh = append(fresh, p)
			}
		}
	}
	if err := s.writeJSON(seenFile(cycle), st); err != nil {
		return fresh, err
	}
	return fresh, nil
}

func (s *JSONStore) UnseenNews(ctx context.Context, cycle string, news []domain.NewsItem) ([]domain.NewsItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.loadSeen(cycle)
	firstRun := len(st.News) == 0
	now := time.Now()

	var fresh []domain.NewsItem
	for _, n := range news {
		key := n.ID
		if key == "" {
			key = n.URL
		}
		if key == "" {
			key = n.Title
		}
		if _, seen := st.News[key]; !seen {
			st.News[key] = now
			// Na primeira execução mostra as 5 mais recentes em vez de tudo.
			fresh = append(fresh, n)
		}
	}
	if firstRun && len(fresh) > 5 {
		sort.Slice(fresh, func(i, j int) bool {
			return fresh[i].PublishedAt.After(fresh[j].PublishedAt)
		})
		fresh = fresh[:5]
	}
	if err := s.writeJSON(seenFile(cycle), st); err != nil {
		return fresh, err
	}
	return fresh, nil
}

// SaveSnapshot grava o snapshot do dia, poda o que passou dos 30 dias de
// retenção e mantém o resumo leve (SnapshotHistory) em dia — um arquivo por
// ciclo, para o gráfico de tendência não exigir reler o histórico inteiro.
func (s *JSONStore) SaveSnapshot(ctx context.Context, snap Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	day := snap.GeneratedAt
	if day.IsZero() {
		day = time.Now()
	}
	name := snapshotFile(snap.Cycle, day)
	if err := os.MkdirAll(filepath.Dir(s.path(name)), 0o755); err != nil {
		return fmt.Errorf("criando diretório de snapshots: %w", err)
	}
	if err := s.writeJSON(name, snap); err != nil {
		return err
	}
	if err := s.pruneSnapshots(snap.Cycle); err != nil {
		return err
	}
	return s.appendHistory(snap.Cycle, SnapshotSummary{
		Date:       day.Format("2006-01-02"),
		SquadScore: snap.SquadScore,
		Coins:      snap.Club.Coins,
	})
}

// pruneSnapshots mantém só os `snapshotRetention` arquivos mais recentes —
// por NOME (a data no arquivo já ordena lexicograficamente), não por
// relógio: assim o teste de poda não depende de injetar hora nenhuma.
func (s *JSONStore) pruneSnapshots(cycle string) error {
	dir := s.path(snapshotsDir(cycle))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // diretório recém-criado, nada para podar
	}
	var dates []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "history.json" {
			continue
		}
		dates = append(dates, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	cut := snapshotRetention
	if len(dates) < cut {
		cut = len(dates)
	}
	for _, name := range dates[cut:] {
		_ = os.Remove(filepath.Join(dir, name))
	}
	return nil
}

// appendHistory faz upsert por data: rodar `run` duas vezes no mesmo dia
// atualiza o ponto em vez de duplicar.
func (s *JSONStore) appendHistory(cycle string, sum SnapshotSummary) error {
	name := historyFile(cycle)
	var hist []SnapshotSummary
	_ = s.readJSON(name, &hist)

	replaced := false
	for i, h := range hist {
		if h.Date == sum.Date {
			hist[i] = sum
			replaced = true
			break
		}
	}
	if !replaced {
		hist = append(hist, sum)
	}
	sort.Slice(hist, func(i, j int) bool { return hist[i].Date < hist[j].Date })
	if len(hist) > snapshotRetention {
		hist = hist[len(hist)-snapshotRetention:]
	}
	return s.writeJSON(name, hist)
}

func (s *JSONStore) LatestSnapshot(ctx context.Context, cycle string) (Snapshot, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.path(snapshotsDir(cycle))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Snapshot{}, false, nil
	}
	var latest string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "history.json" {
			continue
		}
		if e.Name() > latest {
			latest = e.Name()
		}
	}
	if latest == "" {
		return Snapshot{}, false, nil
	}
	var snap Snapshot
	if err := s.readJSON(filepath.Join(snapshotsDir(cycle), latest), &snap); err != nil {
		return Snapshot{}, false, err
	}
	return snap, true, nil
}

func (s *JSONStore) SnapshotHistory(ctx context.Context, cycle string, days int) ([]SnapshotSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var hist []SnapshotSummary
	if err := s.readJSON(historyFile(cycle), &hist); err != nil {
		return nil, nil // sem histórico ainda não é erro
	}
	if days > 0 && len(hist) > days {
		hist = hist[len(hist)-days:]
	}
	return hist, nil
}

// ClubHistory lê só o campo "club" de cada snapshot do período. Decodifica
// num struct enxuto de propósito: o snapshot inteiro passa de 30 MB e o resto
// (mercado, cartas) não interessa para calibrar entrosamento.
func (s *JSONStore) ClubHistory(ctx context.Context, cycle string, days int) ([]domain.Club, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := s.path(snapshotsDir(cycle))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil // sem snapshot ainda não é erro
	}
	var nomes []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "history.json" {
			continue
		}
		nomes = append(nomes, e.Name())
	}
	sort.Strings(nomes) // a data no nome já ordena: mais antigo -> mais novo
	if days > 0 && len(nomes) > days {
		nomes = nomes[len(nomes)-days:]
	}

	out := make([]domain.Club, 0, len(nomes))
	for _, nome := range nomes {
		var envelope struct {
			Club domain.Club `json:"club"`
		}
		if err := s.readJSON(filepath.Join(snapshotsDir(cycle), nome), &envelope); err != nil {
			continue // um dia ilegível não derruba a calibração inteira
		}
		out = append(out, envelope.Club)
	}
	return out, nil
}

// PriceSeries é o mesmo histórico que Trends já lê, sem colapsar num resumo
// — a série ponto a ponto que os gráficos de preço por carta precisam.
func (s *JSONStore) PriceSeries(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64][]PricePoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var hist priceHistory
	if err := s.readJSON(pricesFile(cycle), &hist); err != nil {
		return map[int64][]PricePoint{}, nil
	}

	want := make(map[int64]bool, len(eaIDs))
	for _, id := range eaIDs {
		want[id] = true
	}
	cutoff := time.Now().Add(-since)
	out := make(map[int64][]PricePoint, len(eaIDs))

	for key, series := range hist.Points {
		var id int64
		if _, err := fmt.Sscan(key, &id); err != nil {
			continue
		}
		if len(want) > 0 && !want[id] {
			continue
		}
		var inWindow []PricePoint
		for _, pt := range series {
			if pt.ObservedAt.After(cutoff) {
				inWindow = append(inWindow, pt)
			}
		}
		if len(inWindow) == 0 {
			continue
		}
		sort.Slice(inWindow, func(i, j int) bool {
			return inWindow[i].ObservedAt.Before(inWindow[j].ObservedAt)
		})
		out[id] = inWindow
	}
	return out, nil
}

func sbcCostFile(cycle string) string { return "sbc_cost_" + cycle + ".json" }

type sbcCostHistory struct {
	Points map[string][]SBCCostPoint `json:"points"` // challenge key -> série
}

// SaveSBCCost espelha SavePrices: mesmo dedupe de 1h, mesma retenção de 60
// dias, só que a chave é SBCChallengeKey em vez do id da carta.
func (s *JSONStore) SaveSBCCost(ctx context.Context, cycle string, sbcs []domain.SBC) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hist := sbcCostHistory{Points: map[string][]SBCCostPoint{}}
	_ = s.readJSON(sbcCostFile(cycle), &hist)
	if hist.Points == nil {
		hist.Points = map[string][]SBCCostPoint{}
	}

	now := time.Now()
	cutoff := now.Add(-60 * 24 * time.Hour) // mesma retenção de prices_<cycle>.json

	for _, sbc := range sbcs {
		for idx, ch := range sbc.Challenges {
			if ch.CheapestSolutionCoins <= 0 {
				continue
			}
			key := SBCChallengeKey(sbc.ID, idx, ch.Name)
			series := hist.Points[key]

			if n := len(series); n > 0 && now.Sub(series[n-1].ObservedAt) < time.Hour {
				continue
			}
			series = append(series, SBCCostPoint{Key: key, Coins: ch.CheapestSolutionCoins, ObservedAt: now})

			keep := series[:0]
			for _, pt := range series {
				if pt.ObservedAt.After(cutoff) {
					keep = append(keep, pt)
				}
			}
			hist.Points[key] = keep
		}
	}
	return s.writeJSON(sbcCostFile(cycle), hist)
}

func (s *JSONStore) SBCCostTrend(ctx context.Context, cycle string, keys []string, since time.Duration) (map[string]PriceTrend, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var hist sbcCostHistory
	if err := s.readJSON(sbcCostFile(cycle), &hist); err != nil {
		return map[string]PriceTrend{}, nil // sem histórico ainda não é erro
	}

	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[k] = true
	}
	cutoff := time.Now().Add(-since)
	out := make(map[string]PriceTrend, len(keys))

	for key, series := range hist.Points {
		if len(want) > 0 && !want[key] {
			continue
		}
		var inWindow []SBCCostPoint
		for _, pt := range series {
			if pt.ObservedAt.After(cutoff) && pt.Coins > 0 {
				inWindow = append(inWindow, pt)
			}
		}
		if len(inWindow) < 2 {
			continue
		}
		sort.Slice(inWindow, func(i, j int) bool {
			return inWindow[i].ObservedAt.Before(inWindow[j].ObservedAt)
		})

		t := PriceTrend{
			First:   inWindow[0].Coins,
			Last:    inWindow[len(inWindow)-1].Coins,
			Min:     inWindow[0].Coins,
			Max:     inWindow[0].Coins,
			Samples: len(inWindow),
		}
		for _, pt := range inWindow {
			if pt.Coins < t.Min {
				t.Min = pt.Coins
			}
			if pt.Coins > t.Max {
				t.Max = pt.Coins
			}
		}
		if t.First > 0 {
			t.ChangePct = (float64(t.Last-t.First) / float64(t.First)) * 100
		}
		out[key] = t
	}
	return out, nil
}

func momentumFile(cycle string) string { return "momentum_" + cycle + ".json" }

// SaveMomentum sobrescreve o cache — não acumula histórico, é sempre o
// último valor lido (ver o comentário do método na interface Store).
func (s *JSONStore) SaveMomentum(ctx context.Context, cycle string, momentum []domain.Player) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON(momentumFile(cycle), momentum)
}

func (s *JSONStore) LatestMomentum(ctx context.Context, cycle string) ([]domain.Player, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var momentum []domain.Player
	if err := s.readJSON(momentumFile(cycle), &momentum); err != nil {
		return nil, nil // ciclo rápido ainda não rodou não é erro
	}
	return momentum, nil
}

func (s *JSONStore) Close() error { return nil }

var _ Store = (*JSONStore)(nil)
