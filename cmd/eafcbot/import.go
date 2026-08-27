package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// cmdImportClub aceita apenas um retrato LOCAL, JSON ou CSV. Nao autentica,
// nao abre navegador e nao conhece formato de sessao da EA; segredos sao
// rejeitados antes de qualquer decode para evitar que acabem em log/snapshot.
func cmdImportClub(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "club" {
		return fmt.Errorf("uso: eafcbot import club -file elenco.json|-file elenco.csv [-dry-run]")
	}
	fs := flag.NewFlagSet("import club", flag.ExitOnError)
	file := fs.String("file", "", "arquivo JSON ou CSV local")
	cfgPath := fs.String("config", config.DefaultPath(), "arquivo de configuracao")
	dry := fs.Bool("dry-run", false, "validar sem gravar")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return fmt.Errorf("informe -file com um JSON ou CSV exportado localmente")
	}
	b, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("lendo importacao local: %w", err)
	}
	if contemSegredo(b) {
		return fmt.Errorf("a importacao foi recusada: remova senha, cookie, token ou sessao EA do arquivo")
	}
	club, err := decodeClubImport(*file, b)
	if err != nil {
		return err
	}
	if err := validarClubImport(club); err != nil {
		return err
	}
	if *dry {
		fmt.Printf("importacao valida: %d cartas; nenhuma escrita foi feita\n", len(club.Players))
		return nil
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	if club.Cycle == "" {
		club.Cycle = cfg.FutGG.Cycle
	}
	if club.Cycle == "" {
		return fmt.Errorf("ciclo ausente na importacao e na configuracao")
	}
	club.Source = "importacao_local"
	club.SyncedAt = time.Now()
	st, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer st.Close()
	// Tudo ja foi decodificado e validado em memoria. Nao existe escrita
	// parcial de linhas: o Store recebe somente o clube completo e nao vazio.
	if err := st.SaveClub(ctx, club); err != nil {
		return fmt.Errorf("gravando clube importado: %w", err)
	}
	if err := st.SaveSnapshot(ctx, store.Snapshot{GeneratedAt: club.SyncedAt, Cycle: club.Cycle, Club: club, Errors: []string{"importacao local: sem mercado, SBCs ou conta EA"}}); err != nil {
		return fmt.Errorf("gravando snapshot importado: %w", err)
	}
	fmt.Printf("importacao local gravada: %d cartas no ciclo %s\n", len(club.Players), club.Cycle)
	return nil
}

func contemSegredo(b []byte) bool {
	var v any
	if json.Unmarshal(b, &v) != nil {
		return false
	}
	var scan func(any) bool
	scan = func(x any) bool {
		switch v := x.(type) {
		case map[string]any:
			for k, val := range v {
				key := strings.ToLower(k)
				if strings.Contains(key, "password") || strings.Contains(key, "cookie") || strings.Contains(key, "token") || strings.Contains(key, "session") || strings.Contains(key, "authorization") {
					return true
				}
				if scan(val) {
					return true
				}
			}
		case []any:
			for _, item := range v {
				if scan(item) {
					return true
				}
			}
		}
		return false
	}
	return scan(v)
}
func decodeClubImport(name string, b []byte) (domain.Club, error) {
	if strings.HasSuffix(strings.ToLower(name), ".csv") {
		return decodeClubCSV(b)
	}
	var envelope struct {
		Club domain.Club `json:"club"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil {
		return domain.Club{}, fmt.Errorf("JSON de importacao invalido: %w", err)
	}
	if len(envelope.Club.Players) == 0 {
		var club domain.Club
		if err := json.Unmarshal(b, &club); err != nil {
			return domain.Club{}, fmt.Errorf("JSON de clube invalido: %w", err)
		}
		return club, nil
	}
	return envelope.Club, nil
}
func decodeClubCSV(b []byte) (domain.Club, error) {
	rows, err := csv.NewReader(strings.NewReader(string(b))).ReadAll()
	if err != nil {
		return domain.Club{}, fmt.Errorf("CSV de importacao invalido: %w", err)
	}
	if len(rows) < 2 {
		return domain.Club{}, fmt.Errorf("CSV nao tem cartas; uma importacao vazia nunca sobrescreve o clube")
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	need := func(key string) (int, error) {
		i, ok := idx[key]
		if !ok {
			return 0, fmt.Errorf("CSV precisa da coluna %q", key)
		}
		return i, nil
	}
	idCol, e := need("id")
	if e != nil {
		return domain.Club{}, e
	}
	nameCol, e := need("name")
	if e != nil {
		return domain.Club{}, e
	}
	ratingCol, e := need("rating")
	if e != nil {
		return domain.Club{}, e
	}
	posCol, e := need("position")
	if e != nil {
		return domain.Club{}, e
	}
	club := domain.Club{Source: "importacao_local"}
	for line, row := range rows[1:] {
		get := func(i int) string {
			if i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		id, err := strconv.ParseInt(get(idCol), 10, 64)
		if err != nil || id <= 0 {
			return domain.Club{}, fmt.Errorf("linha %d: id invalido", line+2)
		}
		rating, err := strconv.Atoi(get(ratingCol))
		if err != nil {
			return domain.Club{}, fmt.Errorf("linha %d: rating invalido", line+2)
		}
		pos, err := domain.ParsePosition(get(posCol))
		if err != nil {
			return domain.Club{}, fmt.Errorf("linha %d: posicao invalida", line+2)
		}
		p := domain.ClubPlayer{Player: domain.Player{ID: id, Name: get(nameCol), Rating: rating, Position: pos}}
		if i, ok := idx["club_item_id"]; ok {
			p.ClubItemID = get(i)
		}
		if i, ok := idx["untradeable"]; ok {
			p.Untradeable = strings.EqualFold(get(i), "true")
		}
		club.Players = append(club.Players, p)
	}
	return club, nil
}
func validarClubImport(c domain.Club) error {
	if len(c.Players) == 0 {
		return fmt.Errorf("a importacao nao tem cartas; o snapshot atual foi preservado")
	}
	for i, p := range c.Players {
		if p.ID <= 0 || strings.TrimSpace(p.Name) == "" || p.Rating <= 0 || p.Position == "" {
			return fmt.Errorf("carta %d esta incompleta (id, nome, nota e posicao sao obrigatorios)", i+1)
		}
	}
	return nil
}
