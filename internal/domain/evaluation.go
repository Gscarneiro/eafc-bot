package domain

import (
	"fmt"
	"strings"
)

// FonteAvaliacao identifica quem publicou a escala. Ela nunca é convertida
// implicitamente: uma diferença só faz sentido dentro da mesma fonte,
// perfil e contexto.
type FonteAvaliacao string

const (
	FonteFutGG  FonteAvaliacao = "futgg"
	FonteBot    FonteAvaliacao = "bot"
	FonteFUTBIN FonteAvaliacao = "futbin"
	FonteFUTWIZ FonteAvaliacao = "futwiz"
)

// ContextoAvaliacao reúne as escolhas que alteram o uso da carta em campo.
// A carta e o perfil permanecem imutáveis; contexto permite reproduzir uma
// resposta antiga sem confundir uma simulação com o clube observado.
type ContextoAvaliacao struct {
	Fonte              FonteAvaliacao `json:"fonte"`
	Perfil             string         `json:"perfil,omitempty"`
	VersaoPerfil       string         `json:"versao_perfil,omitempty"`
	VersaoMotor        string         `json:"versao_motor,omitempty"`
	Ciclo              string         `json:"ciclo,omitempty"`
	Patch              string         `json:"patch,omitempty"`
	Plataforma         string         `json:"plataforma,omitempty"`
	EstiloJogo         string         `json:"estilo_jogo,omitempty"`
	Funcao             string         `json:"funcao,omitempty"`
	Posicao            Position       `json:"posicao,omitempty"`
	Quimica            *int           `json:"quimica,omitempty"`
	EstiloEntrosamento string         `json:"estilo_entrosamento,omitempty"`
	RevisaoPlano       string         `json:"revisao_plano,omitempty"`
	Snapshot           string         `json:"snapshot,omitempty"`
}

// ComponenteAvaliacao explica uma parcela da nota. Valor é a contribuição na
// escala final, e não um atributo bruto disfarçado de recomendação.
type ComponenteAvaliacao struct {
	Chave  string  `json:"chave"`
	Rotulo string  `json:"rotulo"`
	Valor  float64 `json:"valor"`
}

// AvaliacaoCarta é a resposta única usada por mercado, elenco, evolução e
// Gauntlet. Indisponível continua explícita; zero nunca significa ausência.
type AvaliacaoCarta struct {
	Disponivel    bool                  `json:"disponivel"`
	Parcial       bool                  `json:"parcial"`
	Nota          float64               `json:"nota,omitempty"`
	Contexto      ContextoAvaliacao     `json:"contexto"`
	Componentes   []ComponenteAvaliacao `json:"componentes,omitempty"`
	PontosFortes  []string              `json:"pontos_fortes,omitempty"`
	Limitacoes    []string              `json:"limitacoes,omitempty"`
	DadosAusentes []string              `json:"dados_ausentes,omitempty"`
	Cobertura     []string              `json:"cobertura,omitempty"`
	Motivo        string                `json:"motivo,omitempty"`
}

// PerfilMeta descreve um perfil aprovado ou experimental sem acoplar a
// persistência ao formato interno dos pesos.
type PerfilMeta struct {
	ID          string   `json:"id"`
	Versao      string   `json:"versao"`
	Nome        string   `json:"nome"`
	Status      string   `json:"status"`
	Descricao   string   `json:"descricao,omitempty"`
	Plataformas []string `json:"plataformas,omitempty"`
	Evidencias  []string `json:"evidencias,omitempty"`
}

// PropostaMeta registra a mudança candidata antes de qualquer ativação. O
// aplicativo não aplica propostas: a aprovação pertence ao fluxo de entrega.
type PropostaMeta struct {
	ID                 string          `json:"id"`
	Pacote             string          `json:"pacote"`
	Ciclo              string          `json:"ciclo"`
	Patch              string          `json:"patch"`
	Plataformas        []string        `json:"plataformas"`
	Perfil             string          `json:"perfil"`
	VersaoCandidata    string          `json:"versao_candidata"`
	CriadaEm           string          `json:"criada_em"`
	AtualizadaEm       string          `json:"atualizada_em"`
	Status             string          `json:"status"` // proposta, aprovada, rejeitada, ativada, revertida
	MecanicasAfetadas  []string        `json:"mecanicas_afetadas,omitempty"`
	Fatos              []EvidenciaMeta `json:"fatos,omitempty"`
	Hipoteses          []EvidenciaMeta `json:"hipoteses,omitempty"`
	Evidencias         []string        `json:"evidencias,omitempty"` // snapshots legados
	Impacto            string          `json:"impacto,omitempty"`
	ResultadoAvaliacao string          `json:"resultado_avaliacao,omitempty"`
	Limitacoes         []string        `json:"limitacoes,omitempty"`
	MudancasCodigo     []MudancaMeta   `json:"mudancas_codigo,omitempty"`
}

// MudancaMeta registra um limite que não cabe só em pesos. Ela é uma
// proposta para desenvolvimento, não uma alteração automática do motor.
type MudancaMeta struct {
	Descricao string   `json:"descricao"`
	Ramo      string   `json:"ramo,omitempty"`
	Arquivos  []string `json:"arquivos,omitempty"`
}

// EvidenciaMeta preserva o que foi observado, onde, quando e até onde a
// conclusão vale. Fatos e hipóteses usam o mesmo formato, mas permanecem em
// listas separadas para uma publicação da EA não virar peso numérico sozinha.
type EvidenciaMeta struct {
	Descricao      string `json:"descricao"`
	Fonte          string `json:"fonte"`
	Data           string `json:"data"`
	Aplicabilidade string `json:"aplicabilidade,omitempty"`
}

func (p PropostaMeta) Validate() error {
	if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Pacote) == "" || strings.TrimSpace(p.Ciclo) == "" ||
		strings.TrimSpace(p.Patch) == "" || strings.TrimSpace(p.Perfil) == "" || strings.TrimSpace(p.VersaoCandidata) == "" {
		return fmt.Errorf("proposta de meta precisa de id, pacote, ciclo, patch, perfil e versão candidata")
	}
	switch p.Status {
	case "proposta", "aprovada", "rejeitada", "ativada", "revertida":
	default:
		return fmt.Errorf("status de proposta de meta inválido: %q", p.Status)
	}
	if len(p.Plataformas) == 0 {
		return fmt.Errorf("proposta de meta precisa declarar plataformas")
	}
	if len(p.Fatos) == 0 && len(p.Hipoteses) == 0 && len(p.Evidencias) == 0 {
		return fmt.Errorf("proposta de meta precisa de ao menos uma evidência ou hipótese")
	}
	for _, change := range p.MudancasCodigo {
		if strings.TrimSpace(change.Descricao) == "" {
			return fmt.Errorf("mudança de código precisa de descrição")
		}
	}
	return nil
}

// FeedbackGameplay separa impressão de campo, decisão de compra e resultado
// financeiro. Um id de comparação evita que cliques repetidos virem provas
// independentes do perfil.
type FeedbackGameplay struct {
	ID                  string   `json:"id"`
	ComparacaoID        string   `json:"comparacao_id"`
	Amostra             string   `json:"amostra,omitempty"` // ajuste ou avaliacao, definido pelo servidor
	Ciclo               string   `json:"ciclo"`
	CartaA              string   `json:"carta_a"`
	CartaB              string   `json:"carta_b"`
	CartaAID            int64    `json:"carta_a_id,omitempty"`
	CartaBID            int64    `json:"carta_b_id,omitempty"`
	CartaAClubItemID    string   `json:"carta_a_club_item_id,omitempty"`
	CartaBClubItemID    string   `json:"carta_b_club_item_id,omitempty"`
	Preferencia         string   `json:"preferencia"` // a, b, empate, insuficiente
	Patch               string   `json:"patch,omitempty"`
	Plataforma          string   `json:"plataforma,omitempty"`
	Posicao             Position `json:"posicao,omitempty"`
	Funcao              string   `json:"funcao,omitempty"`
	Quimica             *int     `json:"quimica,omitempty"`
	EstiloJogo          string   `json:"estilo_jogo,omitempty"`
	Perfil              string   `json:"perfil,omitempty"`
	Uso                 string   `json:"uso,omitempty"`
	DecisaoCompra       string   `json:"decisao_compra,omitempty"`
	ResultadoFinanceiro string   `json:"resultado_financeiro,omitempty"`
	RegistradoEm        string   `json:"registrado_em"`
}
