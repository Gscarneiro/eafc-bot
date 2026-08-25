import type {
  CardDetailResponse,
  ConfigResponse,
  EvolutionFavoritesResponse,
  GauntletResponse,
  JobStatus,
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
