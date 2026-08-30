import type {
  CardDetailResponse,
  ConfigResponse,
  EvolutionFavoritesResponse,
  EvolutionPlanResponse,
  EvolutionProgressResponse,
  EvolutionCatalogCollection,
  EvolutionCatalogDetailResponse,
  EvolutionAnalysisResponse,
  EvolutionPathsCollection,
  SavedEvolutionPathView,
  SavedEvolutionPathsResponse,
  GauntletResponse,
  JobStatus,
	Agenda,
	MarketPlanResponse,
  SquadPlanResponse,
  StatusResponse,
  TimeResponse,
  UISettings,
  ODataPage,
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
export const fetchMarketPlan = () => getJSON<MarketPlanResponse>("/api/planos/mercado");
export const fetchAgenda = () => getJSON<Agenda>("/api/agenda");
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

export async function triggerJob(): Promise<void> {
  const res = await fetch("/api/job", { method: "POST" });
  if (!res.ok) {
    throw new ApiError(res.status, `POST /api/job devolveu ${res.status}`);
  }
}
