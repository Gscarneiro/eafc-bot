package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ReferenciaCartaElenco identifica uma carta no plano sem confundir duas
// cópias físicas. PlayerID sozinho só é aceito quando a coleta provou que há
// uma única cópia com esse id; essa verificação fica na API, que conhece o
// clube atual.
type ReferenciaCartaElenco struct {
	Origem     string `json:"origem,omitempty"` // vazio/clube | mercado | evolucao
	ClubItemID string `json:"club_item_id,omitempty"`
	PlayerID   int64  `json:"player_id,omitempty"`
	EvolucaoID string `json:"evolucao_id,omitempty"`
}

func (r ReferenciaCartaElenco) Vazia() bool {
	return r.ClubItemID == "" && r.PlayerID == 0
}

func (r ReferenciaCartaElenco) chave() string {
	origem := r.Origem
	if origem == "" {
		origem = "clube"
	}
	prefixo := origem + ":"
	if r.EvolucaoID != "" {
		prefixo += r.EvolucaoID + ":"
	}
	if r.ClubItemID != "" {
		return prefixo + "item:" + r.ClubItemID
	}
	return fmt.Sprintf("%splayer:%d", prefixo, r.PlayerID)
}

// VagaPlanoElenco é uma vaga física do XI. Funcao e EstiloEntrosamento são
// decisões do plano, não dados inferidos da carta, para a mesma escalação
// poder ser revista quando o meta ou a química mudarem.
type VagaPlanoElenco struct {
	Index              int                   `json:"index"`
	Posicao            Position              `json:"posicao"`
	Carta              ReferenciaCartaElenco `json:"carta"`
	Funcao             string                `json:"funcao,omitempty"`
	EstiloEntrosamento string                `json:"estilo_entrosamento,omitempty"`
}

// PlanoElencoSalvo é um plano local, particionado por ciclo e clube. Ele não
// altera a conta EA: Referencia só escolhe qual plano o produto usa para suas
// análises e recomendações.
type PlanoElencoSalvo struct {
	ID              string                  `json:"id"`
	Ciclo           string                  `json:"ciclo"`
	Clube           string                  `json:"clube"`
	Nome            string                  `json:"nome"`
	Formacao        string                  `json:"formacao"`
	OrigemFormacao  string                  `json:"origem_formacao"`
	EstiloJogo      string                  `json:"estilo_jogo,omitempty"`
	Vagas           []VagaPlanoElenco       `json:"vagas"`
	Banco           []ReferenciaCartaElenco `json:"banco,omitempty"`
	NaoRelacionados []ReferenciaCartaElenco `json:"nao_relacionados,omitempty"`
	Revisao         int                     `json:"revisao"`
	Referencia      bool                    `json:"referencia"`
	CriadoEm        time.Time               `json:"criado_em"`
	AtualizadoEm    time.Time               `json:"atualizado_em"`
}

// Validate protege o arquivo local de um plano impossível. A presença e a
// identidade exata das cartas são verificadas depois contra a coleta atual.
func (p PlanoElencoSalvo) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Ciclo) == "" || strings.TrimSpace(p.Clube) == "" {
		return fmt.Errorf("plano salvo precisa de id, ciclo e clube")
	}
	if nome := strings.TrimSpace(p.Nome); nome == "" || len([]rune(nome)) > 80 {
		return fmt.Errorf("nome do plano precisa ter entre 1 e 80 caracteres")
	}
	if strings.TrimSpace(p.Formacao) == "" {
		return fmt.Errorf("plano salvo precisa de formação")
	}
	if p.OrigemFormacao != "confirmada" && p.OrigemFormacao != "manual" {
		return fmt.Errorf("origem da formação precisa ser confirmada ou manual")
	}
	if p.Revisao < 1 {
		return fmt.Errorf("plano salvo precisa de revisão positiva")
	}
	if len(p.Vagas) != 11 {
		return fmt.Errorf("plano salvo precisa de 11 vagas; recebeu %d", len(p.Vagas))
	}
	indices := make([]int, len(p.Vagas))
	ocupadas := make(map[string]string)
	for i, vaga := range p.Vagas {
		if vaga.Index < 0 || vaga.Posicao == "" {
			return fmt.Errorf("vaga %d precisa de índice não negativo e posição", i+1)
		}
		indices[i] = vaga.Index
		if err := ocuparReferencia(ocupadas, vaga.Carta, fmt.Sprintf("vaga %d", vaga.Index)); err != nil {
			return err
		}
	}
	sort.Ints(indices)
	for index, got := range indices {
		if got != index {
			return fmt.Errorf("as vagas precisam cobrir os índices 0 a 10")
		}
	}
	for _, carta := range p.Banco {
		if carta.Vazia() {
			return fmt.Errorf("banco não aceita uma carta vazia")
		}
		if err := ocuparReferencia(ocupadas, carta, "banco"); err != nil {
			return err
		}
	}
	for _, carta := range p.NaoRelacionados {
		if carta.Vazia() {
			return fmt.Errorf("não relacionados não aceita uma carta vazia")
		}
		if err := ocuparReferencia(ocupadas, carta, "não relacionados"); err != nil {
			return err
		}
	}
	return nil
}

func ocuparReferencia(ocupadas map[string]string, carta ReferenciaCartaElenco, local string) error {
	if carta.Vazia() {
		return nil
	}
	if carta.PlayerID < 0 {
		return fmt.Errorf("%s tem player_id inválido", local)
	}
	if carta.Origem != "" && carta.Origem != "clube" && carta.Origem != "mercado" && carta.Origem != "evolucao" {
		return fmt.Errorf("%s tem origem de carta %q desconhecida", local, carta.Origem)
	}
	if carta.Origem == "evolucao" && strings.TrimSpace(carta.EvolucaoID) == "" {
		return fmt.Errorf("%s precisa de evolucao_id para um alvo de evolução", local)
	}
	if anterior, existe := ocupadas[carta.chave()]; existe {
		return fmt.Errorf("a mesma carta aparece em %s e %s", anterior, local)
	}
	ocupadas[carta.chave()] = local
	return nil
}
