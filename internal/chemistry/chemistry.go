// Package chemistry modela como o jogo calcula o entrosamento (química) do
// XI titular.
//
// É o pacote mais volátil do repo por natureza: a EA muda essa regra entre
// ciclos e não publica a fórmula. Por isso a regra vive como TABELA DE DADOS
// (Modelo) e não como sequência de ifs — virar FC 26 -> FC 27 deve ser
// escrever outro Modelo, não reescrever este arquivo (mesmo princípio de
// "endpoints são configuração, não código", CLAUDE.md).
//
// Importa SÓ internal/domain: não conhece fut.gg, otimizador nem relatório.
package chemistry

import (
	"sort"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// Limiar é "quantos titulares precisam compartilhar o atributo para render N
// pontos". A avaliação aplica o MAIOR limiar atingido, não a soma: 8 na mesma
// liga vale 3, não 1+2+3.
type Limiar struct {
	Titulares int
	Pontos    int
}

// Curinga descreve uma carta que não segue a contagem normal. A lista de
// Curinga do Modelo é ORDENADA e o PRIMEIRO que casar vence — a ordem no
// slice É a regra de precedência, em vez de um if enterrado no meio do
// cálculo. É assim que o Vincent Kompany (League "Icons" mas Version
// "...Greats of the Game Hero", caso real no elenco) resolve como Icon.
type Curinga struct {
	Nome string // "Icon", "Hero" — só para exibir e explicar a conta

	// Detecção. Não existe campo booleano vindo do fut.gg: Icon aparece como
	// League == "Icons" e Hero só aparece em Version, porque Heroes mantêm a
	// liga REAL deles (confirmado no retrato de 24/08/2026: 24 cartas com
	// League "Icons"; os Heroes com liga de verdade e clube vazio).
	LigaIgual    string
	VersaoContem string

	SempreEmPosicao bool // Icon/Hero não ficam fora de posição
	SempreMaximo    bool // recebe MaxPorJogador independente de vínculo

	// Peso* é quanto a carta conta para o vínculo dos OUTROS.
	PesoClube int
	PesoLiga  int
	PesoNacao int
	// LigaCoringa faz PesoLiga valer para QUALQUER liga — é o Icon, que soma
	// +1 em toda liga do elenco. O Hero mantém a liga real e só reforça a
	// própria, então aqui fica falso.
	LigaCoringa bool
}

// Modelo é a regra completa de entrosamento de UM ciclo do jogo.
type Modelo struct {
	Nome  string
	Fonte string

	// Base é quanto todo titular EM POSIÇÃO ganha antes de qualquer vínculo.
	// É o parâmetro — um número, não um switch — que absorve o que o FC 26
	// de fato faz: com Base == MaxPorJogador o vínculo não acrescenta nada, a
	// química perde o gradiente, e o otimizador passa a ignorá-la sozinho,
	// sem caminho de código separado. Zero é a regra clássica, em que só o
	// vínculo pontua.
	Base int

	Clube []Limiar
	Liga  []Limiar
	Nacao []Limiar

	MaxPorJogador int
	MaxDoTime     int

	// ForaDePosicaoZera: titular fora de posição fica com 0 E para de contar
	// para o vínculo dos outros ("Fora de posição, não contribuindo", como o
	// próprio app diz).
	ForaDePosicaoZera bool

	Curingas []Curinga

	// NaoModelado é o que este Modelo SABE que não considera. Vai inteiro
	// para Resultado.NaoModelado e daí para a tela — um número que parece
	// exato precisa dizer o que ficou de fora ("na dúvida, não afirma").
	NaoModelado []string
}

// Titular é uma carta num slot físico. Existe em vez de receber domain.Club
// porque o otimizador precisa pontuar escalações HIPOTÉTICAS, que não existem
// em Club nenhum. Position é a do SLOT, não a natural da carta — mesma
// pegadinha de domain.SquadSlot.
type Titular struct {
	Index    int
	Position domain.Position
	// Player, não ClubPlayer: entrosamento não depende de untradeable,
	// contrato nem evolução aplicada.
	Player domain.Player
}

// Jogador é o entrosamento de UM titular com a conta ABERTA. Os campos
// parciais existem para a tela poder explicar ("base 3 · clube 0 · liga 0 ·
// nação 1") em vez de exibir um número que ninguém consegue conferir — é o
// mesmo detalhamento que a carta mostra dentro do jogo.
type Jogador struct {
	PlayerID int64 `json:"player_id"`
	Index    int   `json:"index"`
	// Pontos é o entrosamento EFETIVO, já com Base e já limitado ao teto.
	Pontos int `json:"pontos"`
	Base   int `json:"base"`
	Clube  int `json:"clube"`
	Liga   int `json:"liga"`
	Nacao  int `json:"nacao"`
	// Vinculo é clube+liga+nação sem a Base — é o número que aparece na
	// carta dentro do jogo (o Butland mostra 1, por ser inglês, mesmo com
	// efetivo 3). Sem expor isso, o número do bot não bate com o que o
	// usuário vê e parece bug.
	Vinculo       int    `json:"vinculo"`
	ForaDePosicao bool   `json:"fora_de_posicao"`
	Curinga       string `json:"curinga,omitempty"`
}

// Resultado é o entrosamento de um XI inteiro.
type Resultado struct {
	Total       int         `json:"total"`
	Maximo      int         `json:"maximo"`
	Modelo      string      `json:"modelo"`
	Jogadores   []Jogador   `json:"jogadores"`
	ForaDePos   int         `json:"fora_de_posicao"`
	NaoModelado []string    `json:"nao_modelado,omitempty"`
	Verificacao Verificacao `json:"verificacao"`
}

// Calcular é o ÚNICO lugar onde a regra é aplicada. Nenhuma tela, relatório
// ou otimizador recalcula entrosamento por conta própria.
func Calcular(m Modelo, xi []Titular) Resultado {
	return NovoContador(m, xi).Resultado()
}

// Avaliar é o ponto de entrada de conveniência para quem só tem um
// domain.Club: monta o XI (DoClube), calcula (Calcular) e já confronta com o
// que o jogo reportou (Verificar) — para nenhum chamador (cmd, api, report)
// precisar montar Titular na mão. nil quando não há escalação sincronizada;
// é o mesmo sentinela que store.Snapshot.Quimica usa para snapshot antigo.
func Avaliar(m Modelo, club domain.Club) *Resultado {
	xi, ok := DoClube(club)
	if !ok {
		return nil
	}
	res := Calcular(m, xi)
	res.Verificacao = Verificar(m, club)
	return &res
}

// DoClube monta os Titular do XI ativo a partir do retrato do clube. Usa
// Squad.Starters (o slot físico) e devolve false quando não há escalação
// sincronizada — sem XI não existe entrosamento para calcular.
func DoClube(club domain.Club) ([]Titular, bool) {
	if len(club.Squad.Starters) == 0 {
		return nil, false
	}
	xi := make([]Titular, 0, len(club.Squad.Starters))
	for _, s := range club.Squad.Starters {
		p, ok := club.PlayerByID(s.PlayerID)
		if !ok {
			continue
		}
		xi = append(xi, Titular{Index: s.Index, Position: s.Position, Player: p.Player})
	}
	if len(xi) == 0 {
		return nil, false
	}
	sort.Slice(xi, func(i, j int) bool { return xi[i].Index < xi[j].Index })
	return xi, true
}

// classificar acha o Curinga que descreve a carta, ou nil para carta comum.
// O primeiro que casar vence — ver o comentário de Curinga sobre precedência.
func (m Modelo) classificar(p domain.Player) *Curinga {
	for i := range m.Curingas {
		c := &m.Curingas[i]
		if c.LigaIgual != "" && strings.EqualFold(p.League, c.LigaIgual) {
			return c
		}
		if c.VersaoContem != "" && strings.Contains(strings.ToLower(p.Version), strings.ToLower(c.VersaoContem)) {
			return c
		}
	}
	return nil
}

// emPosicao aplica a regra "Posição certa, contribuindo" do próprio jogo.
// Usa PlaysAt contra a posição do SLOT — NÃO o campo ClubPlayer.OutOfPos, que
// vem de outro campo do fut.gg (squadPosition) e pode discordar do slot ativo.
func (m Modelo) emPosicao(t Titular, c *Curinga) bool {
	if c != nil && c.SempreEmPosicao {
		return true
	}
	if t.Position == "" {
		return true // sem slot conhecido não dá pra afirmar que está fora
	}
	return t.Player.PlaysAt(t.Position)
}

// pontosPara aplica o MAIOR limiar atingido pela contagem.
func pontosPara(limiares []Limiar, contagem int) int {
	melhor := 0
	for _, l := range limiares {
		if contagem >= l.Titulares && l.Pontos > melhor {
			melhor = l.Pontos
		}
	}
	return melhor
}

// Contador é Calcular em forma incremental: mantém as contagens por
// clube/liga/nação e responde "e se eu trocar o titular do slot j?" sem
// refazer a soma do zero. É o que torna a busca local do otimizador barata —
// e Calcular é implementado em cima dele, para existir UMA implementação da
// regra.
type Contador struct {
	m  Modelo
	xi []Titular

	clubes  map[string]int
	ligas   map[string]int
	nacoes  map[string]int
	coringa int // quantos Icons somam +PesoLiga em TODA liga
}

// NovoContador indexa o XI. Alterar o slice devolvido por fora invalida as
// contagens — use Aplicar.
func NovoContador(m Modelo, xi []Titular) *Contador {
	c := &Contador{
		m:      m,
		xi:     append([]Titular(nil), xi...),
		clubes: map[string]int{}, ligas: map[string]int{}, nacoes: map[string]int{},
	}
	for _, t := range c.xi {
		c.somar(t, +1)
	}
	return c
}

// somar acrescenta (ou remove, com sinal -1) a contribuição de um titular às
// contagens de vínculo. Titular fora de posição não contribui com nada.
func (c *Contador) somar(t Titular, sinal int) {
	cur := c.m.classificar(t.Player)
	if c.m.ForaDePosicaoZera && !c.m.emPosicao(t, cur) {
		return
	}
	pesoClube, pesoLiga, pesoNacao := 1, 1, 1
	if cur != nil {
		pesoClube, pesoLiga, pesoNacao = cur.PesoClube, cur.PesoLiga, cur.PesoNacao
		if cur.LigaCoringa {
			c.coringa += sinal * pesoLiga
			pesoLiga = 0
		}
	}
	if t.Player.Club != "" && pesoClube != 0 {
		c.clubes[t.Player.Club] += sinal * pesoClube
	}
	if t.Player.League != "" && pesoLiga != 0 {
		c.ligas[t.Player.League] += sinal * pesoLiga
	}
	if t.Player.Nation != "" && pesoNacao != 0 {
		c.nacoes[t.Player.Nation] += sinal * pesoNacao
	}
}

// avaliar calcula o entrosamento de um titular contra as contagens atuais.
// `proprio` desconta a contribuição da própria carta? Não: no FUT o jogador
// conta a si mesmo para o próprio vínculo (2 do mesmo clube = ambos ganham).
func (c *Contador) avaliar(t Titular) Jogador {
	cur := c.m.classificar(t.Player)
	j := Jogador{PlayerID: t.Player.ID, Index: t.Index, Base: c.m.Base}
	if cur != nil {
		j.Curinga = cur.Nome
	}

	if c.m.ForaDePosicaoZera && !c.m.emPosicao(t, cur) {
		j.ForaDePosicao = true
		j.Base = 0
		return j
	}

	j.Clube = pontosPara(c.m.Clube, c.clubes[t.Player.Club])
	j.Nacao = pontosPara(c.m.Nacao, c.nacoes[t.Player.Nation])
	j.Liga = pontosPara(c.m.Liga, c.ligas[t.Player.League]+c.coringa)
	j.Vinculo = j.Clube + j.Liga + j.Nacao

	total := c.m.Base + j.Vinculo
	if cur != nil && cur.SempreMaximo {
		total = c.m.MaxPorJogador
	}
	if total > c.m.MaxPorJogador {
		total = c.m.MaxPorJogador
	}
	j.Pontos = total
	return j
}

// Total é o entrosamento do time inteiro, já limitado ao teto do modelo.
func (c *Contador) Total() int {
	total := 0
	for _, t := range c.xi {
		total += c.avaliar(t).Pontos
	}
	if c.m.MaxDoTime > 0 && total > c.m.MaxDoTime {
		return c.m.MaxDoTime
	}
	return total
}

// TotalSe responde "quanto o time marcaria se o slot `index` fosse `novo`?"
// sem alterar o contador — é consulta pura, para a busca local avaliar
// vizinhança sem desfazer estado.
func (c *Contador) TotalSe(index int, novo domain.Player) int {
	if index < 0 || index >= len(c.xi) {
		return c.Total()
	}
	antigo := c.xi[index]
	trocado := Titular{Index: antigo.Index, Position: antigo.Position, Player: novo}
	c.somar(antigo, -1)
	c.somar(trocado, +1)
	c.xi[index] = trocado

	total := c.Total()

	c.somar(trocado, -1)
	c.somar(antigo, +1)
	c.xi[index] = antigo
	return total
}

// Aplicar troca o titular do slot de vez.
func (c *Contador) Aplicar(index int, novo domain.Player) {
	if index < 0 || index >= len(c.xi) {
		return
	}
	antigo := c.xi[index]
	trocado := Titular{Index: antigo.Index, Position: antigo.Position, Player: novo}
	c.somar(antigo, -1)
	c.somar(trocado, +1)
	c.xi[index] = trocado
}

// Resultado monta a visão completa, com a conta aberta de cada titular.
func (c *Contador) Resultado() Resultado {
	r := Resultado{
		Maximo:      c.m.MaxDoTime,
		Modelo:      c.m.Nome,
		Jogadores:   make([]Jogador, 0, len(c.xi)),
		NaoModelado: c.m.NaoModelado,
	}
	for _, t := range c.xi {
		j := c.avaliar(t)
		if j.ForaDePosicao {
			r.ForaDePos++
		}
		r.Total += j.Pontos
		r.Jogadores = append(r.Jogadores, j)
	}
	if c.m.MaxDoTime > 0 && r.Total > c.m.MaxDoTime {
		r.Total = c.m.MaxDoTime
	}
	return r
}
