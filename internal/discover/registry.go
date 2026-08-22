package discover

import (
	"regexp"
	"sort"
	"strings"
)

// Um SPA moderno não escreve a URL da API por extenso em lugar nenhum. Ele
// guarda um REGISTRO de rotas, com o caminho relativo:
//
//	tc({path:`players/v2/26/`})
//	tc({path:`/tactics/import-code/`,method:`POST`})
//
// e uma função que monta a URL de verdade só na hora da chamada:
//
//	function tc(e){let t=e.path;t.startsWith(`/`)&&(t=t.slice(1));
//	  t=`/api/${e.isNotFutEndpoint?``:`fut/`}${t}/`; … }
//
// É por isso que o pathLit do crawl.go volta de mãos vazias justamente nas
// rotas que importam: "players/v2/26/" não começa com barra, e o "/api/fut/"
// que o completa mora a quilômetros de distância, dentro do montador.
//
// A saída é ler o bundle como o navegador lê. Primeiro descobrir QUEM é o
// montador — pela FORMA do corpo dele, nunca pelo nome, que é minificado e
// muda a cada build — e daí seguir as chamadas. Nada aqui conhece "fut.gg",
// "/api/" ou "fut/": esses três saem do próprio bundle, e é isso que mantém
// a descoberta viva quando a API for reorganizada no FC 27.

// bt é a crase. Os padrões abaixo são strings cruas para não virarem uma
// parede de contrabarras, e string crua é justamente o que não pode conter
// uma crase — então ela entra concatenada.
const bt = "`"

var (
	// builderFunc e builderArrow acham uma função de um argumento só que
	// monta um caminho prefixando um pedaço fixo. As duas formas existem
	// porque o bundler escolhe uma ou outra conforme o alvo de compilação.
	builderFunc = regexp.MustCompile(
		`function\s+([A-Za-z0-9_$]+)\s*\(\s*([A-Za-z0-9_$]+)\s*\)\s*\{` +
			`[^{}]{0,400}` + bt + `(/[A-Za-z0-9/_.-]*/)\$\{`)
	builderArrow = regexp.MustCompile(
		`([A-Za-z0-9_$]+)\s*=\s*\(?\s*([A-Za-z0-9_$]+)\s*\)?\s*=>\s*\{` +
			`[^{}]{0,400}` + bt + `(/[A-Za-z0-9/_.-]*/)\$\{`)

	// headFragment acha os sufixos que a interpolação logo depois do prefixo
	// pode inserir. No fut.gg é o `fut/` do ternário, e é ele que faz
	// "/api/" e "/api/fut/" serem ambos possíveis para a mesma rota.
	headFragment = regexp.MustCompile(bt + `([A-Za-z0-9_.-]+/)` + bt)

	// methodLit lê o verbo que a própria entrada do registro declara.
	methodLit = regexp.MustCompile(`(?i)\bmethod\s*:\s*['"` + bt + `]([A-Za-z]+)`)
)

// routeBuilder é um montador de URL achado no bundle, com os prefixos que
// ele sabe aplicar.
type routeBuilder struct {
	name     string
	prefixes []string
}

// findBuilders varre o texto atrás dos montadores de URL.
//
// A checagem que segura o falso positivo é `<arg>.path`: a função tem que
// ler o caminho DO ARGUMENTO que recebeu. Sem ela, qualquer função que monte
// uma URL fixa viraria "montador" e arrastaria junto centenas de rotas de
// página. É também o que descarta sozinho o montador das rotas web do
// fut.gg, que não tem prefixo estático nenhum.
func findBuilders(text string) []routeBuilder {
	byName := map[string]map[string]bool{}

	scan := func(re *regexp.Regexp) {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			span := text[m[0]:m[1]]
			name, arg, head := text[m[2]:m[3]], text[m[4]:m[5]], text[m[6]:m[7]]
			if !strings.Contains(span, arg+".path") {
				continue
			}
			set := byName[name]
			if set == nil {
				set = map[string]bool{}
				byName[name] = set
			}
			set[head] = true

			end := m[1] + 200
			if end > len(text) {
				end = len(text)
			}
			for _, f := range headFragment.FindAllStringSubmatch(text[m[1]:end], 4) {
				set[head+f[1]] = true
			}
		}
	}
	scan(builderFunc)
	scan(builderArrow)

	out := make([]routeBuilder, 0, len(byName))
	for name, set := range byName {
		b := routeBuilder{name: name}
		for p := range set {
			b.prefixes = append(b.prefixes, p)
		}
		sort.Strings(b.prefixes)
		out = append(out, b)
	}
	// Ordem estável: a lista alimenta o orçamento de sondagem, e um
	// autoconfig que sonda rotas diferentes a cada execução é impossível de
	// depurar.
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// callSites monta a expressão que acha as chamadas de um montador e o
// caminho literal de cada uma. O segundo grupo é o resto da chamada, onde
// mora o `method:` quando a entrada declara um.
func callSites(name string) *regexp.Regexp {
	return regexp.MustCompile(
		`\b` + regexp.QuoteMeta(name) +
			`\(\{\s*(?:path|url|endpoint|route)\s*:\s*` +
			`['"` + bt + `]([^'"` + bt + `]+)['"` + bt + `]([^)]{0,200})`)
}

// writeVerbs marcam rota que MUDA estado. O registro do fut.gg tem
// "gg-debugger/purge-homepage-cache", "moderation/ban-user/" e
// "/evo-lab/delete-gg-club-players/" no meio das rotas de leitura. Sondar
// essas com GET pode disparar efeito de verdade do outro lado — limpar um
// cache, apagar um rascunho, banir alguém. Uma descoberta que só quer LER
// não tem nada a fazer nelas.
var writeVerbs = []string{
	"create", "delete", "remove", "save", "purge", "ban", "report",
	"reset", "update", "set-", "sign-", "password", "takeover", "unlink",
	"submit", "vote", "join", "onboard", "sync", "import", "export",
	"moderation", "debugger", "upload", "complete", "start",
}

// readOnly decide se a rota pode ser sondada com GET. São duas travas
// independentes de propósito: o verbo que o registro declara e o nome do
// caminho. Basta uma apontar escrita para a rota ficar de fora — errar para
// o lado de não tocar é barato, errar para o outro lado não tem desfazer.
func readOnly(path, rest string) bool {
	if m := methodLit.FindStringSubmatch(rest); m != nil && !strings.EqualFold(m[1], "GET") {
		return false
	}
	l := strings.ToLower(path)
	for _, v := range writeVerbs {
		if strings.Contains(l, v) {
			return false
		}
	}
	return true
}

// mineRegistry devolve as rotas de API declaradas no registro do bundle, já
// com o prefixo aplicado, e quantas ficaram de fora por serem de escrita.
func mineRegistry(text string) (paths []string, skippedWrites int) {
	seen := map[string]bool{}
	for _, b := range findBuilders(text) {
		for _, m := range callSites(b.name).FindAllStringSubmatch(text, -1) {
			raw, rest := m[1], m[2]
			if !readOnly(raw, rest) {
				skippedWrites++
				continue
			}
			// O montador normaliza as barras antes de concatenar; fazemos o
			// mesmo, senão "sbc" e "/sbc/" viram duas rotas diferentes.
			trimmed := strings.Trim(raw, "/")
			if trimmed == "" {
				continue
			}
			for _, pre := range b.prefixes {
				full := pre + trimmed + "/"
				if seen[full] {
					continue
				}
				seen[full] = true
				paths = append(paths, full)
			}
		}
	}
	sort.Strings(paths)
	return paths, skippedWrites
}
