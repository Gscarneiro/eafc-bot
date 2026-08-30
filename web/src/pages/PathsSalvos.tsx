import { useState } from "react";
import { deleteSavedEvolutionPath, fetchSavedEvolutionPaths } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatDateTime, formatSigned } from "../format";
import { useData } from "../useData";
import type { SavedEvolutionPathView } from "../types";
import "../shared.css";
import "./Evolucoes.css";

const STATUS_TONE: Record<SavedEvolutionPathView["status"], ChipTone> = { disponivel: "gain", alterado: "turf", expirado: "alert", indisponivel: "flat" };

export default function PathsSalvos() {
  const state = useData(fetchSavedEvolutionPaths, []);
  const [removing, setRemoving] = useState("");
  const [error, setError] = useState("");
  const gate = asyncGate(state.loading || false, state.error, state.data !== null, state.refetch);
  if (gate) return gate;
  const entries = state.data?.value ?? [];
  const remove = async (id: string) => { setRemoving(id); setError(""); try { await deleteSavedEvolutionPath(id); state.refetch(); } catch (reason) { setError(reason instanceof Error ? reason.message : String(reason)); } finally { setRemoving(""); } };
  return <div className="wrap evo-saved-page"><PageHeader eyebrow="evoluções" title="Paths salvos" meta="Fotografias locais: o que você decidiu guardar continua aqui mesmo se o jogo mudar." />
    {error && <p className="evo-warning">{error}</p>}
    {entries.length === 0 ? <EmptyState message="Nenhum path salvo ainda." hint="Abra Análise do elenco e use “salvar path” na rota que quiser acompanhar." /> : <div className="evo-saved-list">{entries.map(({ saved, status }) => <article className="evo-saved-card" key={saved.id}><div className="evo-saved-head"><div>{saved.player.image_url && <img src={saved.player.image_url} alt="" />}<div><span className="evo-hero-kicker">salvo {formatDateTime(saved.saved_at)}</span><h2>{saved.player.common_name || saved.player.name}</h2><p>{saved.player.rating} OVR · GG final {saved.final_gg_rating.toFixed(1)} · {formatSigned(saved.gg_rating_gain)} GG</p></div></div><Chip tone={STATUS_TONE[status]}>{status}</Chip></div><div className="evo-saved-chain">{saved.path.chain?.join(" → ") || "Evolução sem título publicado"}</div><div className="evo-saved-meta"><span>{saved.final_overall} OVR final</span><span>{saved.impact.kind.replaceAll("_", " ")}</span>{saved.impact.position && <span>vaga {saved.impact.position}</span>}</div><button className="evo-remove-path" type="button" disabled={removing === saved.id} onClick={() => void remove(saved.id)}>{removing === saved.id ? "removendo…" : "remover dos salvos"}</button></article>)}</div>}
  </div>;
}
