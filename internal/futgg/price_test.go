package futgg

import "testing"

// Antes desta correção, Coins==0 virava Extinct=true automaticamente —
// misturando "sem oferta de verdade" (sinal real do site) com "não
// conseguimos ler o preço" (falha de raspagem, sem chave nenhuma de
// extinção na resposta). Scan de prices_26.json em 22/08/2026 achou 22,5%
// dos pontos gravados nesse estado contaminado.
func TestExtinctNaoEInferidoDePrecoZeroSemChaveDaFonte(t *testing.T) {
	p := mapPrice(node{"price": 0.0}, lens{})
	if p.Extinct {
		t.Error("sem a chave isExtinct/extinct na resposta, preço zero não deveria virar Extinct=true")
	}
	if p.Coins != 0 {
		t.Errorf("Coins = %d, esperava 0", p.Coins)
	}
}

// Quando a FONTE manda a chave, o valor dela é respeitado — inclusive
// quando price>0 (carta chegando perto do teto de preço é o próprio sinal
// de alta que a pesquisa de mercado descreveu, não uma contradição).
func TestExtinctConfiaNaChaveDaFonteQuandoPresente(t *testing.T) {
	p := mapPrice(node{"price": 0.0, "isExtinct": true}, lens{})
	if !p.Extinct {
		t.Error("com isExtinct:true na resposta, deveria respeitar")
	}

	p2 := mapPrice(node{"price": 500000.0, "isExtinct": false}, lens{})
	if p2.Extinct {
		t.Error("com isExtinct:false explícito, não deveria marcar como extinto mesmo com preço alto")
	}
}

// SBC continua sem preço (não é comprável no mercado) e isso não tem
// relação nenhuma com Extinct — os dois viraram conceitos separados.
func TestPrecoDeSBCContinuaSemMarcarExtinctAutomaticamente(t *testing.T) {
	p := mapPrice(node{"isSbc": true}, lens{})
	if !p.IsSBC {
		t.Fatal("IsSBC deveria ser true")
	}
	if p.Extinct {
		t.Error("carta de SBC sem preço não deveria virar Extinct=true sozinha")
	}
}
