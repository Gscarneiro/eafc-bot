package futgg

import (
	"context"

	"github.com/gscarneiro/eafc-bot/internal/discover"
)

// checkRobots conta quantos dos endpoints JÁ CONFIGURADOS (não a sondagem de
// candidatos novos que autoconfig faz) o robots.txt do site pede para não
// tocar, e grava o total em Stats.RobotsBypassed. Rodado a cada Collect,
// independente de RespectRobots: aquela flag só controla a DESCOBERTA de
// rotas novas (ver ShouldRespectRobots e cmd/eafcbot/autoconfig.go); os
// endpoints de produção, uma vez aprendidos, sempre são lidos — o objetivo
// aqui não é bloquear isso, é nunca deixar a leitura calada sobre o que está
// ignorando (ver internal/discover/robots.go para a política completa).
//
// Falha ao buscar o robots.txt (rede fora, 404, o que for) não é erro de
// coleta: fica sem o aviso, silenciosamente — o mesmo princípio de
// Collect() para qualquer fonte acessória.
func (c *Client) checkRobots(ctx context.Context) {
	robots := discover.FetchRobots(ctx, c, c.cfg.BaseURL)
	if !robots.Loaded {
		return
	}
	var blocked int
	for _, path := range c.cfg.Endpoints {
		if !robots.Allowed(path) {
			blocked++
		}
	}
	if blocked == 0 {
		return
	}
	c.mu.Lock()
	c.stats.RobotsBypassed = blocked
	c.mu.Unlock()
}
