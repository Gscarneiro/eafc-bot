import type {
  CardDetailResponse,
  ConfigResponse,
  EvolucoesResponse,
  EvolutionFavoritesResponse,
  InvestimentosResponse,
  JobStatus,
  MercadoResponse,
  StatusResponse,
  TimeResponse,
  UISettings,
} from "./types";

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

export const fetchStatus = () => getJSON<StatusResponse>("/api/status");
export interface TimeQuery { page?: number; search?: string; position?: string; tradeable?: "all" | "tradeable" | "untradeable" }
export const fetchTime = (query: TimeQuery = {}) => {
  const params = new URLSearchParams();
  if (query.page && query.page > 1) params.set("bench_page", String(query.page));
  if (query.search) params.set("bench_search", query.search);
  if (query.position) params.set("bench_position", query.position);
  if (query.tradeable && query.tradeable !== "all") params.set("bench_tradeable", query.tradeable);
  const suffix = params.toString();
  return getJSON<TimeResponse>(`/api/time${suffix ? `?${suffix}` : ""}`);
};
export const fetchCard = (slug: string) => getJSON<CardDetailResponse>(`/api/time/${encodeURIComponent(slug)}`);
export const fetchMercado = () => getJSON<MercadoResponse>("/api/mercado");
export const fetchEvolucoes = (query = "") => getJSON<EvolucoesResponse>(`/api/evolucoes${query ? `?${query}` : ""}`);
export const fetchEvolutionFavorites = () => getJSON<EvolutionFavoritesResponse>("/api/evolucoes/favoritos");
export async function saveEvolutionFavorites(favorites: string[]): Promise<EvolutionFavoritesResponse> {
  const res = await fetch("/api/evolucoes/favoritos", { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ favorites }) });
  if (!res.ok) throw new ApiError(res.status, `PUT /api/evolucoes/favoritos devolveu ${res.status}`);
  return res.json();
}
export const fetchInvestimentos = () => getJSON<InvestimentosResponse>("/api/investimentos");
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
