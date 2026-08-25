package chemistry

import (
	"fmt"
	"sort"
	"strings"
)

// limiaresClube/Liga/Nacao são a tabela CONFIRMADA na tela "Mais
// entrosamento" do próprio app (prints de 24/08/2026), e batem com o que
// duas fontes independentes documentam. A tela mostra os degraus de forma
// incremental ("quantos A MAIS para o próximo nível"); aqui ficam em total
// acumulado, que é como a contagem de fato funciona.
var (
	limiaresClube = []Limiar{{2, 1}, {4, 2}, {7, 3}}
	limiaresLiga  = []Limiar{{3, 1}, {5, 2}, {8, 3}}
	limiaresNacao = []Limiar{{2, 1}, {5, 2}, {8, 3}}
)

// curingas é a mesma lista nos dois modelos. A ORDEM importa: Icon primeiro,
// porque existe carta com League "Icons" e Version "...Hero" ao mesmo tempo
// (Vincent Kompany, caso real no elenco) — e nesse conflito quem manda é a
// liga, que é o que o jogo usa para colocar a carta na liga "Icons".
func curingas() []Curinga {
	return []Curinga{
		{
			Nome: "Icon", LigaIgual: "Icons",
			SempreEmPosicao: true, SempreMaximo: true,
			// Icon conta 2 para a PRÓPRIA nação e 1 para TODA liga do elenco.
			// Clube fica em zero: Icon não tem clube real (vem "" ou "ICON").
			PesoClube: 0, PesoLiga: 1, PesoNacao: 2, LigaCoringa: true,
		},
		{
			Nome: "Hero", VersaoContem: "hero",
			SempreEmPosicao: true, SempreMaximo: true,
			// Herói conta 2 para a PRÓPRIA liga (que é real, diferente do
			// Icon) e 1 para a própria nação. Também não tem clube.
			PesoClube: 0, PesoLiga: 2, PesoNacao: 1,
		},
	}
}

// naoModelado é o que nenhum dos modelos considera hoje. Vai para a tela em
// vez de ficar como conhecimento tribal — o número precisa dizer o que ficou
// de fora ("na dúvida, não afirma").
var naoModelado = []string{
	"técnico (+1 a quem compartilha a nação ou a liga dele) — o bot não coleta técnico do fut.gg",
}

// modeloFC26Observado é o modelo PADRÃO. Base 3 não é regra publicada em
// lugar nenhum: é o que reproduz o que o jogo de fato reporta.
//
// A evidência, verificada e confirmada pelo usuário: no XI de 24/08/2026 os
// 11 titulares tinham 11 clubes, 8 ligas e 9 nações DISTINTAS — pela tabela
// de limiares daria ~7 — e o app mostrava 33/33, com cada titular em 3 e
// todos em posição. É o MESMO time que a coleta das 16:02 pegou. Na carta do
// Butland aparece 1 ponto de vínculo (England x2, o degrau +1 da tabela) sob
// um efetivo 3. Leitura que os dados sustentam: estar na posição certa é o
// que preenche o entrosamento, e o vínculo não soma por cima de uma barra
// cheia.
//
// Não sabemos POR QUÊ (mudança do FC 26? efeito de promo — os 11 titulares
// são todos FUTTIES/FUT Birthday? modo específico?). Não afirmamos o motivo;
// afirmamos só o que o oráculo mostra, e a Verificacao acompanha cada
// resultado dizendo se ainda bate.
//
// Os limiares vão preenchidos de propósito: se Base cair para 0 num ciclo
// futuro, eles já são a regra certa, sem pesquisa nova.
var modeloFC26Observado = Modelo{
	Nome:              "fc26_observado",
	Fonte:             "calibrado contra o próprio jogo em 24/08/2026 (XI 33/33 sem vínculo; limiares conferidos na tela \"Mais entrosamento\")",
	Base:              3,
	Clube:             limiaresClube,
	Liga:              limiaresLiga,
	Nacao:             limiaresNacao,
	MaxPorJogador:     3,
	MaxDoTime:         33,
	ForaDePosicaoZera: true,
	Curingas:          curingas(),
	NaoModelado:       naoModelado,
}

// modeloFC26Vinculos é a regra clássica: só o vínculo pontua (Base 0), com os
// mesmos limiares confirmados na tela do jogo. NÃO é o padrão — ela não
// descreve o que este jogo reporta hoje. Existe para a diferença ser
// MENSURÁVEL em vez de virar folclore, e é o modelo que volta a valer se a
// posição deixar de encher a barra.
var modeloFC26Vinculos = Modelo{
	Nome:              "fc26_vinculos",
	Fonte:             "tela \"Mais entrosamento\" do app + duas fontes públicas independentes",
	Base:              0,
	Clube:             limiaresClube,
	Liga:              limiaresLiga,
	Nacao:             limiaresNacao,
	MaxPorJogador:     3,
	MaxDoTime:         33,
	ForaDePosicaoZera: true,
	Curingas:          curingas(),
	NaoModelado:       naoModelado,
}

// Modelos devolve o registro completo, em ordem estável.
func Modelos() []Modelo {
	return []Modelo{modeloFC26Observado, modeloFC26Vinculos}
}

// ModeloPadrao é o que o bot usa quando o config não diz outra coisa.
func ModeloPadrao() Modelo { return modeloFC26Observado }

// Escolher acha o modelo pelo nome. O erro LISTA os nomes válidos — quem
// errou o nome precisa saber quais existem sem abrir o código (convenção de
// mensagem de erro do repo).
func Escolher(nome string) (Modelo, error) {
	if strings.TrimSpace(nome) == "" {
		return ModeloPadrao(), nil
	}
	for _, m := range Modelos() {
		if strings.EqualFold(m.Nome, nome) {
			return m, nil
		}
	}
	nomes := make([]string, 0, len(Modelos()))
	for _, m := range Modelos() {
		nomes = append(nomes, m.Nome)
	}
	sort.Strings(nomes)
	return Modelo{}, fmt.Errorf("modelo de química %q não existe — use um destes: %s",
		nome, strings.Join(nomes, ", "))
}
