package chemistry

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// carta monta um titular comum (sem curinga) para os testes de limiar.
func carta(id int64, pos domain.Position, clube, liga, nacao string) Titular {
	return Titular{
		Index:    int(id),
		Position: pos,
		Player: domain.Player{
			ID: id, Name: "Carta", Position: pos,
			Club: clube, League: liga, Nation: nacao, Version: "Ouro Raro",
		},
	}
}

// vinculos é o modelo sem Base — é nele que os limiares aparecem, já que sob
// o modelo observado a posição sozinha enche a barra e esconde a conta.
func vinculos() Modelo { return modeloFC26Vinculos }

func TestLimiarDeLigaExigeTresTitularesParaOPrimeiroPonto(t *testing.T) {
	// Dois da mesma liga (e nada mais em comum) ainda não pontuam: liga é a
	// única categoria cujo primeiro degrau é 3, não 2.
	dois := []Titular{
		carta(1, domain.CB, "Clube A", "Premier League", "Nação A"),
		carta(2, domain.CM, "Clube B", "Premier League", "Nação B"),
	}
	if got := Calcular(vinculos(), dois).Total; got != 0 {
		t.Fatalf("dois da mesma liga deram %d, esperava 0 (o degrau de liga é 3)", got)
	}

	tres := append(dois, carta(3, domain.ST, "Clube C", "Premier League", "Nação C"))
	if got := Calcular(vinculos(), tres).Total; got != 3 {
		t.Fatalf("três da mesma liga deram %d, esperava 3 (1 ponto para cada um)", got)
	}
}

func TestLimiarDeClubeEDeNacaoComecamEmDoisTitulares(t *testing.T) {
	mesmoClube := []Titular{
		carta(1, domain.CB, "Arsenal", "Liga A", "Nação A"),
		carta(2, domain.CM, "Arsenal", "Liga B", "Nação B"),
	}
	if got := Calcular(vinculos(), mesmoClube).Total; got != 2 {
		t.Fatalf("dois do mesmo clube deram %d, esperava 2 (1 ponto cada)", got)
	}

	mesmaNacao := []Titular{
		carta(1, domain.CB, "Clube A", "Liga A", "England"),
		carta(2, domain.CM, "Clube B", "Liga B", "England"),
	}
	if got := Calcular(vinculos(), mesmaNacao).Total; got != 2 {
		t.Fatalf("dois da mesma nação deram %d, esperava 2 (1 ponto cada)", got)
	}
}

// O degrau vale pelo MAIOR limiar atingido, não pela soma dos degraus: 7 do
// mesmo clube valem 3, não 1+2+3.
func TestLimiarUsaOMaiorAtingidoNaoASoma(t *testing.T) {
	var xi []Titular
	for i := int64(1); i <= 7; i++ {
		xi = append(xi, carta(i, domain.CM, "Arsenal", "Liga X", "Nação Y"))
	}
	res := Calcular(vinculos(), xi)
	for _, j := range res.Jogadores {
		if j.Clube != 3 {
			t.Fatalf("clube com 7 titulares deu %d pontos, esperava 3", j.Clube)
		}
	}
}

func TestQuimicaDeJogadorNuncaPassaDoTetoDeTres(t *testing.T) {
	// 8 do mesmo clube, mesma liga e mesma nação: 3+3+3 = 9 antes do teto.
	var xi []Titular
	for i := int64(1); i <= 8; i++ {
		xi = append(xi, carta(i, domain.CM, "Arsenal", "Premier League", "England"))
	}
	res := Calcular(vinculos(), xi)
	for _, j := range res.Jogadores {
		if j.Pontos != 3 {
			t.Fatalf("jogador com vínculo triplo ficou com %d, o teto é 3", j.Pontos)
		}
		if j.Vinculo != 9 {
			t.Fatalf("vínculo bruto = %d, esperava 9 (3+3+3 antes do teto)", j.Vinculo)
		}
	}
}

func TestQuimicaDoTimeNuncaPassaDoTetoDeTrintaETres(t *testing.T) {
	var xi []Titular
	for i := int64(1); i <= 15; i++ {
		xi = append(xi, carta(i, domain.CM, "Arsenal", "Premier League", "England"))
	}
	if got := Calcular(vinculos(), xi).Total; got != 33 {
		t.Fatalf("total = %d, esperava o teto de 33", got)
	}
}

// A observação que motivou o modelo padrão: sob ele, um XI sem vínculo
// nenhum marca 33 desde que todos estejam em posição.
func TestBaseTresDaTrintaETresIndependenteDeVinculo(t *testing.T) {
	var xi []Titular
	for i := int64(1); i <= 11; i++ {
		xi = append(xi, carta(i, domain.CM, "Clube "+string(rune('A'+i)), "Liga "+string(rune('A'+i)), "Nação "+string(rune('A'+i))))
	}
	res := Calcular(ModeloPadrao(), xi)
	if res.Total != 33 {
		t.Fatalf("total = %d, esperava 33 (a posição sozinha enche a barra no modelo observado)", res.Total)
	}
	// E o mesmo XI, pela regra de vínculo, não marca quase nada — é
	// exatamente a diferença que a calibração mede.
	if got := Calcular(vinculos(), xi).Total; got != 0 {
		t.Fatalf("pelo modelo de vínculos o mesmo XI deu %d, esperava 0", got)
	}
}

// --- Curingas ---

func icon(id int64, nacao string) Titular {
	return Titular{Index: int(id), Position: domain.ST, Player: domain.Player{
		ID: id, Position: domain.ST, Club: "ICON", League: "Icons", Nation: nacao, Version: "FUTTIES ICON",
	}}
}

func heroi(id int64, liga, nacao string) Titular {
	return Titular{Index: int(id), Position: domain.ST, Player: domain.Player{
		ID: id, Position: domain.ST, Club: "", League: liga, Nation: nacao, Version: "FUTTIES Hero",
	}}
}

func TestIconContaDoisParaAPropriaNacaoEUmParaTodaLiga(t *testing.T) {
	// O Icon é brasileiro e vale 2 para o Brasil: sozinho com UM brasileiro
	// comum, a nação já bate o degrau de 2... e o de 5 não.
	xi := []Titular{
		icon(1, "Brazil"),
		carta(2, domain.CM, "Clube A", "Liga A", "Brazil"),
	}
	res := Calcular(vinculos(), xi)
	if res.Jogadores[1].Nacao != 1 {
		t.Fatalf("brasileiro comum ao lado de um Icon brasileiro ficou com %d de nação, esperava 1 (Icon conta 2)", res.Jogadores[1].Nacao)
	}

	// E o +1 em TODA liga: dois de uma liga qualquer + um Icon fecham o
	// degrau de 3 daquela liga, mesmo o Icon não pertencendo a ela.
	comIcon := []Titular{
		icon(1, "Brazil"),
		carta(2, domain.CM, "Clube A", "Serie A", "Nação A"),
		carta(3, domain.CB, "Clube B", "Serie A", "Nação B"),
	}
	res = Calcular(vinculos(), comIcon)
	if res.Jogadores[1].Liga != 1 {
		t.Fatalf("liga com 2 titulares + Icon deu %d, esperava 1 (o Icon vale +1 em toda liga)", res.Jogadores[1].Liga)
	}
}

func TestHeroiContaDoisParaAPropriaLigaEUmParaAPropriaNacao(t *testing.T) {
	// Herói vale 2 na PRÓPRIA liga: com mais um da mesma liga, fecha o
	// degrau de 3.
	xi := []Titular{
		heroi(1, "Premier League", "Bulgaria"),
		carta(2, domain.CM, "Clube A", "Premier League", "Nação A"),
	}
	res := Calcular(vinculos(), xi)
	if res.Jogadores[1].Liga != 1 {
		t.Fatalf("liga com 1 titular + Herói deu %d, esperava 1 (Herói conta 2 na própria liga)", res.Jogadores[1].Liga)
	}

	// Mas NÃO é curinga de liga: uma liga diferente não recebe nada dele.
	outra := []Titular{
		heroi(1, "Premier League", "Bulgaria"),
		carta(2, domain.CM, "Clube A", "Serie A", "Nação A"),
		carta(3, domain.CB, "Clube B", "Serie A", "Nação B"),
	}
	res = Calcular(vinculos(), outra)
	if res.Jogadores[1].Liga != 0 {
		t.Fatalf("Serie A com 2 titulares deu %d, esperava 0 — o Herói é de outra liga e não é curinga", res.Jogadores[1].Liga)
	}
}

func TestIconEHeroiRecebemOTetoMesmoSemVinculo(t *testing.T) {
	xi := []Titular{icon(1, "Brazil"), heroi(2, "Premier League", "Bulgaria")}
	res := Calcular(vinculos(), xi)
	for _, j := range res.Jogadores {
		if j.Pontos != 3 {
			t.Fatalf("curinga %q ficou com %d, esperava o teto 3 mesmo sem vínculo", j.Curinga, j.Pontos)
		}
	}
}

// Caso real do elenco: Vincent Kompany tem League "Icons" e Version
// "...Greats of the Game Hero" ao mesmo tempo. A ORDEM do slice Curingas é a
// regra de precedência — Icon primeiro.
func TestKompanyComLigaIconsEVersaoHeroContaComoIcon(t *testing.T) {
	kompany := Titular{Index: 0, Position: domain.CB, Player: domain.Player{
		ID: 1, Position: domain.CB, Club: "", League: "Icons", Nation: "Belgium",
		Version: "Festival of Football: Greats of the Game Hero",
	}}
	res := Calcular(vinculos(), []Titular{kompany})
	if res.Jogadores[0].Curinga != "Icon" {
		t.Fatalf("Kompany classificou como %q, esperava Icon (liga vence a versão)", res.Jogadores[0].Curinga)
	}
}

func TestIconSemClubeNaoFormaVinculoDeClube(t *testing.T) {
	// Dois Icons com o clube placeholder "ICON" não podem formar vínculo de
	// clube entre si — Icon não tem clube real.
	xi := []Titular{icon(1, "Brazil"), icon(2, "England")}
	res := Calcular(vinculos(), xi)
	for _, j := range res.Jogadores {
		if j.Clube != 0 {
			t.Fatalf("Icon ganhou %d de clube, esperava 0 (placeholder \"ICON\" não é clube)", j.Clube)
		}
	}
}

// --- Fora de posição ---

func TestTitularForaDePosicaoFicaComZero(t *testing.T) {
	fora := Titular{Index: 0, Position: domain.ST, Player: domain.Player{
		ID: 1, Position: domain.GK, Club: "Clube A", League: "Liga A", Nation: "Nação A",
	}}
	res := Calcular(ModeloPadrao(), []Titular{fora})
	if res.Jogadores[0].Pontos != 0 || !res.Jogadores[0].ForaDePosicao {
		t.Fatalf("goleiro escalado no ataque ficou com %+v, esperava 0 e fora_de_posicao", res.Jogadores[0])
	}
	if res.ForaDePos != 1 {
		t.Fatalf("Resultado.ForaDePos = %d, esperava 1", res.ForaDePos)
	}
}

// "Fora de posição, não contribuindo" — o app é literal: ele também para de
// contar para o vínculo dos OUTROS.
func TestTitularForaDePosicaoParaDeContarParaOVinculoDosOutros(t *testing.T) {
	xi := []Titular{
		carta(1, domain.CB, "Arsenal", "Liga A", "Nação A"),
		{Index: 2, Position: domain.ST, Player: domain.Player{
			ID: 2, Position: domain.GK, Club: "Arsenal", League: "Liga B", Nation: "Nação B",
		}},
	}
	res := Calcular(vinculos(), xi)
	if res.Jogadores[0].Clube != 0 {
		t.Fatalf("o titular em posição ganhou %d de clube, esperava 0 — o companheiro fora de posição não conta", res.Jogadores[0].Clube)
	}
}

// A posição avaliada é a do SLOT, não a natural da carta — e alt position
// conta como "em posição".
func TestForaDePosicaoAceitaPosicaoAlternativa(t *testing.T) {
	alt := Titular{Index: 0, Position: domain.CDM, Player: domain.Player{
		ID: 1, Position: domain.CB, AltPositions: []domain.Position{domain.CDM},
	}}
	res := Calcular(ModeloPadrao(), []Titular{alt})
	if res.Jogadores[0].ForaDePosicao {
		t.Fatal("carta com CDM entre as alt positions foi marcada como fora de posição")
	}
}

// --- Contador incremental ---

func TestContadorIncrementalDaOMesmoResultadoQueCalcular(t *testing.T) {
	xi := []Titular{
		carta(1, domain.CB, "Arsenal", "Premier League", "England"),
		carta(2, domain.CM, "Arsenal", "Premier League", "Brazil"),
		carta(3, domain.ST, "Liverpool", "Premier League", "England"),
	}
	c := NovoContador(vinculos(), xi)

	novo := domain.Player{ID: 9, Position: domain.ST, Club: "Arsenal", League: "Premier League", Nation: "England"}
	esperado := Calcular(vinculos(), []Titular{xi[0], xi[1], {Index: 3, Position: domain.ST, Player: novo}}).Total
	if got := c.TotalSe(2, novo); got != esperado {
		t.Fatalf("TotalSe = %d, Calcular do mesmo XI = %d", got, esperado)
	}

	c.Aplicar(2, novo)
	if got := c.Total(); got != esperado {
		t.Fatalf("depois de Aplicar, Total = %d, esperava %d", got, esperado)
	}
}

func TestTotalSeNaoAlteraOContador(t *testing.T) {
	xi := []Titular{
		carta(1, domain.CB, "Arsenal", "Premier League", "England"),
		carta(2, domain.CM, "Chelsea", "Premier League", "Brazil"),
	}
	c := NovoContador(vinculos(), xi)
	antes := c.Total()
	c.TotalSe(0, domain.Player{ID: 9, Position: domain.CB, Club: "Chelsea", League: "Premier League", Nation: "Brazil"})
	if depois := c.Total(); depois != antes {
		t.Fatalf("TotalSe mexeu no contador: %d -> %d", antes, depois)
	}
}

func TestEscolherModeloDesconhecidoListaOsValidos(t *testing.T) {
	_, err := Escolher("fc99_inventado")
	if err == nil {
		t.Fatal("esperava erro para modelo inexistente")
	}
	for _, nome := range []string{"fc26_observado", "fc26_vinculos"} {
		if !contains(err.Error(), nome) {
			t.Errorf("mensagem de erro não cita %q: %s", nome, err)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
