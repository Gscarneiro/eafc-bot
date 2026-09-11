// Package ratingsource importa notas externas exportadas de forma rastreável.
// Ele não raspa FUTBIN nem FUTWIZ: endpoints, permissões e formato precisam
// ser confirmados antes de uma coleta de rede entrar no produto.
package ratingsource

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type Documento struct {
	Fonte        domain.FonteAvaliacao `json:"fonte"`
	Metrica      string                `json:"metrica"`
	EscalaMinima float64               `json:"escala_minima"`
	EscalaMaxima float64               `json:"escala_maxima"`
	Ciclo        string                `json:"ciclo"`
	Patch        string                `json:"patch,omitempty"`
	Plataforma   string                `json:"plataforma,omitempty"`
	Evidencia    string                `json:"evidencia"`
	CapturadaEm  string                `json:"capturada_em"`
	Cartas       []Carta               `json:"cartas"`
}

// Carta usa o id da versão como chave obrigatória. BasePlayerEAID e Versao
// são redundâncias de segurança quando a fonte as publica: divergência não
// é reconciliada por nome, pois nomes iguais não provam a mesma carta.
type Carta struct {
	ID             int64                       `json:"id"`
	BasePlayerEAID int64                       `json:"base_player_ea_id,omitempty"`
	Versao         string                      `json:"versao,omitempty"`
	PorPosicao     map[domain.Position]float64 `json:"por_posicao"`
}

type Resultado struct {
	Aplicadas int     `json:"aplicadas"`
	Ausentes  []int64 `json:"ausentes,omitempty"`
}

func Ler(path string) (Documento, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Documento{}, fmt.Errorf("lendo importação de notas: %w", err)
	}
	var doc Documento
	if err := json.Unmarshal(contents, &doc); err != nil {
		return Documento{}, fmt.Errorf("decodificando importação de notas: %w", err)
	}
	if err := doc.Validar(); err != nil {
		return Documento{}, err
	}
	return doc, nil
}

func (d Documento) Validar() error {
	if d.Fonte != domain.FonteFUTBIN && d.Fonte != domain.FonteFUTWIZ {
		return fmt.Errorf("fonte %q não é um adaptador externo aceito", d.Fonte)
	}
	if strings.TrimSpace(d.Metrica) == "" || strings.TrimSpace(d.Ciclo) == "" || strings.TrimSpace(d.Evidencia) == "" ||
		d.EscalaMaxima <= d.EscalaMinima || len(d.Cartas) == 0 {
		return fmt.Errorf("fonte, métrica, escala, ciclo, evidência e cartas são obrigatórios")
	}
	seen := make(map[int64]bool, len(d.Cartas))
	for _, card := range d.Cartas {
		if card.ID == 0 || seen[card.ID] || len(card.PorPosicao) == 0 {
			return fmt.Errorf("cada carta precisa de id único e nota por posição")
		}
		seen[card.ID] = true
		for pos, note := range card.PorPosicao {
			if _, err := domain.ParsePosition(string(pos)); err != nil || note < d.EscalaMinima || note > d.EscalaMaxima {
				return fmt.Errorf("nota inválida da carta %d em %s", card.ID, pos)
			}
		}
	}
	return nil
}

// Aplicar associa uma importação somente a cartas cuja identidade foi
// confirmada. IDs não encontrados são devolvidos para diagnóstico, sem
// apagar outras fontes nem interromper a coleta parcial.
func Aplicar(doc Documento, club *domain.Club, market *[]domain.Player) (Resultado, error) {
	if err := doc.Validar(); err != nil {
		return Resultado{}, err
	}
	byID := make(map[int64]Carta, len(doc.Cartas))
	for _, card := range doc.Cartas {
		byID[card.ID] = card
	}
	matched := make(map[int64]bool, len(byID))
	apply := func(player *domain.Player) error {
		card, found := byID[player.ID]
		if !found {
			return nil
		}
		if card.BasePlayerEAID != 0 && player.BasePlayerEaID != card.BasePlayerEAID {
			return fmt.Errorf("carta %d tem base_player_ea_id divergente", card.ID)
		}
		if card.Versao != "" && !strings.EqualFold(card.Versao, player.Version) {
			return fmt.Errorf("carta %d tem versão divergente", card.ID)
		}
		if player.Cycle != "" && player.Cycle != doc.Ciclo {
			return fmt.Errorf("carta %d pertence ao ciclo %s, mas a importação é do ciclo %s", card.ID, player.Cycle, doc.Ciclo)
		}
		if player.ExternalRatings == nil {
			player.ExternalRatings = make(map[domain.FonteAvaliacao]domain.NotaExterna)
		}
		positions := make(map[domain.Position]float64, len(card.PorPosicao))
		for pos, note := range card.PorPosicao {
			positions[pos] = note
		}
		player.ExternalRatings[doc.Fonte] = domain.NotaExterna{Fonte: doc.Fonte, Metrica: doc.Metrica,
			EscalaMinima: doc.EscalaMinima, EscalaMaxima: doc.EscalaMaxima, Ciclo: doc.Ciclo, Patch: doc.Patch,
			Plataforma: doc.Plataforma, Evidencia: doc.Evidencia, CapturadaEm: doc.CapturadaEm, PorPosicao: positions}
		matched[card.ID] = true
		return nil
	}
	for i := range club.Players {
		if err := apply(&club.Players[i].Player); err != nil {
			return Resultado{}, err
		}
	}
	if market != nil {
		for i := range *market {
			if err := apply(&(*market)[i]); err != nil {
				return Resultado{}, err
			}
		}
	}
	result := Resultado{Aplicadas: len(matched)}
	for id := range byID {
		if !matched[id] {
			result.Ausentes = append(result.Ausentes, id)
		}
	}
	sort.Slice(result.Ausentes, func(i, j int) bool { return result.Ausentes[i] < result.Ausentes[j] })
	return result, nil
}
