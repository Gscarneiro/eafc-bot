package futgg

import (
	"context"
	"strconv"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// EvolutionPaths busca os caminhos de evolução de um JOGADOR — basePlayerEaID
// é compartilhado por todas as versões dele (ouro, TOTW, FUTTIES...) — e
// devolve só os que começam na carta indicada por cardEaID.
//
// O filtro por carta é feito AQUI, no cliente: a URL da página do fut.gg tem
// um parâmetro player_item_ea_ids que sugere filtro no servidor, mas ele é
// ignorado — testei ao vivo, a resposta vem do mesmo tamanho com ou sem o
// parâmetro. Quem filtra é o JavaScript da página, e é isso que replicamos.
//
// Cuidado que decide a correção do filtro: dentro de "path", a chave "id" é
// o id INTERNO da carta (ex. 136512), não o eaId (ex. 50537761) que o resto
// do bot usa como identidade — mapPlayer tentaria "id" primeiro e devolveria
// o número errado. Por isso este método lê "eaId" direto do nó, sem passar
// pelo alias genérico, tanto para o filtro quanto para corrigir o campo ID
// de cada passo depois de montado.
func (c *Client) EvolutionPaths(ctx context.Context, basePlayerEaID, cardEaID int64) ([]domain.EvolutionPath, error) {
	// Sem isto, um caminho de evolução que ganha PlayStyle chega com o eaId
	// numérico cru em vez do nome — mesmo problema que Club() já resolve
	// chamando isto antes de montar o elenco.
	c.ensurePlayStyles(ctx)

	u, err := c.URL("evolution_paths", map[string]string{"id": strconv.FormatInt(basePlayerEaID, 10)})
	if err != nil {
		return nil, err
	}
	body, err := c.GetRaw(ctx, u)
	if err != nil {
		return nil, err
	}
	nodes, err := c.decodeList(body, "evolution_paths")
	if err != nil {
		return nil, err
	}

	l := c.lensFor("players")
	var out []domain.EvolutionPath
	for _, n := range nodes {
		steps := n.nodes("path")
		if len(steps) == 0 {
			continue
		}
		if steps[0].i64("eaId") != cardEaID {
			continue // caminho de outra versão do mesmo jogador
		}

		p := domain.EvolutionPath{
			CoinsCost:    n.int("coinsCost", "coins_cost"),
			PointsCost:   n.int("pointsCost", "points_cost"),
			IsExpired:    n.bool_("isExpired", "is_expired"),
			TrainingTime: n.str("readableTrainingTime", "readable_training_time"),
		}
		for _, s := range steps {
			step := mapPlayer(s, c.cfg.Cycle, l)
			step.ID = s.i64("eaId") // ver comentário acima: "id" não serve aqui
			p.Steps = append(p.Steps, step)
		}
		for _, e := range n.nodes("evolutions") {
			if name := e.str("name"); name != "" {
				p.Chain = append(p.Chain, name)
			}
		}
		out = append(out, p)
	}
	return out, nil
}
