package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/galeria"
)

// PostgresStore usa só database/sql: o driver entra por blank import no
// pacote main (veja cmd/eafcbot/driver_pgx.go), então este pacote continua
// sem dependência externa e testável sem banco.
type PostgresStore struct {
	db        *sql.DB
	retention int
}

// OpenPostgres abre a conexão. driverName costuma ser "pgx" ou "postgres".
func OpenPostgres(ctx context.Context, driverName, dsn string) (*PostgresStore, error) {
	return OpenPostgresWithRetention(ctx, driverName, dsn, snapshotRetention)
}

// OpenPostgresWithRetention mantém a poda do banco alinhada a
// serve.retention_days. O construtor antigo continua com 30 dias.
func OpenPostgresWithRetention(ctx context.Context, driverName, dsn string, retention int) (*PostgresStore, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("abrindo conexão: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("conectando no Postgres: %w", err)
	}
	if retention <= 0 {
		retention = snapshotRetention
	}
	return &PostgresStore{db: db, retention: retention}, nil
}

func (s *PostgresStore) Close() error { return s.db.Close() }

// SavePrices grava as cotações e mantém a tabela players atualizada,
// tudo numa transação para o relatório nunca ler estado pela metade.
func (s *PostgresStore) SavePrices(ctx context.Context, cycle string, players []domain.Player) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	upsert, err := tx.PrepareContext(ctx, `
		INSERT INTO players (ea_id, cycle, name, common_name, rating, position, alt_positions,
		                     version, club, league, nation,
		                     pace, shooting, passing, dribbling, defending, physical,
		                     play_styles, weak_foot, skill_moves, last_seen)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20, now())
		ON CONFLICT (ea_id, cycle) DO UPDATE SET
			rating = EXCLUDED.rating,
			play_styles = EXCLUDED.play_styles,
			last_seen = now()`)
	if err != nil {
		return err
	}
	defer upsert.Close()

	tick, err := tx.PrepareContext(ctx, `
		INSERT INTO price_ticks (ea_id, cycle, observed_at, coins, extinct)
		VALUES ($1, $2, date_trunc('hour', now()), $3, $4)
		ON CONFLICT (ea_id, cycle, observed_at) DO UPDATE SET
			coins = EXCLUDED.coins, extinct = EXCLUDED.extinct`)
	if err != nil {
		return err
	}
	defer tick.Close()

	for _, p := range players {
		ps, err := json.Marshal(p.PlayStyles)
		if err != nil {
			return err
		}
		alts := make([]string, 0, len(p.AltPositions))
		for _, a := range p.AltPositions {
			alts = append(alts, string(a))
		}
		if _, err := upsert.ExecContext(ctx,
			p.ID, cycle, p.Name, p.CommonName, p.Rating, string(p.Position), pgArray(alts),
			p.Version, p.Club, p.League, p.Nation,
			p.Attributes.Pace, p.Attributes.Shooting, p.Attributes.Passing,
			p.Attributes.Dribbling, p.Attributes.Defending, p.Attributes.Physical,
			ps, p.WeakFoot, p.SkillMoves,
		); err != nil {
			return fmt.Errorf("upsert do jogador %d: %w", p.ID, err)
		}
		if p.Price.Coins > 0 || p.Price.Extinct {
			if _, err := tick.ExecContext(ctx, p.ID, cycle, p.Price.Coins, p.Price.Extinct); err != nil {
				return fmt.Errorf("tick de preço do jogador %d: %w", p.ID, err)
			}
		}
	}
	return tx.Commit()
}

// pgArray formata []string como literal de array do Postgres.
func pgArray(items []string) string {
	if len(items) == 0 {
		return "{}"
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = `"` + strings.ReplaceAll(it, `"`, `\"`) + `"`
	}
	return "{" + strings.Join(quoted, ",") + "}"
}

func (s *PostgresStore) Trends(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64]PriceTrend, error) {
	// Uma query só resolve tudo: primeiro e último preço da janela vêm de
	// funções de janela, min/max e contagem de agregação.
	const q = `
		WITH win AS (
			SELECT ea_id, coins, observed_at,
			       first_value(coins) OVER w  AS first_coins,
			       last_value(coins)  OVER w  AS last_coins
			FROM price_ticks
			WHERE cycle = $1
			  AND observed_at >= now() - $2::interval
			  AND coins > 0
			  AND ($3::bigint[] IS NULL OR ea_id = ANY($3))
			WINDOW w AS (PARTITION BY ea_id ORDER BY observed_at
			             ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING)
		)
		SELECT ea_id, min(first_coins), min(last_coins), min(coins), max(coins), count(*)
		FROM win GROUP BY ea_id HAVING count(*) >= 2`

	var idArg any
	if len(eaIDs) > 0 {
		parts := make([]string, len(eaIDs))
		for i, id := range eaIDs {
			parts[i] = fmt.Sprint(id)
		}
		idArg = "{" + strings.Join(parts, ",") + "}"
	}

	rows, err := s.db.QueryContext(ctx, q, cycle, since.String(), idArg)
	if err != nil {
		return nil, fmt.Errorf("consultando tendências: %w", err)
	}
	defer rows.Close()

	out := map[int64]PriceTrend{}
	for rows.Next() {
		var t PriceTrend
		if err := rows.Scan(&t.EAID, &t.First, &t.Last, &t.Min, &t.Max, &t.Samples); err != nil {
			return nil, err
		}
		if t.First > 0 {
			t.ChangePct = (float64(t.Last-t.First) / float64(t.First)) * 100
		}
		out[t.EAID] = t
	}
	return out, rows.Err()
}

func (s *PostgresStore) SaveClub(ctx context.Context, club domain.Club) error {
	payload, err := json.Marshal(club)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO club_snapshots (gamer_tag, cycle, synced_at, coins, payload)
		VALUES ($1, $2, COALESCE($3, now()), $4, $5)`,
		club.GamerTag, club.Cycle, nullTime(club.SyncedAt), club.Coins, payload)
	return err
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func (s *PostgresStore) PreviousClub(ctx context.Context, gamerTag, cycle string) (domain.Club, bool, error) {
	// OFFSET 1: queremos o penúltimo, porque o último é o desta execução.
	var payload []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT payload FROM club_snapshots
		WHERE gamer_tag = $1 AND cycle = $2
		ORDER BY synced_at DESC OFFSET 1 LIMIT 1`, gamerTag, cycle).Scan(&payload)
	if err == sql.ErrNoRows {
		return domain.Club{}, false, nil
	}
	if err != nil {
		return domain.Club{}, false, err
	}
	var club domain.Club
	if err := json.Unmarshal(payload, &club); err != nil {
		return domain.Club{}, false, err
	}
	return club, true, nil
}

func (s *PostgresStore) NewPlayers(ctx context.Context, cycle string, players []domain.Player) ([]domain.Player, error) {
	if len(players) == 0 {
		return nil, nil
	}
	// Uma carta é "nova" se ainda não existe na tabela players. Como
	// SavePrices roda depois, esta checagem vê o estado anterior.
	ids := make([]string, len(players))
	byID := make(map[int64]domain.Player, len(players))
	for i, p := range players {
		ids[i] = fmt.Sprint(p.ID)
		byID[p.ID] = p
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT ea_id FROM players WHERE cycle = $1 AND ea_id = ANY($2::bigint[])`,
		cycle, "{"+strings.Join(ids, ",")+"}")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	known := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		known[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Banco vazio: primeira execução, tudo é "novo" e nada deve ser alertado.
	if len(known) == 0 {
		return nil, nil
	}

	var fresh []domain.Player
	for id, p := range byID {
		if !known[id] {
			fresh = append(fresh, p)
		}
	}
	return fresh, nil
}

func (s *PostgresStore) UnseenNews(ctx context.Context, cycle string, news []domain.NewsItem) ([]domain.NewsItem, error) {
	var fresh []domain.NewsItem
	for _, n := range news {
		id := n.ID
		if id == "" {
			id = n.URL
		}
		if id == "" {
			id = n.Title
		}
		var inserted bool
		err := s.db.QueryRowContext(ctx, `
			INSERT INTO seen_news (id, cycle, title, url, published_at)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO NOTHING
			RETURNING true`, id, cycle, n.Title, n.URL, nullTime(n.PublishedAt)).Scan(&inserted)
		if err == sql.ErrNoRows {
			continue // já tinha sido reportada
		}
		if err != nil {
			return fresh, err
		}
		fresh = append(fresh, n)
	}
	return fresh, nil
}

// SaveSnapshot grava o snapshot do dia (upsert por cycle+day: rodar `run`
// duas vezes no mesmo dia atualiza o ponto em vez de duplicar) e poda o que
// passou dos snapshotRetention dias mais recentes daquele ciclo.
func (s *PostgresStore) SaveSnapshot(ctx context.Context, snap Snapshot) error {
	day := snap.GeneratedAt
	if day.IsZero() {
		day = time.Now()
	}
	payload, err := json.Marshal(snap)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO snapshots (cycle, day, generated_at, squad_score, coins, payload)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (cycle, day) DO UPDATE SET
			generated_at = EXCLUDED.generated_at,
			squad_score  = EXCLUDED.squad_score,
			coins        = EXCLUDED.coins,
			payload      = EXCLUDED.payload`,
		snap.Cycle, day.Format("2006-01-02"), day, snap.SquadScore, snap.Club.Coins, payload); err != nil {
		return fmt.Errorf("gravando snapshot: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM snapshots WHERE cycle = $1 AND day NOT IN (
			SELECT day FROM snapshots WHERE cycle = $1 ORDER BY day DESC LIMIT $2)`,
		snap.Cycle, s.retention); err != nil {
		return fmt.Errorf("podando snapshots antigos: %w", err)
	}
	return tx.Commit()
}

func (s *PostgresStore) LatestSnapshot(ctx context.Context, cycle string) (Snapshot, bool, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT payload FROM snapshots WHERE cycle = $1 ORDER BY day DESC LIMIT 1`, cycle).Scan(&payload)
	if err == sql.ErrNoRows {
		return Snapshot{}, false, nil
	}
	if err != nil {
		return Snapshot{}, false, err
	}
	var snap Snapshot
	if err := json.Unmarshal(payload, &snap); err != nil {
		return Snapshot{}, false, err
	}
	return snap, true, nil
}

func (s *PostgresStore) SnapshotHistory(ctx context.Context, cycle string, days int) ([]SnapshotSummary, error) {
	if days <= 0 {
		days = s.retention
	}
	// A subconsulta pega os `days` mais recentes; a de fora reordena
	// crescente, porque é a ordem que o gráfico de tendência consome.
	rows, err := s.db.QueryContext(ctx, `
		SELECT day, squad_score, coins FROM (
			SELECT day, squad_score, coins FROM snapshots
			WHERE cycle = $1 ORDER BY day DESC LIMIT $2
		) recent ORDER BY day ASC`, cycle, days)
	if err != nil {
		return nil, fmt.Errorf("consultando histórico de snapshots: %w", err)
	}
	defer rows.Close()

	var out []SnapshotSummary
	for rows.Next() {
		var day time.Time
		var sum SnapshotSummary
		if err := rows.Scan(&day, &sum.SquadScore, &sum.Coins); err != nil {
			return nil, err
		}
		sum.Date = day.Format("2006-01-02")
		out = append(out, sum)
	}
	return out, rows.Err()
}

// ClubHistory extrai só o "club" de cada payload. Barato aqui, diferente do
// JSONStore: o payload já é JSONB, então o banco devolve o sub-objeto sem o
// cliente precisar ler o snapshot inteiro.
func (s *PostgresStore) ClubHistory(ctx context.Context, cycle string, days int) ([]domain.Club, error) {
	if days <= 0 {
		days = s.retention
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT club FROM (
			SELECT day, payload->'club' AS club FROM snapshots
			WHERE cycle = $1 ORDER BY day DESC LIMIT $2
		) recent ORDER BY day ASC`, cycle, days)
	if err != nil {
		return nil, fmt.Errorf("consultando histórico de clubes: %w", err)
	}
	defer rows.Close()

	var out []domain.Club
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var club domain.Club
		if err := json.Unmarshal(raw, &club); err != nil {
			continue // um dia ilegível não derruba a calibração inteira
		}
		out = append(out, club)
	}
	return out, rows.Err()
}

// PriceSeries é o mesmo price_ticks que Trends já consulta, sem agregar num
// resumo — a série ponto a ponto que os gráficos de preço por carta usam.
func (s *PostgresStore) PriceSeries(ctx context.Context, cycle string, eaIDs []int64, since time.Duration) (map[int64][]PricePoint, error) {
	var idArg any
	if len(eaIDs) > 0 {
		parts := make([]string, len(eaIDs))
		for i, id := range eaIDs {
			parts[i] = fmt.Sprint(id)
		}
		idArg = "{" + strings.Join(parts, ",") + "}"
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT ea_id, coins, extinct, observed_at FROM price_ticks
		WHERE cycle = $1
		  AND observed_at >= now() - $2::interval
		  AND ($3::bigint[] IS NULL OR ea_id = ANY($3))
		ORDER BY ea_id, observed_at ASC`, cycle, since.String(), idArg)
	if err != nil {
		return nil, fmt.Errorf("consultando série de preços: %w", err)
	}
	defer rows.Close()

	out := map[int64][]PricePoint{}
	for rows.Next() {
		var pt PricePoint
		if err := rows.Scan(&pt.EAID, &pt.Coins, &pt.Extinct, &pt.ObservedAt); err != nil {
			return nil, err
		}
		out[pt.EAID] = append(out[pt.EAID], pt)
	}
	return out, rows.Err()
}

// SaveSBCCost grava o custo da solução mais barata de cada challenge —
// mesmo padrão de bucket por hora de SavePrices/price_ticks
// (date_trunc('hour', now()) com ON CONFLICT DO UPDATE), só que a chave é
// SBCChallengeKey em vez de ea_id.
func (s *PostgresStore) SaveSBCCost(ctx context.Context, cycle string, sbcs []domain.SBC) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tick, err := tx.PrepareContext(ctx, `
		INSERT INTO sbc_cost_ticks (challenge_key, cycle, observed_at, coins)
		VALUES ($1, $2, date_trunc('hour', now()), $3)
		ON CONFLICT (challenge_key, cycle, observed_at) DO UPDATE SET
			coins = EXCLUDED.coins`)
	if err != nil {
		return err
	}
	defer tick.Close()

	for _, sbc := range sbcs {
		for idx, ch := range sbc.Challenges {
			if ch.CheapestSolutionCoins <= 0 {
				continue
			}
			key := SBCChallengeKey(sbc.ID, idx, ch.Name)
			if _, err := tick.ExecContext(ctx, key, cycle, ch.CheapestSolutionCoins); err != nil {
				return fmt.Errorf("tick de custo do challenge %q: %w", key, err)
			}
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) SBCCostTrend(ctx context.Context, cycle string, keys []string, since time.Duration) (map[string]PriceTrend, error) {
	const q = `
		WITH win AS (
			SELECT challenge_key, coins, observed_at,
			       first_value(coins) OVER w  AS first_coins,
			       last_value(coins)  OVER w  AS last_coins
			FROM sbc_cost_ticks
			WHERE cycle = $1
			  AND observed_at >= now() - $2::interval
			  AND coins > 0
			  AND ($3::text[] IS NULL OR challenge_key = ANY($3))
			WINDOW w AS (PARTITION BY challenge_key ORDER BY observed_at
			             ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING)
		)
		SELECT challenge_key, min(first_coins), min(last_coins), min(coins), max(coins), count(*)
		FROM win GROUP BY challenge_key HAVING count(*) >= 2`

	var keyArg any
	if len(keys) > 0 {
		quoted := make([]string, len(keys))
		for i, k := range keys {
			quoted[i] = `"` + strings.ReplaceAll(k, `"`, `\"`) + `"`
		}
		keyArg = "{" + strings.Join(quoted, ",") + "}"
	}

	rows, err := s.db.QueryContext(ctx, q, cycle, since.String(), keyArg)
	if err != nil {
		return nil, fmt.Errorf("consultando tendência de custo de SBC: %w", err)
	}
	defer rows.Close()

	out := map[string]PriceTrend{}
	for rows.Next() {
		var key string
		var t PriceTrend
		if err := rows.Scan(&key, &t.First, &t.Last, &t.Min, &t.Max, &t.Samples); err != nil {
			return nil, err
		}
		if t.First > 0 {
			t.ChangePct = (float64(t.Last-t.First) / float64(t.First)) * 100
		}
		out[key] = t
	}
	return out, rows.Err()
}

// SaveMomentum faz upsert do cache — não é série temporal, é sempre o
// último valor lido (ver o comentário do método na interface Store).
func (s *PostgresStore) SaveMomentum(ctx context.Context, cycle string, momentum []domain.Player) error {
	payload, err := json.Marshal(momentum)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO momentum_cache (cycle, updated_at, payload)
		VALUES ($1, now(), $2)
		ON CONFLICT (cycle) DO UPDATE SET updated_at = now(), payload = EXCLUDED.payload`,
		cycle, payload)
	return err
}

func (s *PostgresStore) LatestMomentum(ctx context.Context, cycle string) ([]domain.Player, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM momentum_cache WHERE cycle = $1`, cycle).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil // ciclo rápido ainda não rodou não é erro
	}
	if err != nil {
		return nil, err
	}
	var momentum []domain.Player
	if err := json.Unmarshal(payload, &momentum); err != nil {
		return nil, err
	}
	return momentum, nil
}

func (s *PostgresStore) ListWatchlist(ctx context.Context, cycle string) ([]domain.WatchlistEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM market_watchlist WHERE cycle = $1 ORDER BY updated_at DESC`, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.WatchlistEntry
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var entry domain.WatchlistEntry
		if err := json.Unmarshal(payload, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *PostgresStore) UpsertWatchlist(ctx context.Context, cycle string, entry domain.WatchlistEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	now := time.Now()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO market_watchlist (cycle, id, updated_at, payload) VALUES ($1,$2,$3,$4)
		ON CONFLICT (cycle,id) DO UPDATE SET updated_at = EXCLUDED.updated_at, payload = EXCLUDED.payload`, cycle, entry.ID, entry.UpdatedAt, payload)
	return err
}

func (s *PostgresStore) DeleteWatchlist(ctx context.Context, cycle, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM market_watchlist WHERE cycle = $1 AND id = $2`, cycle, id)
	return err
}

func (s *PostgresStore) ListLedger(ctx context.Context, cycle string) ([]domain.LedgerEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM market_ledger WHERE cycle = $1 ORDER BY recorded_at DESC`, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []domain.LedgerEntry
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var entry domain.LedgerEntry
		if err := json.Unmarshal(payload, &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *PostgresStore) AppendLedger(ctx context.Context, cycle string, entry domain.LedgerEntry) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now()
	}
	if entry.OccurredAt.IsZero() {
		entry.OccurredAt = entry.RecordedAt
	}
	if entry.Kind == domain.LedgerReversao {
		var exists bool
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM market_ledger WHERE cycle = $1 AND id = $2)`, cycle, entry.ReversesID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("ledger: lançamento a reverter %q não existe neste ciclo", entry.ReversesID)
		}
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO market_ledger (cycle, id, recorded_at, payload) VALUES ($1,$2,$3,$4)`, cycle, entry.ID, entry.RecordedAt, payload)
	if err != nil && strings.Contains(err.Error(), "duplicate") {
		return fmt.Errorf("ledger: id %q já existe; use reversão para corrigir", entry.ID)
	}
	return err
}

func (s *PostgresStore) SaveClubRollup(ctx context.Context, cycle string, rollup domain.ClubRollup) error {
	if rollup.ObservedAt.IsZero() {
		rollup.ObservedAt = time.Now()
	}
	rollup.Cycle = cycle
	payload, err := json.Marshal(rollup)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO club_rollups (cycle, day, observed_at, payload)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (cycle,day) DO UPDATE SET observed_at = EXCLUDED.observed_at, payload = EXCLUDED.payload`,
		cycle, rollup.ObservedAt.Format("2006-01-02"), rollup.ObservedAt, payload)
	return err
}

func (s *PostgresStore) ClubRollups(ctx context.Context, cycle string, days int) ([]domain.ClubRollup, error) {
	if days <= 0 {
		days = clubRollupRetention
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM (
		SELECT day, payload FROM club_rollups WHERE cycle = $1 ORDER BY day DESC LIMIT $2
	) recent ORDER BY day ASC`, cycle, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ClubRollup
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var rollup domain.ClubRollup
		if err := json.Unmarshal(payload, &rollup); err != nil {
			return nil, err
		}
		out = append(out, rollup)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListEvolutionAnalyses(ctx context.Context, cycle, inputHash string) ([]domain.EvolutionAnalysis, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT payload FROM evolution_analyses
		WHERE cycle = $1 AND ($2 = '' OR input_hash = $2)
		ORDER BY updated_at DESC`, cycle, inputHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.EvolutionAnalysis
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var analysis domain.EvolutionAnalysis
		if err := json.Unmarshal(payload, &analysis); err != nil {
			return nil, err
		}
		out = append(out, analysis)
	}
	return out, rows.Err()
}

func (s *PostgresStore) SaveEvolutionAnalysis(ctx context.Context, analysis domain.EvolutionAnalysis) error {
	if analysis.ID == "" || analysis.Cycle == "" {
		return fmt.Errorf("análise de evolução sem id ou ciclo")
	}
	if analysis.UpdatedAt.IsZero() {
		analysis.UpdatedAt = time.Now()
	}
	if analysis.CreatedAt.IsZero() {
		analysis.CreatedAt = analysis.UpdatedAt
	}
	payload, err := json.Marshal(analysis)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO evolution_analyses
			(id, cycle, input_hash, status, created_at, updated_at, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at,
			payload = EXCLUDED.payload`,
		analysis.ID, analysis.Cycle, analysis.InputHash, analysis.Status,
		analysis.CreatedAt, analysis.UpdatedAt, payload)
	return err
}

func (s *PostgresStore) ListFeedback(ctx context.Context, cycle string) ([]domain.DecisionFeedback, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM decision_feedback WHERE cycle = $1 ORDER BY recorded_at`, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.DecisionFeedback
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var entry domain.DecisionFeedback
		if err := json.Unmarshal(payload, &entry); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (s *PostgresStore) AppendFeedback(ctx context.Context, cycle string, entry domain.DecisionFeedback) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	if entry.Cycle != cycle {
		return fmt.Errorf("ciclo do feedback difere da colecao")
	}
	if entry.RecordedAt.IsZero() {
		entry.RecordedAt = time.Now()
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO decision_feedback (cycle,id,recorded_at,payload) VALUES ($1,$2,$3,$4)`, cycle, entry.ID, entry.RecordedAt, payload)
	if err != nil && strings.Contains(err.Error(), "duplicate") {
		return fmt.Errorf("feedback %q ja existe; feedback e append-only", entry.ID)
	}
	return err
}

func (s *PostgresStore) ListSavedEvolutionPaths(ctx context.Context, cycle string) ([]domain.SavedEvolutionPath, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM saved_evolution_paths WHERE cycle = $1 ORDER BY saved_at DESC`, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SavedEvolutionPath
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var entry domain.SavedEvolutionPath
		if err := json.Unmarshal(payload, &entry); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (s *PostgresStore) SaveEvolutionPath(ctx context.Context, path domain.SavedEvolutionPath) error {
	if err := path.Validate(); err != nil {
		return err
	}
	if path.SavedAt.IsZero() {
		path.SavedAt = time.Now()
	}
	payload, err := json.Marshal(path)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO saved_evolution_paths (cycle,id,saved_at,payload) VALUES ($1,$2,$3,$4)
		ON CONFLICT (cycle,id) DO UPDATE SET saved_at = EXCLUDED.saved_at, payload = EXCLUDED.payload`, path.Cycle, path.ID, path.SavedAt, payload)
	return err
}

func (s *PostgresStore) DeleteSavedEvolutionPath(ctx context.Context, cycle, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM saved_evolution_paths WHERE cycle = $1 AND id = $2`, cycle, id)
	return err
}

func (s *PostgresStore) ListSavedSquadPlans(ctx context.Context, cycle, club string) ([]domain.PlanoElencoSalvo, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM saved_squad_plans
		WHERE cycle = $1 AND club = $2 ORDER BY is_reference DESC, updated_at DESC`, cycle, club)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PlanoElencoSalvo
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var plan domain.PlanoElencoSalvo
		if err := json.Unmarshal(payload, &plan); err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, rows.Err()
}

func (s *PostgresStore) SaveSquadPlan(ctx context.Context, plan domain.PlanoElencoSalvo) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if plan.CriadoEm.IsZero() {
		plan.CriadoEm = time.Now()
	}
	if plan.AtualizadoEm.IsZero() {
		plan.AtualizadoEm = plan.CriadoEm
	}
	payload, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO saved_squad_plans
		(cycle,club,id,is_reference,updated_at,payload) VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (cycle,club,id) DO UPDATE SET is_reference = EXCLUDED.is_reference,
		updated_at = EXCLUDED.updated_at, payload = EXCLUDED.payload`,
		plan.Ciclo, plan.Clube, plan.ID, plan.Referencia, plan.AtualizadoEm, payload)
	return err
}

func (s *PostgresStore) DeleteSavedSquadPlan(ctx context.Context, cycle, club, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM saved_squad_plans WHERE cycle = $1 AND club = $2 AND id = $3`, cycle, club, id)
	return err
}

func (s *PostgresStore) ListGameplayFeedback(ctx context.Context, cycle string) ([]domain.FeedbackGameplay, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM gameplay_feedback WHERE cycle = $1 ORDER BY registered_at`, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FeedbackGameplay
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var entry domain.FeedbackGameplay
		if err := json.Unmarshal(payload, &entry); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpsertGameplayFeedback(ctx context.Context, entry domain.FeedbackGameplay) error {
	if entry.Ciclo == "" || entry.ComparacaoID == "" || entry.ID == "" {
		return fmt.Errorf("feedback de gameplay precisa de ciclo, comparação e id")
	}
	registeredAt := time.Now().UTC()
	if entry.RegistradoEm != "" {
		parsed, err := time.Parse(time.RFC3339, entry.RegistradoEm)
		if err != nil {
			return fmt.Errorf("data do feedback de gameplay inválida: %w", err)
		}
		registeredAt = parsed
	} else {
		entry.RegistradoEm = registeredAt.Format(time.RFC3339)
	}
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO gameplay_feedback
		(cycle,comparison_id,id,registered_at,payload) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (cycle,comparison_id) DO UPDATE SET id = EXCLUDED.id,
		registered_at = EXCLUDED.registered_at, payload = EXCLUDED.payload`,
		entry.Ciclo, entry.ComparacaoID, entry.ID, registeredAt, payload)
	return err
}

func (s *PostgresStore) ListMetaProposals(ctx context.Context, cycle string) ([]domain.PropostaMeta, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM meta_proposals WHERE cycle = $1 ORDER BY created_at DESC`, cycle)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.PropostaMeta
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var proposal domain.PropostaMeta
		if err := json.Unmarshal(payload, &proposal); err != nil {
			return nil, err
		}
		out = append(out, proposal)
	}
	return out, rows.Err()
}

func (s *PostgresStore) SaveMetaProposal(ctx context.Context, proposal domain.PropostaMeta) error {
	if err := proposal.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if proposal.CriadaEm == "" {
		proposal.CriadaEm = now.Format(time.RFC3339)
	}
	if proposal.AtualizadaEm == "" {
		proposal.AtualizadaEm = proposal.CriadaEm
	}
	createdAt, err := time.Parse(time.RFC3339, proposal.CriadaEm)
	if err != nil {
		return fmt.Errorf("data de criação da proposta inválida: %w", err)
	}
	updatedAt, err := time.Parse(time.RFC3339, proposal.AtualizadaEm)
	if err != nil {
		return fmt.Errorf("data de atualização da proposta inválida: %w", err)
	}
	payload, err := json.Marshal(proposal)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO meta_proposals
		(cycle,id,package_hash,status,created_at,updated_at,payload) VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (cycle,id) DO UPDATE SET status = EXCLUDED.status,
		updated_at = EXCLUDED.updated_at, payload = EXCLUDED.payload
		WHERE meta_proposals.package_hash = EXCLUDED.package_hash`, proposal.Ciclo, proposal.ID,
		proposal.Pacote, proposal.Status, createdAt, updatedAt, payload)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return fmt.Errorf("proposta %q tentou trocar o pacote imutável", proposal.ID)
	}
	return nil
}

func (s *PostgresStore) ListGallery(ctx context.Context, cycle, club, platform string) ([]galeria.Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM gallery_records WHERE cycle=$1 AND club=$2 AND platform=$3 ORDER BY set_id`, cycle, club, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []galeria.Record{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var row galeria.Record
		if err := json.Unmarshal(b, &row); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
func (s *PostgresStore) SaveGallery(ctx context.Context, cycle, club, platform string, records []galeria.Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, row := range records {
		b, e := json.Marshal(row)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO gallery_records(cycle,club,platform,set_id,payload,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(cycle,club,platform,set_id) DO UPDATE SET payload=EXCLUDED.payload,updated_at=EXCLUDED.updated_at`, cycle, club, platform, row.Set.ID, b); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *PostgresStore) ListGalleryCards(ctx context.Context, cycle, club, platform string) ([]galeria.Card, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM gallery_cards WHERE cycle=$1 AND club=$2 AND platform=$3`, cycle, club, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []galeria.Card{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var c galeria.Card
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *PostgresStore) SaveGalleryCards(ctx context.Context, cycle, club, platform string, cards []galeria.Card) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range cards {
		b, e := json.Marshal(c)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO gallery_cards(cycle,club,platform,card_id,payload,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(cycle,club,platform,card_id) DO UPDATE SET payload=EXCLUDED.payload,updated_at=EXCLUDED.updated_at`, cycle, club, platform, c.ID, b); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *PostgresStore) ListGalleryOverrides(ctx context.Context, cycle, club, platform string) ([]galeria.CollectionOverride, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM gallery_overrides WHERE cycle=$1 AND club=$2 AND platform=$3`, cycle, club, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []galeria.CollectionOverride{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var x galeria.CollectionOverride
		if err := json.Unmarshal(b, &x); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (s *PostgresStore) SaveGalleryOverride(ctx context.Context, cycle, club, platform string, x galeria.CollectionOverride) error {
	b, err := json.Marshal(x)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO gallery_overrides(cycle,club,platform,card_id,payload,updated_at) VALUES($1,$2,$3,$4,$5,now()) ON CONFLICT(cycle,club,platform,card_id) DO UPDATE SET payload=EXCLUDED.payload,updated_at=EXCLUDED.updated_at`, cycle, club, platform, x.CardID, b)
	return err
}
func (s *PostgresStore) DeleteGalleryOverride(ctx context.Context, cycle, club, platform string, cardID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM gallery_overrides WHERE cycle=$1 AND club=$2 AND platform=$3 AND card_id=$4`, cycle, club, platform, cardID)
	return err
}
func (s *PostgresStore) SaveGalleryCompletion(ctx context.Context, cycle, club, platform string, c galeria.Completion) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO gallery_completions(cycle,club,platform,set_id,payload,completed_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(cycle,club,platform,set_id) DO UPDATE SET payload=EXCLUDED.payload,completed_at=EXCLUDED.completed_at`, cycle, club, platform, c.SetID, b, c.CompletedAt)
	return err
}
func (s *PostgresStore) ListGalleryCompletions(ctx context.Context, cycle, club, platform string) ([]galeria.Completion, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM gallery_completions WHERE cycle=$1 AND club=$2 AND platform=$3 ORDER BY set_id`, cycle, club, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []galeria.Completion{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var c galeria.Completion
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *PostgresStore) DeleteGalleryCompletion(ctx context.Context, cycle, club, platform, setID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM gallery_completions WHERE cycle=$1 AND club=$2 AND platform=$3 AND set_id=$4`, cycle, club, platform, setID)
	return err
}

var _ Store = (*PostgresStore)(nil)
var _ GaleriaStore = (*PostgresStore)(nil)
var _ SavedSquadPlanStore = (*PostgresStore)(nil)
var _ GameplayFeedbackStore = (*PostgresStore)(nil)
var _ MetaProposalStore = (*PostgresStore)(nil)
