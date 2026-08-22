package config

import "testing"

// O usuário pensa em link, não em slug ("já passei o link"). As três formas
// abaixo têm que produzir o mesmo valor — é o que o fut.gg de fato aceita
// na URL, e é o único identificador que a rota do clube reconhece.
func TestNormalizeProfileAceitaOLinkColado(t *testing.T) {
	casos := map[string]string{
		"https://www.fut.gg/gg-club/BilingualBee/": "BilingualBee",
		"fut.gg/gg-club/BilingualBee":              "BilingualBee",
		"BilingualBee":                             "BilingualBee",
		"https://www.fut.gg/gg-club/BilingualBee":  "BilingualBee",
		"www.fut.gg/gg-club/BilingualBee/?x=1":     "BilingualBee",
		"":                                         "",
	}
	for in, want := range casos {
		if got := normalizeProfile(in); got != want {
			t.Errorf("normalizeProfile(%q) = %q, esperava %q", in, got, want)
		}
	}
}
