import type {
  CardDetailResponse,
  EvolucoesResponse,
  JobStatus,
  MercadoResponse,
  StatusResponse,
  TimeResponse,
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
export const fetchTime = () => getJSON<TimeResponse>("/api/time");
export const fetchCard = (slug: string) => getJSON<CardDetailResponse>(`/api/time/${encodeURIComponent(slug)}`);
export const fetchMercado = () => getJSON<MercadoResponse>("/api/mercado");
export const fetchEvolucoes = () => getJSON<EvolucoesResponse>("/api/evolucoes");
export const fetchJob = () => getJSON<JobStatus>("/api/job");

export async function triggerJob(): Promise<void> {
  const res = await fetch("/api/job", { method: "POST" });
  if (!res.ok) {
    throw new ApiError(res.status, `POST /api/job devolveu ${res.status}`);
  }
}
