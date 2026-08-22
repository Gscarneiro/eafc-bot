package discover

import (
	"strings"
	"testing"
)

// O fixture usa "~" no lugar da crase e a troca na hora: uma string crua do
// Go não pode conter crase, e o JavaScript minificado é feito delas.
func js(s string) string { return strings.ReplaceAll(s, "~", "`") }

// bundleReal é o miolo do bundle do fut.gg, copiado como o servidor entrega:
// o montador de rotas de API (tc), o montador de rotas web (nc) — que NÃO
// pode ser confundido com o primeiro — e uma amostra das 141 chamadas.
const bundleReal = `
function tc(e){let t=e.path;t.startsWith(~/~)&&(t=t.slice(1)),t.endsWith(~/~)&&(t=t.slice(0,-1)),t=~/api/${e.isNotFutEndpoint?~~:~fut/~}${t}/~;let n=e.path.endsWith(~/~)?e.path:~${e.path}/~;return{...e,path:n,type:~api~,get fullPath(){return~${Qs.url.baseSiteUrl}${t}~}}}
function nc(e){let t=!!e.external,n=e.path;!t&&!n.endsWith(~/~)&&(n=~${n}/~);let r=n.startsWith(~/~)?n.slice(1):n;return{...e,path:n,type:~web~,get fullPath(){return t?r:~${Qs.url.baseSiteUrl}/${r}~}}}
var a=tc({path:~players/v2/26/~}),b=tc({path:~evolutions/v2/26/v3/all/~}),c=tc({path:~sbc~});
var d=tc({path:~/voting/entities/~,isNotFutEndpoint:!0});
var e2=tc({path:~/tactics/import-code/~,method:~POST~});
var f2=tc({path:~gg-debugger/purge-homepage-cache~});
var g2=tc({path:~moderation/ban-user/~});
var h2=nc({path:~/about~}),i2=nc({path:~/account/data-retention~});
`

func TestMineraORegistroDeRotasDoBundle(t *testing.T) {
	paths, skipped := mineRegistry(js(bundleReal))
	got := map[string]bool{}
	for _, p := range paths {
		got[p] = true
	}

	// O caminho vem relativo e sem barra inicial; quem completa é o
	// montador. Como a interpolação escolhe entre "" e "fut/", as duas
	// formas são possíveis e as duas são sondadas.
	for _, want := range []string{
		"/api/fut/players/v2/26/",
		"/api/players/v2/26/",
		"/api/fut/evolutions/v2/26/v3/all/",
		"/api/fut/sbc/", // uma palavra só: o interesting() reprovaria
		"/api/voting/entities/",
	} {
		if !got[want] {
			t.Errorf("%s não foi minerada (achou: %v)", want, paths)
		}
	}

	// Rotas que MUDAM estado não podem ser sondadas com GET. São duas
	// travas: o verbo declarado no registro e o nome do caminho.
	for _, naoQuero := range []string{
		"/api/fut/tactics/import-code/",              // method: POST
		"/api/fut/gg-debugger/purge-homepage-cache/", // "purge"
		"/api/fut/moderation/ban-user/",              // "ban"
	} {
		if got[naoQuero] {
			t.Errorf("%s é rota de escrita e foi minerada assim mesmo", naoQuero)
		}
	}
	if skipped != 3 {
		t.Errorf("pulou %d rotas de escrita, esperava 3", skipped)
	}

	// O montador de rotas WEB não prefixa nada e não pode ser tratado como
	// montador de API — se fosse, as ~142 páginas do site entrariam na fila
	// de sondagem e comeriam o orçamento inteiro.
	for _, p := range paths {
		if strings.Contains(p, "about") || strings.Contains(p, "data-retention") {
			t.Errorf("rota web %s entrou como rota de API", p)
		}
	}
}

// O nome do montador é minificado e muda a cada build do site. Reconhecê-lo
// pela FORMA do corpo é o que faz a descoberta sobreviver ao próximo deploy.
func TestReconheceOMontadorMesmoComOutroNome(t *testing.T) {
	renomeado := strings.ReplaceAll(bundleReal, "tc(", "Zq9$(")
	paths, _ := mineRegistry(js(renomeado))
	for _, p := range paths {
		if p == "/api/fut/players/v2/26/" {
			return
		}
	}
	t.Fatalf("não achou a rota com o montador renomeado (achou: %v)", paths)
}

// Um bundle sem registro nenhum não pode inventar rota.
func TestBundleSemRegistroNaoProduzRota(t *testing.T) {
	paths, skipped := mineRegistry(js(`var x=fetch(~/v3/algo/~);var y={path:~solto/~};`))
	if len(paths) != 0 {
		t.Errorf("inventou %v", paths)
	}
	if skipped != 0 {
		t.Errorf("pulou %d, esperava 0", skipped)
	}
}

// As rotas do registro não podem ser cortadas pelo teto de sondagem: são a
// única fonte em que o site declara o que é endpoint, e cortá-las para caber
// num limite pensado para literais soltos era o que devolvia descoberta
// vazia.
func TestOrcamentoNaoSacrificaAsRotasDoRegistro(t *testing.T) {
	var cands []Candidate
	for i := 0; i < 5; i++ {
		cands = append(cands, Candidate{URL: "https://x/reg/", Registry: true})
	}
	for i := 0; i < 50; i++ {
		cands = append(cands, Candidate{URL: "https://x/solta/"})
	}

	out := withinBudget(cands, 10)
	if len(out) != 10 {
		t.Fatalf("devolveu %d, esperava 10", len(out))
	}
	registry := 0
	for _, c := range out {
		if c.Registry {
			registry++
		}
	}
	if registry != 5 {
		t.Errorf("sobraram %d rotas de registro, esperava as 5", registry)
	}

	// Teto menor que o próprio registro: passa o registro inteiro mesmo
	// assim. Estourar o orçamento é melhor que voltar sem a API.
	if out := withinBudget(cands, 3); len(out) != 5 {
		t.Errorf("com teto 3 devolveu %d, esperava as 5 do registro", len(out))
	}
}
