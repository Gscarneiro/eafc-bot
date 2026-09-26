package futgg

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// O GG Rating que o elenco do GG Club manda (ggRating/ggRatingPos) é só a
// nota da MELHOR posição da carta. A página da carta no fut.gg mostra uma
// nota por posição — Yui Hasegawa (245830): CM 85.83, CDM 85.53, CAM 81.2
// —, e é a falta dela que fazia o planejador dizer que uma CDM de ofício
// estava "sem nota" na CDM. Essa tabela vem de
// /api/fut/metarank/player/{eaId}/?game={ciclo}: uma nota por (função,
// estilo de química). A nota da posição é o MAIOR score entre as funções
// daquela posição, em qualquer estilo — é assim que a página monta a
// tabela "GG Rating", e o máximo geral é exatamente o ggRating publicado.
//
// Verificado ao vivo em 23/09/2026 contra Hasegawa, Galeno e Savinho (as
// três batem com a página, casa decimal a casa decimal) e, pela mesma
// numeração de funções do campo roleGgRatings, em 19.303 cartas do cache:
// o máximo das funções da ggRatingPos é igual ao ggRating em 100% delas.
//
// posicaoDaFuncaoMetarank traduz o id de função do metarank para a
// posição. Esta numeração NÃO é a do catálogo /api/fut/roles/ (plusEaId
// 1–49, outro espaço de id, e o metarankId de lá vem nulo). A definição
// oficial (/api/fut/metarank/{ciclo}/definition/) vem cifrada de propósito
// e o bot não a decifra. A tabela saiu dos próprios dados: em 14.921 cartas
// distintas, cada id só aparece em cartas que jogam uma mesma posição, e
// os blocos batem com a quantidade de funções por posição do catálogo
// (GK 3, CB 4, laterais 5, CDM 5, CM 5, CAM 4, meias 4, pontas 3, ST 4).
// Um ciclo novo pode renumerar: por isso conferirGGPorPosicao descarta a
// tabela da carta quando ela não reproduz o ggRating publicado.
var posicaoDaFuncaoMetarank = map[int]domain.Position{
	1: domain.GK, 2: domain.GK, 71: domain.GK,
	3: domain.CB, 4: domain.CB, 5: domain.CB, 72: domain.CB,
	6: domain.RB, 7: domain.RB, 8: domain.RB, 9: domain.RB, 74: domain.RB,
	10: domain.LB, 11: domain.LB, 12: domain.LB, 13: domain.LB, 73: domain.LB,
	14: domain.CDM, 15: domain.CDM, 16: domain.CDM, 17: domain.CDM, 75: domain.CDM,
	23: domain.CM, 24: domain.CM, 25: domain.CM, 26: domain.CM, 27: domain.CM,
	37: domain.CAM, 38: domain.CAM, 39: domain.CAM, 40: domain.CAM,
	45: domain.RM, 46: domain.RM, 47: domain.RM, 48: domain.RM,
	49: domain.LM, 50: domain.LM, 51: domain.LM, 52: domain.LM,
	53: domain.RW, 54: domain.RW, 55: domain.RW,
	56: domain.LW, 57: domain.LW, 58: domain.LW,
	63: domain.ST, 64: domain.ST, 65: domain.ST, 66: domain.ST,
}

// ggPorPosicaoDoMetarank reduz a resposta do metarank a uma nota por
// posição. Função de id desconhecido é ignorada — melhor faltar uma
// posição do que chutar em qual ela cai.
func ggPorPosicaoDoMetarank(body []byte) (map[domain.Position]float64, error) {
	var root node
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	payload := root.sub("data")
	if len(payload) == 0 {
		payload = root
	}
	notas := make(map[domain.Position]float64)
	for _, s := range payload.nodes("scores") {
		pos, ok := posicaoDaFuncaoMetarank[s.int("role")]
		if !ok {
			continue
		}
		if score := s.float64("score"); score > notas[pos] {
			notas[pos] = score
		}
	}
	return notas, nil
}

// conferirGGPorPosicao aceita a tabela só quando ela reproduz o GG que o
// elenco publicou: a nota na ggRatingPos igual ao ggRating, e nenhuma outra
// posição acima dele (o publicado é o máximo). Se a numeração das funções
// mudar num ciclo novo, a conta deixa de bater e a carta volta a ter nota
// só na melhor posição — nunca uma nota atribuída à posição errada.
func conferirGGPorPosicao(p domain.Player, notas map[domain.Position]float64) bool {
	if p.GGRating <= 0 || len(notas) == 0 {
		return false
	}
	ratingPos := p.GGRatingPos
	if ratingPos == "" {
		ratingPos = p.Position
	}
	const tolerancia = 0.051 // o elenco manda 2 casas; a página arredonda para 1
	if math.Abs(notas[ratingPos]-p.GGRating) > tolerancia {
		return false
	}
	for _, v := range notas {
		if v > p.GGRating+tolerancia {
			return false
		}
	}
	return true
}

// GGPorPosicao busca a tabela de uma carta pelo eaId. A nota é calculada
// no nível do eaId, então as cópias físicas da mesma carta compartilham.
func (c *Client) GGPorPosicao(ctx context.Context, eaID int64) (map[domain.Position]float64, error) {
	u, err := c.URL("gg_rating_posicao", map[string]string{"id": strconv.FormatInt(eaID, 10)})
	if err != nil {
		return nil, err
	}
	body, err := c.GetRaw(ctx, u)
	if err != nil {
		return nil, err
	}
	return ggPorPosicaoDoMetarank(body)
}

// ResultadoGGPorPosicao resume o preenchimento para a coleta avisar o que
// faltou em vez de deixar a lacuna silenciosa.
type ResultadoGGPorPosicao struct {
	Cartas      int   // eaIds distintos com GG publicado
	Preenchidas int   // tabelas aceitas
	Divergentes int   // tabelas descartadas pela conferência
	Falhas      int   // requisições que não voltaram
	PrimeiroErr error // um exemplo, para a mensagem ensinar o próximo passo
}

// PreencherGGPorPosicao busca a nota por posição de cada carta do clube e
// grava em todas as cópias. Uma carta que falha só fica como antes (nota na
// melhor posição); a coleta segue — ver "coleta tolera falha parcial".
// Paraleliza à vontade: GetRaw já aplica o rate limit e o teto de
// concorrência do cliente.
func (c *Client) PreencherGGPorPosicao(ctx context.Context, club *domain.Club) ResultadoGGPorPosicao {
	var res ResultadoGGPorPosicao
	if club == nil {
		return res
	}
	if _, ok := c.cfg.Endpoints["gg_rating_posicao"]; !ok {
		return res
	}
	porCarta := make(map[int64]domain.Player)
	for _, p := range club.Players {
		if p.GGRating > 0 && p.ID > 0 {
			porCarta[p.ID] = p.Player
		}
	}
	res.Cartas = len(porCarta)

	var mu sync.Mutex
	var wg sync.WaitGroup
	tabelas := make(map[int64]map[domain.Position]float64, len(porCarta))
	for id, player := range porCarta {
		wg.Add(1)
		go func(id int64, player domain.Player) {
			defer wg.Done()
			notas, err := c.GGPorPosicao(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				res.Falhas++
				if res.PrimeiroErr == nil {
					res.PrimeiroErr = fmt.Errorf("%s (%d): %w", player.Display(), id, err)
				}
			case !conferirGGPorPosicao(player, notas):
				res.Divergentes++
			default:
				tabelas[id] = notas
				res.Preenchidas++
			}
		}(id, player)
	}
	wg.Wait()

	for i := range club.Players {
		if notas, ok := tabelas[club.Players[i].ID]; ok {
			copia := make(map[domain.Position]float64, len(notas))
			for pos, v := range notas {
				copia[pos] = v
			}
			club.Players[i].GGRatingPorPosicao = copia
		}
	}
	return res
}

// Aviso devolve a frase para Snapshot.Errors, ou "" quando não há o que
// dizer. A mensagem diz o efeito (vaga fora da melhor posição fica sem
// nota) e o comando para investigar.
func (r ResultadoGGPorPosicao) Aviso(cycle string) string {
	switch {
	case r.Cartas == 0 || (r.Falhas == 0 && r.Divergentes == 0):
		return ""
	case r.Divergentes > 0:
		return fmt.Sprintf("nota por posição do fut.gg descartada em %d de %d cartas: a tabela de funções não reproduz o GG publicado — a numeração das funções pode ter mudado no ciclo %s. Essas cartas ficam com nota só na melhor posição. Confira com: eafcbot discover -url \"https://www.fut.gg/api/fut/metarank/player/<eaId>/?game=%s\"",
			r.Divergentes, r.Cartas, cycle, cycle)
	default:
		return fmt.Sprintf("nota por posição do fut.gg indisponível para %d de %d cartas (%v) — elas ficam com nota só na melhor posição. Confira com: eafcbot discover gg_rating_posicao <eaId>",
			r.Falhas, r.Cartas, r.PrimeiroErr)
	}
}
