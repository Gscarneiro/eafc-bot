package domain

import "testing"

func TestNormalizeFootAceitaCodigosENomes(t *testing.T) {
	tests := map[string]string{"1": "Direito", "right": "Direito", "2": "Esquerdo", "left": "Esquerdo", "Direito": "Direito", "": ""}
	for input, want := range tests {
		if got := NormalizeFoot(input); got != want {
			t.Fatalf("NormalizeFoot(%q) = %q, want %q", input, got, want)
		}
	}
}
