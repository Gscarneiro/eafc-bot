import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { fetchEvolutionFavorites, saveEvolutionFavorites } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import ExpandIcon from "../components/ExpandIcon";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import { formatCoins, formatSigned } from "../format";
import { formatFilter, type Filter } from "../odata";
import { useCollection } from "../useCollection";
import { useData } from "../useData";
import type { EvoMatch, EvoPotential, EvolucoesCollection } from "../types";
import "../shared.css";
import "./Evolucoes.css";

function acquisitionLabel(value: string) { return value || "origem não identificada"; }

function EvolutionPath({ path }: { path: EvoPotential }) {
  const steps = path.path?.steps ?? [];
  return <div className="evo-path"><div className="evo-path-head"><span className="evo-path-kicker">melhor path fut.gg · {path.gg_rating_gain > 0 ? `${formatSigned(path.gg_rating_gain)} GG` : "potencial"}</span><span>{path.path.training_time || "tempo não informado"}</span></div><div className="evo-timeline" aria-label="Linha de tempo da evolução">{steps.map((step, index) => <div className="evo-step" key={`${step.id}-${index}`}><span className="evo-step-index">{String(index + 1).padStart(2, "0")}</span>{step.image_url && <img src={step.image_url} alt="" loading="lazy" />}<div><strong>{step.rating} {step.position}</strong><span>{step.gg_rating ? `GG ${step.gg_rating.toFixed(1)}` : index === 0 ? "carta atual" : "estado projetado"}</span></div>{index < steps.length - 1 && <span className="evo-step-arrow" aria-hidden="true">→</span>}</div>)}</div>{path.path.chain?.length ? <div className="chain">via {path.path.chain.join(" → ")}</div> : null}</div>;
}

function MatchRow({ match, index, open, favorite, onFavorite, onToggle }: { match: EvoMatch; index: number; open: boolean; favorite: boolean; onFavorite: () => void; onToggle: () => void }) {
  const objectives = (match.evolution.levels ?? []).flatMap((level) => level.objectives ?? []);
  return <article className={`rank-row evo-row${open ? " open" : ""}`}><div className="rank-main"><div className="rank-number">{String(index + 1).padStart(2, "0")}</div><div className="rank-player">{match.result.image_url && <img src={match.result.image_url} alt="" loading="lazy" />}<div className="rank-player-text"><strong className="rank-player-name">{match.evolution.name}</strong><span className="rank-player-sub">{match.player.common_name || match.player.name} · {match.slot} · {match.player.rating} OVR</span></div><button className={`evo-favorite${favorite ? " active" : ""}`} type="button" aria-label={favorite ? "remover evolução dos favoritos" : "acompanhar evolução"} aria-pressed={favorite} onClick={onFavorite}>{favorite ? "★" : "☆"}</button></div><div className="rank-metric"><span className="rank-metric-label">impacto</span><span className="rank-metric-value"><Chip tone="gain">{formatSigned(match.impact)}</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">resultado</span><span className="rank-metric-value"><Chip tone={match.beats_starter ? "gain" : "flat"}>{match.beats_starter ? "titular" : "reserva"}</Chip></span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">obtenção</span><span className="rank-metric-value">{acquisitionLabel(match.acquisition)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">custo</span><span className="rank-metric-value"><Chip tone={match.affordable ? "coin" : "alert"}>{match.cost ? formatCoins(match.cost) : "grátis"}</Chip></span></div><button className="rank-chevron" type="button" aria-expanded={open} onClick={onToggle}><ExpandIcon expanded={open} /></button></div>{open && <div className="rank-detail evo-detail"><div className="rank-detail-copy"><div className="evo-detail-heading"><strong>{match.evolution.name} em {match.player.common_name || match.player.name}</strong>{match.card_slug && <Link className="rank-link" to={`/time/${encodeURIComponent(match.card_slug)}`}>abrir carta →</Link>}</div>{match.highlights?.length ? <ul>{match.highlights.map((highlight, i) => <li key={i}>{highlight}</li>)}</ul> : null}{objectives.length > 0 && <div className="evo-objective-list"><span className="evo-objectives-label">desafios de conclusão</span>{objectives.map((objective, i) => <span className="evo-objective" key={i}>{objective}</span>)}</div>}</div><div className="rank-detail-side"><Chip tone={match.affordable ? "coin" : "alert"}>{match.affordable ? "dentro do orçamento" : "meta fora do orçamento"}</Chip><div className="rank-player-sub">GG fut.gg: {match.current_gg_rating.toFixed(1)} → {match.final_gg_rating.toFixed(1)}</div></div><div className="evo-detail-path"><EvolutionPath path={match.best_path} /></div></div>}</article>;
}

export default function Evolucoes() {
  const collection = useCollection<EvoMatch>("/api/evolucoes", { defaultOrderBy: [{ field: "impact", desc: true }], pageSize: 20 });
  const favoritesState = useData(fetchEvolutionFavorites, []);
  const [favorites, setFavorites] = useState<string[]>([]);
  const [params, setParams] = useSearchParams();
  useEffect(() => { if (favoritesState.data) setFavorites(favoritesState.data.favorites ?? []); }, [favoritesState.data]);
  const data = collection.raw as EvolucoesCollection | null;
  const gate = asyncGate(collection.loading || favoritesState.loading, collection.error ?? favoritesState.error, data !== null, collection.refetch);
  if (gate) return gate;
  if (!data) return null;
  const summary = data["@eafc.summary"] ?? { matches: collection.count, players: 0, starters: 0, unaffordable: 0, expiring_soon: 0, by_acquisition: {} };
  const position = params.get("position") || "todas";
  const impact = params.get("impact") || "todos";
  const category = params.get("category") || "todas";
  const status = params.get("status") || "todos";
  const search = params.get("q") || "";
  const sort = params.get("sort") || "impact";
  const open = params.get("open");
  const filterFor = (nextPosition = position, nextImpact = impact, nextCategory = category, nextStatus = status): Filter | undefined => {
    const filters: Filter[] = [];
    if (nextPosition !== "todas") filters.push({ kind: "compare", field: "slot", op: "eq", value: nextPosition });
    if (nextImpact !== "todos") filters.push({ kind: "compare", field: "beats_starter", op: "eq", value: nextImpact === "titular" });
    if (nextCategory !== "todas") filters.push({ kind: "compare", field: "acquisition", op: "eq", value: nextCategory });
    if (nextStatus !== "todos") filters.push({ kind: "compare", field: "affordable", op: "eq", value: nextStatus === "disponivel" });
    return filters.reduce<Filter | undefined>((result, item) => result ? { kind: "and", left: result, right: item } : item, undefined);
  };
  const setParam = (key: string, value: string, defaultValue: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === defaultValue) next.delete(key); else next.set(key, value);
    if (["position", "impact", "category", "status"].includes(key)) {
      const filter = filterFor(key === "position" ? value || "todas" : position, key === "impact" ? value || "todos" : impact, key === "category" ? value || "todas" : category, key === "status" ? value || "todos" : status);
      if (filter) next.set("$filter", formatFilter(filter)); else next.delete("$filter");
      next.delete("$skip");
    }
    if (key === "q") { if (value) next.set("$search", value); else next.delete("$search"); next.delete("$skip"); }
    if (key === "sort") { const field = value === "cost" ? "cost" : value === "result" ? "beats_starter" : "impact"; next.set("$orderby", `${field} ${value === "cost" ? "asc" : "desc"}`); next.delete("$skip"); }
    setParams(next);
  };
  const clear = () => { const next = new URLSearchParams(); setParams(next); };
  const positions = (collection.facets.slot ?? []).map((facet) => facet.value);
  const categories = (collection.facets.acquisition ?? []).map((facet) => facet.value);
  const hasFilters = Boolean(search || position !== "todas" || impact !== "todos" || category !== "todas" || status !== "todos");
  const toggleFavorite = async (id: string) => { const next = favorites.includes(id) ? favorites.filter((item) => item !== id) : [...favorites, id]; setFavorites(next); try { await saveEvolutionFavorites(next); } catch { setFavorites(favorites); } };
  return <div className="wrap evolutions-page"><PageHeader eyebrow="planejamento" title="Evoluções" meta="Escolha o investimento que muda seu XI — carta a carta." /><section className="evo-hero"><div><span className="evo-hero-kicker">painel de decisão</span><h2>O próximo salto do seu elenco</h2><p>Somente paths com ganho de GG Rating confirmado pelo fut.gg: {collection.count} combinações elegíveis.</p></div><div className="evo-hero-mark" aria-hidden="true">↗</div></section><div className="stat-grid evo-stat-grid"><div className="stat-tile"><div className="label">combinações</div><div className="value up">{summary.matches}</div><div className="sub">em {summary.players} cartas</div></div><div className="stat-tile"><div className="label">viram titular</div><div className="value up">{summary.starters}</div><div className="sub">prioridade de impacto</div></div><div className="stat-tile"><div className="label">expiram em 7 dias</div><div className="value down">{summary.expiring_soon}</div><div className="sub">não deixe para depois</div></div><div className="stat-tile"><div className="label">fora do orçamento</div><div className="value coin">{summary.unaffordable}</div><div className="sub">metas para juntar moedas</div></div></div><div className="evo-search"><label htmlFor="evo-search">buscar evolução ou carta</label><input id="evo-search" value={search} placeholder="ex.: atacante, nome da carta…" onChange={(event) => setParam("q", event.target.value, "")} /></div><RankingControls count={collection.count} sort={sort} onSort={(value) => setParam("sort", value, "impact")} options={[{ value: "impact", label: "maior impacto" }, { value: "result", label: "vira titular" }, { value: "cost", label: "menor custo" }]} hasFilters={hasFilters} onClear={clear}><FilterSelect label="posição" value={position} onChange={(value) => setParam("position", value, "todas")} options={[{ value: "todas", label: "todas" }, ...positions.map((value) => ({ value, label: value }))]} /><FilterSelect label="resultado" value={impact} onChange={(value) => setParam("impact", value, "todos")} options={[{ value: "todos", label: "todos" }, { value: "titular", label: "vira titular" }, { value: "reserva", label: "fica reserva" }]} /><FilterSelect label="orçamento" value={status} onChange={(value) => setParam("status", value, "todos")} options={[{ value: "todos", label: "todos" }, { value: "disponivel", label: "disponíveis" }, { value: "fora_orcamento", label: "fora do bolso" }]} /></RankingControls><div className="evo-acquisition-strip"><span className="evo-strip-label">por como se conclui</span>{categories.map((value) => <button key={value} className={`evo-acquisition-pill${category === value ? " active" : ""}`} type="button" onClick={() => setParam("category", category === value ? "todas" : value, "todas")}><strong>{summary.by_acquisition[value] ?? 0}</strong>{acquisitionLabel(value)}</button>)}</div>{collection.count === 0 ? <EmptyState message="Nenhum ganho de evolução foi confirmado pelo fut.gg para estes filtros." hint={hasFilters ? "Limpe os filtros para consultar todos os paths." : "A tela não exibe estimativas sem GG Rating válido."} /> : <><div className="rank-list">{collection.rows.map((match, index) => { const key = `${match.slot}-${match.player.id}-${match.evolution.id}`; return <MatchRow key={key} match={match} index={(collection.page - 1) * 20 + index} favorite={favorites.includes(match.evolution.id)} onFavorite={() => void toggleFavorite(match.evolution.id)} open={open === key} onToggle={() => setParam("open", open === key ? "" : key, "")} />; })}</div><Pagination page={collection.page} pages={collection.pages} onPage={collection.setPage} /></>}</div>;
}
