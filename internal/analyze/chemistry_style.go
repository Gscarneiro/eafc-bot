package analyze

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

//go:embed chemistry_styles/*.json
var chemistryStyleFiles embed.FS

// estiloEntrosamentoArquivo é deliberadamente separado dos perfis de meta:
// os valores de um consumível pertencem ao ciclo do jogo, enquanto os pesos
// da nota pertencem à opinião do perfil. Isso permite revisar um sem publicar
// silenciosamente o outro.
type estiloEntrosamentoArquivo struct {
	Ciclo        string                               `json:"ciclo"`
	Versao       string                               `json:"versao"`
	Status       string                               `json:"status"`
	Fonte        string                               `json:"fonte"`
	VerificadoEm string                               `json:"verificado_em"`
	Estilos      map[string]map[string]map[string]int `json:"estilos"`
}

// RegistroEstilosEntrosamento só expõe incrementos cujo ciclo e fonte foram
// confirmados. Uma tabela histórica pode ficar documentada fora daqui, mas
// não tem permissão para alterar a avaliação de outro ciclo.
type RegistroEstilosEntrosamento struct {
	porCiclo map[string]estiloEntrosamentoArquivo
}

func novoRegistroEstilosEntrosamento() (*RegistroEstilosEntrosamento, error) {
	entries, err := chemistryStyleFiles.ReadDir("chemistry_styles")
	if err != nil {
		return nil, fmt.Errorf("lendo estilos de entrosamento: %w", err)
	}
	r := &RegistroEstilosEntrosamento{porCiclo: make(map[string]estiloEntrosamentoArquivo, len(entries))}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		contents, err := chemistryStyleFiles.ReadFile("chemistry_styles/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("lendo estilos de entrosamento %s: %w", entry.Name(), err)
		}
		var table estiloEntrosamentoArquivo
		if err := json.Unmarshal(contents, &table); err != nil {
			return nil, fmt.Errorf("decodificando estilos de entrosamento %s: %w", entry.Name(), err)
		}
		if err := validarEstilosEntrosamento(table); err != nil {
			return nil, fmt.Errorf("estilos de entrosamento %s: %w", entry.Name(), err)
		}
		canonical := make(map[string]map[string]map[string]int, len(table.Estilos))
		for style, levels := range table.Estilos {
			key := normalizarNomeEstilo(style)
			if _, duplicate := canonical[key]; duplicate {
				return nil, fmt.Errorf("estilos de entrosamento %s: estilos duplicados após normalização: %s", entry.Name(), style)
			}
			canonical[key] = levels
		}
		table.Estilos = canonical
		if _, duplicate := r.porCiclo[table.Ciclo]; duplicate {
			return nil, fmt.Errorf("há mais de uma tabela de estilos para o ciclo %s", table.Ciclo)
		}
		r.porCiclo[table.Ciclo] = table
	}
	return r, nil
}

func validarEstilosEntrosamento(table estiloEntrosamentoArquivo) error {
	if strings.TrimSpace(table.Ciclo) == "" || strings.TrimSpace(table.Versao) == "" ||
		strings.TrimSpace(table.Status) == "" || strings.TrimSpace(table.Fonte) == "" {
		return fmt.Errorf("ciclo, versão, status e fonte são obrigatórios")
	}
	switch table.Status {
	case "confirmado":
		if len(table.Estilos) == 0 || strings.TrimSpace(table.VerificadoEm) == "" {
			return fmt.Errorf("tabela confirmada precisa de estilos e data de verificação")
		}
	case "nao_confirmado":
		if len(table.Estilos) != 0 {
			return fmt.Errorf("tabela não confirmada não pode carregar incrementos")
		}
	default:
		return fmt.Errorf("status desconhecido: %q", table.Status)
	}
	for style, porQuimica := range table.Estilos {
		if strings.TrimSpace(style) == "" {
			return fmt.Errorf("nome de estilo vazio")
		}
		for level, attributes := range porQuimica {
			n, err := strconv.Atoi(level)
			if err != nil || n < 0 || n > 3 || len(attributes) == 0 {
				return fmt.Errorf("nível inválido para %s: %q", style, level)
			}
			for attribute, gain := range attributes {
				if _, known := detailedAttributeNames[attribute]; !known || gain < 0 {
					return fmt.Errorf("incremento inválido de %s em %s", attribute, style)
				}
			}
		}
	}
	return nil
}

// incrementos devolve somente dados confirmados pelo ciclo. A ausência de
// regra nunca se transforma em zero escondido: ela vira uma limitação que a
// tela pode explicar ao jogador.
func (r *RegistroEstilosEntrosamento) incrementos(ctx domain.ContextoAvaliacao) (map[string]int, string) {
	style := strings.TrimSpace(ctx.EstiloEntrosamento)
	if style == "" {
		return nil, ""
	}
	table, found := r.porCiclo[ctx.Ciclo]
	if !found || table.Status != "confirmado" {
		return nil, "estilo de entrosamento " + style + " sem regra verificada"
	}
	if ctx.Quimica == nil {
		return nil, "estilo de entrosamento " + style + " sem química contextual"
	}
	level := *ctx.Quimica
	if level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}
	byChemistry, found := table.Estilos[normalizarNomeEstilo(style)]
	if !found {
		return nil, "estilo de entrosamento " + style + " sem regra verificada"
	}
	gains, found := byChemistry[strconv.Itoa(level)]
	if !found {
		return nil, "estilo de entrosamento " + style + " sem regra verificada para " + strconv.Itoa(level) + " de química"
	}
	return gains, ""
}

func normalizarNomeEstilo(style string) string {
	return strings.ToLower(strings.TrimSpace(style))
}

func nomesIncrementos(increments map[string]int) []string {
	names := make([]string, 0, len(increments))
	for name := range increments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
