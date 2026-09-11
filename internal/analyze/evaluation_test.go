package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestAvaliadorCarregaPerfisVersionados(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(r.Perfis()); got != 4 {
		t.Fatalf("perfis = %d, esperava 4", got)
	}
}

func TestAvaliadorNaoInventaSubatributosAusentes(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	got := r.Avaliar(domain.Player{Cycle: "27", Position: domain.CB}, domain.CB, domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: "meta_competitivo"})
	if got.Disponivel || got.Motivo == "" {
		t.Fatalf("avaliação sem subatributos = %+v; deveria ser indisponível", got)
	}
}

func TestAvaliadorUsaVersaoDoPerfilENaoDaCarta(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	p := zagueiroDetalhado()
	got := r.Avaliar(p, domain.CB, domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: "meta_competitivo", Plataforma: "ps5", Patch: "TU-1"})
	if !got.Disponivel {
		t.Fatalf("avaliação indisponível: %+v", got)
	}
	if got.Contexto.VersaoPerfil != "2026.09.2" || got.Contexto.VersaoPerfil == p.Version {
		t.Fatalf("versão = %q; deve ser a versão do perfil", got.Contexto.VersaoPerfil)
	}
	if got.Nota < 0 || got.Nota > 100 {
		t.Fatalf("nota fora da escala: %v", got.Nota)
	}
}

func TestAvaliadorFutGGNaoCaiParaNotaDoBot(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	got := r.Avaliar(zagueiroDetalhado(), domain.CB, domain.ContextoAvaliacao{Fonte: domain.FonteFutGG})
	if got.Disponivel {
		t.Fatalf("FUT.GG sem nota deveria ficar indisponível, veio %+v", got)
	}
}

func TestAvaliadorFUTBINDaNotaPosicionalImportadaSemMisturarEscalas(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	p := domain.Player{Cycle: "27", Position: domain.CM, ExternalRatings: map[domain.FonteAvaliacao]domain.NotaExterna{
		domain.FonteFUTBIN: {Fonte: domain.FonteFUTBIN, Metrica: "rating_per_position", EscalaMinima: 0, EscalaMaxima: 100, Ciclo: "27", Evidencia: "https://exemplo.test", PorPosicao: map[domain.Position]float64{domain.CM: 94.2}},
	}}
	got := r.Avaliar(p, domain.CM, domain.ContextoAvaliacao{Fonte: domain.FonteFUTBIN, Ciclo: "27"})
	if !got.Disponivel || got.Nota != 94.2 || got.Contexto.Fonte != domain.FonteFUTBIN {
		t.Fatalf("avaliação FUTBIN = %+v", got)
	}
	if unavailable := r.Avaliar(p, domain.CM, domain.ContextoAvaliacao{Fonte: domain.FonteFUTWIZ, Ciclo: "27"}); unavailable.Disponivel {
		t.Fatalf("fonte ausente não pode herdar FUTBIN: %+v", unavailable)
	}
}

func TestAvaliadorExplicaPorteEFamiliaridadeDaFuncao(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	p := zagueiroDetalhado()
	p.BodyType, p.Foot = "Stocky", "Direito"
	p.FamiliaridadesFuncao = []domain.FamiliaridadeFuncao{{Nome: "Defender", Posicao: domain.CB, Nivel: "plus_plus"}}
	chem := 3
	got := r.Avaliar(p, domain.CB, domain.ContextoAvaliacao{
		Fonte: domain.FonteBot, Perfil: "meta_competitivo", Plataforma: "ps5", Patch: "TU-1",
		Funcao: "defender", Quimica: &chem,
	})
	if !got.Disponivel {
		t.Fatalf("avaliação indisponível: %+v", got)
	}
	components := make(map[string]float64)
	for _, component := range got.Componentes {
		components[component.Chave] = component.Valor
	}
	for _, key := range []string{"porte_fisico", "acelerate_rate", "familiaridade_funcao"} {
		if _, ok := components[key]; !ok {
			t.Fatalf("componente %q ausente em %+v", key, got.Componentes)
		}
	}
	if components["familiaridade_funcao"] != 0.9 {
		t.Fatalf("bônus de função = %v, esperava 0.9", components["familiaridade_funcao"])
	}
}

func TestAvaliadorMarcaEstiloSemRegraVerificadaComoLimitacao(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	p := zagueiroDetalhado()
	p.BodyType = "Average"
	chem := 3
	got := r.Avaliar(p, domain.CB, domain.ContextoAvaliacao{
		Fonte: domain.FonteBot, Perfil: "meta_competitivo", Plataforma: "ps5", Patch: "TU-1",
		Quimica: &chem, EstiloEntrosamento: "Sombra",
	})
	if !got.Parcial {
		t.Fatalf("estilo sem regra deveria manter avaliação parcial: %+v", got)
	}
	found := false
	for _, limitation := range got.Limitacoes {
		if limitation == "estilo de entrosamento Sombra sem regra verificada" {
			found = true
		}
	}
	if !found {
		t.Fatalf("limitação de estilo ausente: %+v", got.Limitacoes)
	}
}

func TestAvaliadorAplicaSomenteIncrementosDeEstiloConfirmadosParaOCiclo(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	r.estilos = &RegistroEstilosEntrosamento{porCiclo: map[string]estiloEntrosamentoArquivo{
		"27": {
			Ciclo: "27", Versao: "teste", Status: "confirmado", Fonte: "teste", VerificadoEm: "2026-09-10",
			Estilos: map[string]map[string]map[string]int{
				"sombra": {"3": {"acceleration": 9, "sprint_speed": 9}},
			},
		},
	}}
	p := zagueiroDetalhado()
	p.BodyType = "Average"
	chem := 3
	base := r.Avaliar(p, domain.CB, domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: "meta_competitivo", Plataforma: "ps5", Patch: "TU-1", Quimica: &chem})
	styled := r.Avaliar(p, domain.CB, domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: "meta_competitivo", Plataforma: "ps5", Patch: "TU-1", Quimica: &chem, EstiloEntrosamento: "Sombra"})
	if !styled.Disponivel || styled.Nota <= base.Nota {
		t.Fatalf("estilo confirmado deveria elevar a nota: base=%.1f estilo=%+v", base.Nota, styled)
	}
	for _, limitation := range styled.Limitacoes {
		if limitation == "estilo de entrosamento Sombra sem regra verificada" {
			t.Fatalf("regra confirmada ainda foi marcada como ausente: %+v", styled.Limitacoes)
		}
	}
	for _, component := range styled.Componentes {
		if component.Chave == "estilo_entrosamento" && component.Valor > 0 {
			return
		}
	}
	t.Fatalf("componente explicável do estilo ausente: %+v", styled.Componentes)
}

func TestClubeNaReguaNaoMisturaFonteQuandoBotNaoTemCobertura(t *testing.T) {
	r, err := NovoRegistroAvaliadores()
	if err != nil {
		t.Fatal(err)
	}
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1, Position: domain.CB, GGRating: 94}}}}
	got := ClubeNaRegua(club, r, domain.ContextoAvaliacao{Fonte: domain.FonteBot, Perfil: "meta_competitivo"})
	if got.Players[0].GGRating != 0 {
		t.Fatalf("nota externa vazou para perfil sem cobertura: %v", got.Players[0].GGRating)
	}
}

func zagueiroDetalhado() domain.Player {
	v := func(n int) *int { return &n }
	return domain.Player{Cycle: "27", Version: "TOTW", Position: domain.CB, WeakFoot: 4, SkillMoves: 2, Height: 185, WeightKg: v(80), AccelerateType: "Lengthy", DetailedAttributes: &domain.DetailedAttributes{
		Acceleration: v(86), SprintSpeed: v(90), Reactions: v(91), DefensiveAwareness: v(93), StandingTackle: v(92), SlidingTackle: v(88), Strength: v(89), Aggression: v(85), Jumping: v(90), ShortPassing: v(82), Composure: v(88), BallControl: v(79),
	}}
}
