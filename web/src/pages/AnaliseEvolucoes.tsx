import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { fetchEvolutionPaths, saveEvolutionPath } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatCoins, formatSigned, styleNames } from "../format";
import { useData } from "../useData";
import type { EvolutionPathCandidate, EvolutionPathImpactKind, EvolutionPathsCollection } from "../types";
import "../shared.css";
import "./Evolucoes.css";

const STATUS_LABEL: Record<string, string> = { confirmed: "com path", no_path: "sem path ativo", not_eligible: "inelegível", fetch_error: "falha na coleta", not_checked: "não verificada" };
const IMPACT_LABEL: Record<EvolutionPathImpactKind, string> = { entra_no_xi: "entra no XI", melhora_titular: "melhora titular", nao_supera: "não supera", sem_comparacao: "comparação indisponível" };
const IMPACT_TONE: Record<EvolutionPathImpactKind, ChipTone> = { entra_no_xi: "gain", melhora_titular: "turf", nao_supera: "flat", sem_comparacao: "alert" };

function PathCard({ candidate, saved, onSave, saving }: { candidate: EvolutionPathCandidate; saved: boolean; onSave: () => void; saving: boolean }) {
  const steps = candidate.potential.path.steps ?? [];
  const final = steps[steps.length - 1];
  const impact = candidate.impact;
  return <article className={`evo-route${impact.kind === "entra_no_xi" ? " is-xi" : ""}`}>
    <div className="evo-route-rail" aria-hidden="true"><span /><i /><b /></div>
    <div className="evo-route-main">
      <div className="evo-route-head"><div><span className="evo-route-label">rota confirmada</span><strong>{candidate.potential.path.chain?.join(" → ") || "Evolução sem título publicado"}</strong></div><Chip tone={IMPACT_TONE[impact.kind]}>{IMPACT_LABEL[impact.kind]}</Chip></div>
      <div className="evo-route-outcome">
        <span><small>carta atual</small>GG {candidate.potential.final_gg_rating - candidate.potential.gg_rating_gain > 0 ? (candidate.potential.final_gg_rating - candidate.potential.gg_rating_gain).toFixed(1) : "—"}</span>
        <span aria-hidden="true">→</span>
        <span><small>final · {final?.gg_rating_pos || final?.position || "—"}</small>GG {candidate.potential.final_gg_rating > 0 ? candidate.potential.final_gg_rating.toFixed(1) : "—"}</span>
        {impact.kind !== "sem_comparacao" && <span className="evo-route-xi"><small>{impact.kind === "melhora_titular" ? "ganho na vaga" : "ganho sobre o XI"}</small>{formatSigned(impact.gain ?? 0)} GG</span>}
      </div>
      <div className="evo-route-meta"><span>{candidate.potential.coin_cost ? formatCoins(candidate.potential.coin_cost) : "grátis"}</span>{candidate.potential.point_cost > 0 && <span>{candidate.potential.point_cost} pontos</span>}{candidate.potential.training_time && <span>{candidate.potential.training_time}</span>}{candidate.potential.path.is_expired && <span className="evo-route-expired">expirada</span>}{candidate.potential.gained_play_styles?.length ? <span>{styleNames(candidate.potential.gained_play_styles)}</span> : null}</div>
    </div>
    <button className="evo-save-path" type="button" disabled={saved || saving} onClick={onSave}>{saved ? "salvo" : saving ? "salvando…" : "salvar path"}</button>
  </article>;
}

function finalPathPosition(candidate: EvolutionPathCandidate) {
  const steps = candidate.potential.path.steps ?? [];
  const final = steps[steps.length - 1];
  return final?.gg_rating_pos || final?.position || "";
}

export default function AnaliseEvolucoes() {
  const state = useData(() => fetchEvolutionPaths("$top=500"), []);
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("todas");
  const [status, setStatus] = useState("todas");
  const [xiOnly, setXIOnly] = useState(false);
  const [savedOnly, setSavedOnly] = useState(false);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [savedIDs, setSavedIDs] = useState<Set<string>>(new Set());
  const [savingID, setSavingID] = useState("");
  const [saveError, setSaveError] = useState("");
  const gate = asyncGate(state.loading || false, state.error, state.data !== null, state.refetch);
  const data = state.data as EvolutionPathsCollection | null;
  const sourceRows = data?.value ?? [];
  useEffect(() => { setSavedIDs(new Set(sourceRows.flatMap(row => (row.paths ?? []).filter(path => path.saved).map(path => path.id)))); }, [sourceRows]);
  const rows = useMemo(() => sourceRows.filter((row) => {
    const needle = search.trim().toLocaleLowerCase();
    if (needle && !`${row.player.common_name || row.player.name} ${row.player.version}`.toLocaleLowerCase().includes(needle)) return false;
    // O filtro responde à decisão que o path entrega, não ao lugar onde a
    // carta começou. Assim, uma LM cuja evolução final é RM aparece em RM.
    if (position !== "todas" && !(row.paths ?? []).some(path => finalPathPosition(path) === position)) return false;
    if (status !== "todas" && row.status !== status) return false;
    if (xiOnly && !row.entra_no_xi) return false;
    if (savedOnly && !(row.paths ?? []).some(path => savedIDs.has(path.id))) return false;
    return true;
  }), [position, savedIDs, savedOnly, search, sourceRows, status, xiOnly]);
  if (gate) return gate;
  if (!data) return null;
  const summary = data["@eafc.summary"];
  const save = async (pathID: string) => {
    setSavingID(pathID); setSaveError("");
    try { await saveEvolutionPath(pathID); setSavedIDs(previous => new Set(previous).add(pathID)); }
    catch (error) { setSaveError(error instanceof Error ? error.message : String(error)); }
    finally { setSavingID(""); }
  };
  return <div className="wrap evo-analysis-page">
    <PageHeader eyebrow="evoluções" title="Análise do elenco" meta="Todas as cartas OVR 88+ · decisão ordenada por GG Rating do fut.gg." />
    <section className="evo-analysis-hero"><div><span className="evo-hero-kicker">linha de decisão</span><h2>Da sua carta à vaga no XI.</h2><p>Cada rota confirmada termina numa comparação honesta com a escalação atual: entra, melhora, não supera ou ainda não há nota suficiente para afirmar.</p></div><div className="evo-analysis-scoreboard"><strong>{summary.entra_no_xi}</strong><span>paths entram no XI</span><small>{summary.melhora_titular} melhoram quem já começa</small></div></section>
    <div className="stat-grid evo-analysis-stats"><div className="stat-tile"><div className="label">cartas analisadas</div><div className="value">{summary.players}</div><div className="sub">OVR 88 ou mais</div></div><div className="stat-tile"><div className="label">com caminho</div><div className="value up">{summary.confirmed}</div><div className="sub">paths confirmados</div></div><div className="stat-tile"><div className="label">entram no XI</div><div className="value up">{summary.entra_no_xi}</div><div className="sub">GG posicional maior</div></div><div className="stat-tile"><div className="label">coleta pendente</div><div className="value down">{summary.fetch_error + summary.not_checked}</div><div className="sub">sem afirmação automática</div></div></div>
    <section className="evo-analysis-controls" aria-label="Filtros da análise"><label><span>buscar carta</span><input value={search} onChange={event => setSearch(event.target.value)} placeholder="nome ou versão" /></label><label><span>posição final do path</span><select value={position} onChange={event => setPosition(event.target.value)}><option value="todas">Todas</option>{["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"].map(value => <option key={value}>{value}</option>)}</select></label><label><span>cobertura</span><select value={status} onChange={event => setStatus(event.target.value)}><option value="todas">Todos os estados</option>{Object.entries(STATUS_LABEL).map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label><label className="evo-toggle"><input type="checkbox" checked={xiOnly} onChange={event => setXIOnly(event.target.checked)} /> só paths que entram no XI</label><label className="evo-toggle"><input type="checkbox" checked={savedOnly} onChange={event => setSavedOnly(event.target.checked)} /> só paths salvos</label></section>
    {saveError && <p className="evo-warning">{saveError}</p>}
    <p className="evo-analysis-count">{rows.length} carta{rows.length === 1 ? "" : "s"} na seleção</p>
    {rows.length === 0 ? <EmptyState message="Nenhuma carta atende aos filtros atuais." hint="Limpe a busca ou inclua outros estados de cobertura." /> : <div className="evo-analysis-list">{rows.map((row, index) => { const key = row.player.club_item_id || `${row.player.id}-${index}`; const paths = row.paths ?? []; const visiblePaths = position === "todas" ? paths : paths.filter(path => finalPathPosition(path) === position); const open = expanded === key; return <article className={`evo-player-analysis ${row.entra_no_xi ? "is-xi" : ""}`} key={key}><button className="evo-player-summary" type="button" aria-expanded={open} onClick={() => setExpanded(open ? null : key)}><div className="evo-player-thumb">{row.player.image_url ? <img src={row.player.image_url} alt="" loading="lazy" /> : <span>{row.player.position}</span>}</div><div className="evo-player-name"><strong>{row.player.common_name || row.player.name}</strong><span>{row.player.rating} OVR · {row.player.position} · {row.player.version}</span>{!row.identity_complete && <small>identidade aproximada</small>}</div><div className="evo-player-numbers"><span><small>melhor final</small>GG {row.best_final_gg_rating ? row.best_final_gg_rating.toFixed(1) : "—"}</span>{row.entra_no_xi && <span className="is-gain"><small>sobre o XI</small>{formatSigned(row.best_xi_gain ?? 0)} GG</span>}</div><Chip tone={row.entra_no_xi ? "gain" : row.status === "fetch_error" ? "alert" : "flat"}>{row.entra_no_xi ? "entra no XI" : STATUS_LABEL[row.status]}</Chip><span className="evo-expand">{open ? "fechar" : `${visiblePaths.length} path${visiblePaths.length === 1 ? "" : "s"}`}</span></button>{open && <div className="evo-player-routes">{visiblePaths.length > 0 ? visiblePaths.map(candidate => <PathCard key={candidate.id} candidate={candidate} saved={savedIDs.has(candidate.id)} saving={savingID === candidate.id} onSave={() => void save(candidate.id)} />) : <div className="evo-no-routes"><strong>{STATUS_LABEL[row.status]}</strong><p>{row.status === "fetch_error" ? "A coleta não confirmou caminhos para esta carta; rode uma nova atualização antes de decidir." : row.status === "not_checked" ? "Esta carta ainda não entrou no relatório de paths disponível no snapshot." : "Não há um path confirmado utilizável para esta carta na coleta atual."}</p></div>}{row.card_slug && <Link className="evo-card-link" to={`/time/${encodeURIComponent(row.card_slug)}`}>abrir carta completa →</Link>}</div>}</article>; })}</div>}
  </div>;
}
