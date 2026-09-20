import type {
  AgendaResponse,
  CardDetailResponse,
  ConfigResponse,
	EvaluationCatalogResponse,
  EvolutionFavoritesResponse,
  EvolutionPlanResponse,
  EvolutionProgressResponse,
  EvolutionProgressListResponse,
  EvolutionCatalogCollection,
  EvolutionCatalogDetailResponse,
  EvolutionAnalysisResponse,
  EvolutionPathsCollection,
  ExtratoResponse,
  MesaResponse,
  PosicoesResponse,
  SavedEvolutionPathView,
  SavedEvolutionPathsResponse,
  GauntletResponse,
	GameplayFeedback,
	GameplayFeedbackResponse,
	GameplayQualityResponse,
  JobStatus,
	MarketPlanResponse,
  ResumoResponse,
  SavedSquadPlanInput,
  SavedSquadPlanView,
  SavedSquadPlansResponse,
  SquadEditorEvaluation,
  SquadEditorResponse,
  SquadPlanSlot,
  SquadPlanResponse,
  StatusResponse,
  TimeResponse,
  UISettings,
  ODataPage,
  WatchlistCollection,
  GalleryPage,
  GalleryRecord,
  GalleryCollection,
  GalleryCompletion,
} from "./types";
import { toSearchParams, type ODataQuery } from "./odata";

// ApiError carrega o status HTTP para as telas distinguirem "ainda não
// coletou nada" (503, normal na primeira subida) de um erro de verdade.
export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path);
  if (!res.ok) {
    const text = (await res.text()).trim();
    throw new ApiError(res.status, text || `GET ${path} devolveu ${res.status}`);
  }
  return res.json();
}

export const fetchCollection = <T,>(path: string, query: ODataQuery = {}) => {
  const params = toSearchParams(query).toString();
  return getJSON<ODataPage<T>>(`${path}${params ? `?${params}` : ""}`);
};

export const fetchStatus = () => getJSON<StatusResponse>("/api/status");
export const fetchResumo = () => getJSON<ResumoResponse>("/api/resumo");
export const fetchGaleria = (query = "") => getJSON<GalleryPage>(`/api/galeria${query ? `?${query}` : ""}`);
export const fetchGaleriaDetalhe = (id: string) => getJSON<GalleryRecord>(`/api/galeria/${encodeURIComponent(id)}`);
export const fetchGaleriaColecao = () => getJSON<GalleryCollection>("/api/galeria/colecao");
export async function saveGaleriaCardOverride(cardId: number, input: { first_owner?: boolean; loan?: boolean; eligible?: boolean; item_score?: number }) { const res=await fetch(`/api/galeria/colecao/card/${encodeURIComponent(cardId)}`,{method:"PUT",headers:{"Content-Type":"application/json"},body:JSON.stringify({card_id:cardId,...input})}); if(!res.ok) throw new ApiError(res.status,"Não foi possível corrigir a carta."); return res.json(); }
export async function saveGaleriaConclusao(id: string, input: Omit<GalleryCompletion, "set_id">): Promise<GalleryCompletion> { const res=await fetch(`/api/galeria/${encodeURIComponent(id)}/conclusao`,{method:"PUT",headers:{"Content-Type":"application/json"},body:JSON.stringify(input)}); if(!res.ok) throw new ApiError(res.status,"Não foi possível salvar a conclusão."); return res.json(); }
export async function deleteGaleriaConclusao(id: string): Promise<void> { const res=await fetch(`/api/galeria/${encodeURIComponent(id)}/conclusao`,{method:"DELETE"}); if(!res.ok) throw new ApiError(res.status,"Não foi possível remover a conclusão."); }
export const fetchMarketPlan = () => getJSON<MarketPlanResponse>("/api/planos/mercado");
export const fetchAgenda = () => getJSON<AgendaResponse>("/api/agenda");
export const fetchWatchlist = () => getJSON<WatchlistCollection>("/api/watchlist");
export const fetchExtrato = (query = "") => getJSON<ExtratoResponse>(`/api/capital/extrato${query ? `?${query}` : ""}`);
export const fetchPosicoes = () => getJSON<PosicoesResponse>("/api/capital/posicoes");
export const fetchMesa = (index: number) => getJSON<MesaResponse>(`/api/mercado/mesa?index=${index}`);
export const fetchEvolutionProgressList = () => getJSON<EvolutionProgressListResponse>("/api/evolucoes/progresso");
export async function appendFeedback(entry: { action_id: string; status: "aceita" | "adiada" | "descartada"; reason?: string }) {
  const res = await fetch("/api/feedback", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
  if (!res.ok) throw new ApiError(res.status, "Não foi possível registrar o feedback local.");
  return res.json();
}
export async function addWatchlist(entry: { ea_id: number; name: string; target_coins?: number; note?: string; protected?: boolean }) {
  const res = await fetch("/api/watchlist", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
  if (!res.ok) throw new ApiError(res.status, "Não foi possível gravar a watchlist.");
  return res.json();
}
export async function appendLedger(entry: { kind: string; status: string; gross_coins: number; note?: string }) {
  const res = await fetch("/api/ledger", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
  if (!res.ok) throw new ApiError(res.status, "Não foi possível gravar o lançamento.");
  return res.json();
}
export const fetchTime = () => getJSON<TimeResponse>("/api/time");
export const fetchSquadEditor = () => getJSON<SquadEditorResponse>("/api/editor/elenco");
export const fetchSavedSquadPlans = () => getJSON<SavedSquadPlansResponse>("/api/planos/elenco/salvos");
export async function saveSquadPlan(input: SavedSquadPlanInput, id?: string): Promise<SavedSquadPlanView> {
  const method = id ? "PUT" : "POST";
  const path = id ? `/api/planos/elenco/salvos/${encodeURIComponent(id)}` : "/api/planos/elenco/salvos";
  const res = await fetch(path, { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(input) });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "Não foi possível salvar o plano."); }
  return res.json();
}
export async function deleteSquadPlan(id: string): Promise<void> {
  const res = await fetch(`/api/planos/elenco/salvos/${encodeURIComponent(id)}`, { method: "DELETE" });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "Não foi possível apagar o plano."); }
}
export async function applySquadPlanReference(id: string): Promise<SavedSquadPlanView> {
  const res = await fetch(`/api/planos/elenco/salvos/${encodeURIComponent(id)}/referencia`, { method: "POST" });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "Não foi possível aplicar a referência."); }
  return res.json();
}
export async function evaluateSquadEditor(formacao: string, vagas: SquadPlanSlot[], estiloJogo?: string): Promise<SquadEditorEvaluation> {
	const res = await fetch("/api/editor/elenco/avaliar", {
		method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ formacao, vagas, estilo_jogo: estiloJogo }),
  });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "Não foi possível avaliar o rascunho."); }
  return res.json();
}
export const fetchGameplayFeedback = () => getJSON<GameplayFeedbackResponse>("/api/feedback/gameplay");
export const fetchGameplayQuality = () => getJSON<GameplayQualityResponse>("/api/feedback/gameplay/qualidade");
export async function saveGameplayFeedback(entry: GameplayFeedback): Promise<GameplayFeedback> {
  const res = await fetch("/api/feedback/gameplay", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "Não foi possível registrar a comparação."); }
  return res.json();
}
export const fetchGauntlet = () => getJSON<GauntletResponse>("/api/gauntlet");
export async function fetchSquadPlan(): Promise<SquadPlanResponse> {
  const res = await fetch("/api/planos/elenco", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: "{}",
  });
  if (!res.ok) {
    const text = (await res.text()).trim();
    throw new ApiError(res.status, text || `POST /api/planos/elenco devolveu ${res.status}`);
  }
  return res.json();
}
export const fetchCard = (slug: string) => getJSON<CardDetailResponse>(`/api/time/${encodeURIComponent(slug)}`);
export const fetchEvolutionFavorites = () => getJSON<EvolutionFavoritesResponse>("/api/evolucoes/favoritos");
export async function saveEvolutionFavorites(favorites: string[]): Promise<EvolutionFavoritesResponse> {
  const res = await fetch("/api/evolucoes/favoritos", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ favorites }) });
  if (!res.ok) throw new ApiError(res.status, `PUT /api/evolucoes/favoritos devolveu ${res.status}`);
  return res.json();
}
export const fetchEvolutionPlan = (slug: string) => getJSON<EvolutionPlanResponse>(`/api/evolucoes/${encodeURIComponent(slug)}/plano`);
export const fetchEvolutionCatalog = (query = "") => getJSON<EvolutionCatalogCollection>(`/api/evolucoes/catalogo${query ? `?${query}` : ""}`);
export const fetchEvolutionCatalogDetail = (slug: string, playerKey?: string) => {
  const query = playerKey ? `?player_key=${encodeURIComponent(playerKey)}` : "";
  return getJSON<EvolutionCatalogDetailResponse>(`/api/evolucoes/catalogo/${encodeURIComponent(slug)}${query}`);
};
export async function requestEvolutionAnalysis(slug: string, playerKey: string, force = false): Promise<EvolutionAnalysisResponse> {
  const res = await fetch(`/api/evolucoes/catalogo/${encodeURIComponent(slug)}/analises`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ player_key: playerKey, force }),
  });
  if (!res.ok) {
    const text = (await res.text()).trim();
    throw new ApiError(res.status, text || `POST análise devolveu ${res.status}`);
  }
  return res.json();
}
export const fetchEvolutionAnalysis = (id: string) => getJSON<EvolutionAnalysisResponse>(`/api/evolucoes/analises/${encodeURIComponent(id)}`);
export const fetchEvolutionPaths = (query = "") => getJSON<EvolutionPathsCollection>(`/api/evolucoes/caminhos${query ? `?${query}` : ""}`);
export const fetchSavedEvolutionPaths = () => getJSON<SavedEvolutionPathsResponse>("/api/evolucoes/caminhos/salvos");
export async function saveEvolutionPath(pathID: string): Promise<SavedEvolutionPathView> {
  const res = await fetch("/api/evolucoes/caminhos/salvos", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ path_id: pathID }) });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "NÃ£o foi possÃ­vel salvar o path."); }
  return res.json();
}
export async function deleteSavedEvolutionPath(id: string): Promise<void> {
  const res = await fetch(`/api/evolucoes/caminhos/salvos/${encodeURIComponent(id)}`, { method: "DELETE" });
  if (!res.ok) { const text = (await res.text()).trim(); throw new ApiError(res.status, text || "NÃ£o foi possÃ­vel remover o path salvo."); }
}
export async function saveEvolutionProgress(slug: string, completed: string[]): Promise<EvolutionProgressResponse> {
  const res = await fetch(`/api/evolucoes/${encodeURIComponent(slug)}/progresso`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ completed }) });
  if (!res.ok) throw new ApiError(res.status, `PUT /api/evolucoes/${slug}/progresso devolveu ${res.status}`);
  return res.json();
}
export const fetchJob = () => getJSON<JobStatus>("/api/job");
export const fetchConfig = () => getJSON<ConfigResponse>("/api/config");
export const fetchEvaluationCatalog = () => getJSON<EvaluationCatalogResponse>("/api/avaliacao");

export async function saveConfig(settings: UISettings): Promise<ConfigResponse> {
  const res = await fetch("/api/config", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(settings),
  });
  if (!res.ok) {
    const text = (await res.text()).trim();
    throw new ApiError(res.status, text || `PUT /api/config devolveu ${res.status}`);
  }
  return res.json();
}

export async function saveSaldo(coins: number): Promise<{ coins: number }> {
  const res = await fetch("/api/saldo", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ coins }),
  });
  if (!res.ok) {
    const text = (await res.text()).trim();
    throw new ApiError(res.status, text || `PUT /api/saldo devolveu ${res.status}`);
  }
  return res.json();
}

export async function triggerJob(): Promise<void> {
  const res = await fetch("/api/job", { method: "POST" });
  if (!res.ok) {
    throw new ApiError(res.status, `POST /api/job devolveu ${res.status}`);
  }
}
