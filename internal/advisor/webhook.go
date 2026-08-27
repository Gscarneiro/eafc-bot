// Package advisor integra opcionalmente um agente especialista em evoluções.
// A integração é consultiva: o bot envia um JSON público, valida a resposta e
// nunca permite que o webhook execute ações na conta EA.
package advisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	ContractVersion = "evolution-analysis.v1"
	maxBodyBytes    = 256 << 10
)

// AnalysisResult é o contrato fechado que o agente deve devolver. Campos
// desconhecidos são ignorados pelo decoder, mas os obrigatórios e as fontes
// passam por validação fail-closed antes de chegar à UI.
type AnalysisResult struct {
	Verdict       string   `json:"verdict"`
	Summary       string   `json:"summary"`
	Strengths     []string `json:"strengths"`
	Risks         []string `json:"risks"`
	BestPositions []string `json:"best_positions"`
	Sources       []Source `json:"sources"`
}

type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Client é a pequena fronteira que a API usa; testes e o modo demo podem
// fornecer uma implementação local sem abrir rede.
type Client interface {
	Analyze(ctx context.Context, payload []byte) (AnalysisResult, error)
}

// Func adapta uma função ao contrato Client, útil para fixtures do servidor.
type Func func(context.Context, []byte) (AnalysisResult, error)

func (f Func) Analyze(ctx context.Context, payload []byte) (AnalysisResult, error) {
	return f(ctx, payload)
}

// Webhook é um cliente HTTP com política restrita de transporte. URL remota
// precisa ser HTTPS; HTTP só é aceito para loopback, o que permite um agente
// local sem abrir uma brecha óbvia quando o servidor escuta na LAN.
type Webhook struct {
	endpoint string
	token    string
	client   *http.Client
}

func New(endpoint, token string) (*Webhook, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return nil, fmt.Errorf("EAFC_EVO_AGENT_URL inválida — use uma URL HTTPS do webhook")
	}
	if u.Scheme != "https" && !isLoopbackHost(u.Hostname()) {
		return nil, fmt.Errorf("webhook do agente exige HTTPS (HTTP só é permitido em localhost)")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, fmt.Errorf("esquema do webhook deve ser https ou http local")
	}
	return &Webhook{
		endpoint: u.String(),
		token:    strings.TrimSpace(token),
		client: &http.Client{
			Timeout: 90 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func NewFromEnv() (*Webhook, error) {
	return New(os.Getenv("EAFC_EVO_AGENT_URL"), os.Getenv("EAFC_EVO_AGENT_TOKEN"))
}

func (w *Webhook) Analyze(ctx context.Context, payload []byte) (AnalysisResult, error) {
	if w == nil {
		return AnalysisResult{}, fmt.Errorf("agente de evoluções não configurado")
	}
	if len(payload) > maxBodyBytes {
		return AnalysisResult{}, fmt.Errorf("pedido do agente excede %d KB", maxBodyBytes/1024)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.endpoint, bytes.NewReader(payload))
	if err != nil {
		return AnalysisResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if w.token != "" {
		req.Header.Set("Authorization", "Bearer "+w.token)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("chamando agente de evoluções: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("lendo resposta do agente: %w", err)
	}
	if len(body) > maxBodyBytes {
		return AnalysisResult{}, fmt.Errorf("resposta do agente excede %d KB", maxBodyBytes/1024)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AnalysisResult{}, fmt.Errorf("agente devolveu HTTP %d", resp.StatusCode)
	}
	var result AnalysisResult
	if err := json.Unmarshal(body, &result); err != nil {
		return AnalysisResult{}, fmt.Errorf("resposta do agente não é JSON estruturado: %w", err)
	}
	if err := ValidateResult(result); err != nil {
		return AnalysisResult{}, err
	}
	return result, nil
}

func ValidateResult(result AnalysisResult) error {
	switch result.Verdict {
	case "recomendada", "situacional", "nao_recomendada", "dados_insuficientes":
	default:
		return fmt.Errorf("resposta do agente tem verdict inválido")
	}
	if strings.TrimSpace(result.Summary) == "" {
		return fmt.Errorf("resposta do agente não trouxe summary")
	}
	if len(result.Sources) == 0 {
		return fmt.Errorf("resposta do agente precisa citar ao menos uma fonte")
	}
	for _, source := range result.Sources {
		u, err := url.Parse(strings.TrimSpace(source.URL))
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return fmt.Errorf("fonte do agente deve ser uma URL HTTPS válida")
		}
		if strings.TrimSpace(source.Title) == "" {
			return fmt.Errorf("fonte do agente sem título")
		}
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(strings.ToLower(host))
	if host == "localhost" || host == "localhost.localdomain" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
