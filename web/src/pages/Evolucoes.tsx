import { useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { fetchEvolutionFavorites, fetchEvolucoes, saveEvolutionFavorites } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import ExpandIcon from "../components/ExpandIcon";
import PageHeader from "../components/PageHeader";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import { formatCoins, formatSigned } from "../format";
import { useData } from "../useData";
import type { EvoMatch, EvoPotential } from "../types";
import "../shared.css";
import "./Evolucoes.css";

function acquisitionLabel(value: string): string {
  return value === "origem não identificada" ? "origem não identificada" : value;
}

type EvoSortField = "evolution" | "impact" | "result" | "acquisition" | "cost";
type SortDirection = "asc" | "desc";
interface EvoSortCriterion { field: EvoSortField; direction: SortDirection; }

const defaultSortDirections: Record<EvoSortField, SortDirection> = {
  evolution: "asc",
  impact: "desc",
  result: "desc",
  acquisition: "asc",
  cost: "asc",
};

function parseSortSpec(value: string): EvoSortCriterion[] {
  const seen = new Set<string>();
  const parsed: EvoSortCriterion[] = [];
  for (const item of value.split(",")) {
    const [rawField, rawDirection] = item.trim().toLowerCase().split(":");
    const field = (rawField === "gain" ? "impact" : rawField) as EvoSortField;
    if (!(field in defaultSortDirections) || seen.has(field)) continue;
    seen.add(field);
    parsed.push({ field, direction: rawDirection === "asc" || rawDirection === "desc" ? rawDirection : defaultSortDirections[field] });
  }
  return parsed.length > 0 ? parsed : [{ field: "impact", direction: "desc" }];
}

function toggleSortCriterion(criteria: EvoSortCriterion[], field: EvoSortField): EvoSortCriterion[] {
  const index = criteria.findIndex((item) => item.field === field);
  if (index < 0) return [...criteria, { field, direction: defaultSortDirections[field] }];
  const current = criteria[index];
  if (current.direction === defaultSortDirections[field]) {
    const next = criteria.slice();
    next[index] = { ...current, direction: current.direction === "desc" ? "asc" : "desc" };
    return next;
  }
  const next = criteria.filter((item) => item.field !== field);
  return next.length > 0 ? next : [{ field: "impact", direction: "desc" }];
}

function SortColumn({ field, label, criteria, onChange, optional = false }: { field: EvoSortField; label: string; criteria: EvoSortCriterion[]; onChange: (field: EvoSortField) => void; optional?: boolean }) {
  const index = criteria.findIndex((item) => item.field === field);
  const criterion = index >= 0 ? criteria[index] : undefined;
  const directionLabel = criterion?.direction === "desc" ? "decrescente" : "crescente";
  return (
    <div className={`evo-sort-column${optional ? " optional-sort" : ""}`} role="columnheader" aria-sort={criterion ? (criterion.direction === "desc" ? "descending" : "ascending") : "none"}>
      <button type="button" className={criterion ? "active" : ""} onClick={() => onChange(field)} aria-label={`${label}: ${criterion ? `${directionLabel}, prioridade ${index + 1}` : "não ordenado"}. Clique para alterar.`}>
        <span>{label}</span>{criterion && <span className="evo-sort-state" aria-hidden="true">{criterion.direction === "desc" ? "↓" : "↑"}<b>{index + 1}</b></span>}
      </button>
    </div>
  );
}

function EvolutionPath({ path }: { path: EvoPotential }) {
  const steps = path.path?.steps ?? [];
  return (
    <div className="evo-path">
      <div className="evo-path-head"><span className="evo-path-kicker">melhor path fut.gg · {path.gg_rating_gain > 0 ? `${formatSigned(path.gg_rating_gain)} GG` : "potencial"}</span><span>{path.path.training_time || "tempo não informado"}</span></div>
      <div className="evo-timeline" aria-label="Linha de tempo da evolução">
        {steps.map((step, index) => <div className="evo-step" key={`${step.id}-${index}`}><span className="evo-step-index">{String(index + 1).padStart(2, "0")}</span>{step.image_url && <img src={step.image_url} alt="" loading="lazy" />}<div><strong>{step.rating} {step.position}</strong><span>{step.gg_rating ? `GG ${step.gg_rating.toFixed(1)}` : index === 0 ? "carta atual" : "estado projetado"}</span></div>{index < steps.length - 1 && <span className="evo-step-arrow" aria-hidden="true">→</span>}</div>)}
      </div>
      {(path.path.chain ?? []).length > 0 && <div className="chain">via {(path.path.chain ?? []).join(" → ")}</div>}
    </div>
  );
}

function MatchRow({ match, index, open, favorite, onFavorite, onToggle }: { match: EvoMatch; index: number; open: boolean; favorite: boolean; onFavorite: () => void; onToggle: () => void }) {
  const objectives = (match.evolution.levels ?? []).flatMap((level) => level.objectives ?? []);
  return (
    <article className={`rank-row evo-row${open ? " open" : ""}`}>
      <div className="rank-main">
        <div className="rank-number">{String(index + 1).padStart(2, "0")}</div>
        <div className="rank-player">{match.result.image_url && <img src={match.result.image_url} alt="" loading="lazy" />}<div className="rank-player-text"><strong className="rank-player-name">{match.evolution.name}</strong><span className="rank-player-sub">{match.player.common_name || match.player.name} · {match.slot} · {match.player.rating} OVR</span></div><button className={`evo-favorite${favorite ? " active" : ""}`} type="button" aria-label={favorite ? "remover evolução dos favoritos" : "acompanhar evolução"} aria-pressed={favorite} onClick={onFavorite}>{favorite ? "★" : "☆"}</button></div>
        <div className="rank-metric"><span className="rank-metric-label">impacto</span><span className="rank-metric-value evo-impact-value"><Chip tone="gain">{formatSigned(match.impact)}</Chip><small>GG fut.gg</small></span></div>
        <div className="rank-metric"><span className="rank-metric-label">resultado</span><span className="rank-metric-value"><Chip tone={match.beats_starter ? "gain" : "flat"}>{match.beats_starter ? "titular" : "reserva"}</Chip></span></div>
        <div className="rank-metric optional-metric"><span className="rank-metric-label">obtenção</span><span className="rank-metric-value">{acquisitionLabel(match.acquisition)}</span></div>
        <div className="rank-metric optional-metric"><span className="rank-metric-label">custo</span><span className="rank-metric-value"><Chip tone={match.affordable ? "coin" : "alert"}>{match.cost ? formatCoins(match.cost) : "grátis"}</Chip></span></div>
        <button className="rank-chevron" type="button" aria-expanded={open} aria-label={open ? "recolher detalhes" : "abrir detalhes"} onClick={onToggle}><ExpandIcon expanded={open} /></button>
      </div>
      {open && <div className="rank-detail evo-detail"><div className="rank-detail-copy"><div className="evo-detail-heading"><strong>{match.evolution.name} em {match.player.common_name || match.player.name}</strong>{match.card_slug && <Link className="rank-link" to={`/time/${encodeURIComponent(match.card_slug)}`}>abrir carta →</Link>}</div>{(match.highlights ?? []).length > 0 && <ul>{match.highlights!.map((highlight, i) => <li key={i}>{highlight}</li>)}</ul>}{objectives.length > 0 && <div className="evo-objective-list"><span className="evo-objectives-label">desafios de conclusão</span>{objectives.map((objective, i) => <span className="evo-objective" key={i}>{objective}</span>)}</div>}</div><div className="rank-detail-side"><div className="evo-detail-cost"><Chip tone={match.affordable ? "coin" : "alert"}>{match.affordable ? "dentro do orçamento" : "meta fora do orçamento"}</Chip><span>{match.cost ? `${formatCoins(match.cost)} moedas` : "sem custo em moedas"}</span>{match.evolution.point_cost > 0 && <span>{match.evolution.point_cost} pontos</span>}</div><div className="rank-player-sub">GG fut.gg: {match.current_gg_rating.toFixed(1)} → {match.final_gg_rating.toFixed(1)}</div><div className="rank-player-sub">origem declarada: {acquisitionLabel(match.acquisition)}</div></div><div className="evo-detail-path"><EvolutionPath path={match.best_path} />{(match.alternates ?? []).length > 0 && <details className="evo-alternatives"><summary>{match.alternates!.length} path{match.alternates!.length === 1 ? " alternativo" : "s alternativos"}</summary>{match.alternates!.map((alternate, i) => <EvolutionPath key={i} path={alternate} />)}</details>}</div></div>}
    </article>
  );
}

// Evolucoes é o painel de decisão: o servidor filtra e pagina, enquanto a
// expansão revela o path completo sem transformar a lista em uma árvore.
export default function Evolucoes() {
  const [params, setParams] = useSearchParams();
  const paramString = params.toString();
  const requestQuery = useMemo(() => { const query = new URLSearchParams(paramString); query.delete("open"); return query.toString(); }, [paramString]);
  const { data, error, loading, refetch } = useData(() => fetchEvolucoes(requestQuery), [requestQuery]);
  const favoritesState = useData(fetchEvolutionFavorites, []);
  const [favorites, setFavorites] = useState<string[]>([]);
  useEffect(() => { if (favoritesState.data) setFavorites(favoritesState.data.favorites ?? []); }, [favoritesState.data]);
  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const matches = data.matches ?? [];
  const summary = data.summary ?? { matches: data.total, players: 0, starters: 0, unaffordable: 0, expiring_soon: 0, by_acquisition: {} };
  const open = params.get("open");
  const get = (key: string, fallback: string) => params.get(key) || fallback;
  const sort = get("sort", "impact:desc");
  const sortCriteria = parseSortSpec(sort);
  const position = get("position", "todas");
  const impact = get("impact", "todos");
  const category = get("category", "todas");
  const status = get("status", "todos");
  const expiring = get("expiring", "todas");
  const search = params.get("q") || "";
  const setParam = (key: string, value: string, defaultValue: string, resetPage = true) => { const next = new URLSearchParams(params); if (!value || value === defaultValue) next.delete(key); else next.set(key, value); if (resetPage && key !== "page") next.delete("page"); setParams(next); };
  const clearFilters = () => { const next = new URLSearchParams(); if (sort !== "impact:desc") next.set("sort", sort); setParams(next); };
  const changeSort = (field: EvoSortField) => {
    const next = toggleSortCriterion(sortCriteria, field);
    setParam("sort", next.map((item) => `${item.field}:${item.direction}`).join(","), "impact:desc");
  };
  const positions = data.filters?.positions?.length ? data.filters.positions.slice().sort() : Array.from(new Set(matches.map((m) => m.slot))).sort();
  const categories = data.filters?.categories?.length ? data.filters.categories.slice().sort() : Object.keys(summary.by_acquisition ?? {}).sort();
  const hasFilters = Boolean(search || position !== "todas" || impact !== "todos" || category !== "todas" || status !== "todos" || expiring !== "todas");
  const toggleFavorite = async (id: string) => {
    const next = favorites.includes(id) ? favorites.filter((item) => item !== id) : [...favorites, id];
    setFavorites(next);
    try { await saveEvolutionFavorites(next); } catch { setFavorites(favorites); }
  };

  return (
    <div className="wrap evolutions-page">
      <PageHeader eyebrow="planejamento" title="Evoluções" meta="Escolha o investimento que muda seu XI — carta a carta, passo a passo." />
      <section className="evo-hero"><div><span className="evo-hero-kicker">painel de decisão</span><h2>O próximo salto do seu elenco</h2><p>Somente paths com ganho de GG Rating confirmado pelo fut.gg: {data.total === 0 ? "nenhuma combinação encontrada" : `${data.total} combinação${data.total === 1 ? "" : "ões"} elegível${data.total === 1 ? "" : "eis"}`}.</p></div><div className="evo-hero-mark" aria-hidden="true">↗</div></section>
      <div className="stat-grid evo-stat-grid"><div className="stat-tile"><div className="label">combinações</div><div className="value up">{summary.matches}</div><div className="sub">em {summary.players} cartas</div></div><div className="stat-tile"><div className="label">viram titular</div><div className="value up">{summary.starters}</div><div className="sub">prioridade de impacto</div></div><div className="stat-tile"><div className="label">expiram em 7 dias</div><div className="value down">{summary.expiring_soon}</div><div className="sub">não deixe para depois</div></div><div className="stat-tile"><div className="label">fora do orçamento</div><div className="value coin">{summary.unaffordable}</div><div className="sub">metas para juntar moedas</div></div></div>
      <div className="evo-acquisition-strip"><span className="evo-strip-label">por como se conclui</span>{categories.map((key) => <button key={key} className={`evo-acquisition-pill${category === key ? " active" : ""}`} type="button" onClick={() => setParam("category", category === key ? "todas" : key, "todas")}><strong>{summary.by_acquisition[key] ?? 0}</strong>{acquisitionLabel(key)}</button>)}</div>
      <div className="evo-search"><label htmlFor="evo-search">buscar evolução ou carta</label><input id="evo-search" value={search} placeholder="ex.: atacante, nome da carta…" onChange={(e) => setParam("q", e.target.value, "")} /></div>
      <RankingControls count={data.total} sort={sort} onSort={(v) => setParam("sort", v, "impact:desc")} options={[]} hasFilters={hasFilters} onClear={clearFilters}>
        <FilterSelect label="posição" value={position} onChange={(v) => setParam("position", v, "todas")} options={[{ value: "todas", label: "todas" }, ...positions.map((p) => ({ value: p, label: p }))]} />
        <FilterSelect label="resultado" value={impact} onChange={(v) => setParam("impact", v, "todos")} options={[{ value: "todos", label: "todos" }, { value: "titular", label: "vira titular" }, { value: "reserva", label: "fica reserva" }]} />
        <FilterSelect label="orçamento" value={status} onChange={(v) => setParam("status", v, "todos")} options={[{ value: "todos", label: "todos" }, { value: "disponivel", label: "disponíveis" }, { value: "fora_orcamento", label: "fora do bolso" }]} />
        <FilterSelect label="prazo" value={expiring} onChange={(v) => setParam("expiring", v, "todas")} options={[{ value: "todas", label: "qualquer prazo" }, { value: "proxima", label: "expira em 7 dias" }]} />
      </RankingControls>
      <div className="evo-sort-toolbar"><p>Clique nos títulos para combinar critérios. O número mostra a prioridade; cliques seguintes invertem e depois removem.</p>{sort !== "impact:desc" && <button type="button" onClick={() => setParam("sort", "impact:desc", "impact:desc")}>restaurar ordenação</button>}</div>
      <div className="evo-sort-head" role="row" aria-label="Cabeçalhos e ordenação da lista">
        <span className="evo-sort-number" role="columnheader">#</span>
        <SortColumn field="evolution" label="evolução / carta" criteria={sortCriteria} onChange={changeSort} />
        <SortColumn field="impact" label="impacto" criteria={sortCriteria} onChange={changeSort} />
        <SortColumn field="result" label="resultado" criteria={sortCriteria} onChange={changeSort} />
        <SortColumn field="acquisition" label="obtenção" criteria={sortCriteria} onChange={changeSort} optional />
        <SortColumn field="cost" label="custo" criteria={sortCriteria} onChange={changeSort} optional />
        <span aria-hidden="true" />
      </div>
      {data.total === 0 ? <EmptyState message="Nenhum ganho de evolução foi confirmado pelo fut.gg para estas cartas." hint={hasFilters ? "Limpe os filtros para consultar todos os paths confirmados." : "A tela não exibe estimativas do bot nem paths sem GG Rating válido."} /> : <><div className="rank-list">{matches.map((match, i) => { const key = `${match.slot}-${match.player.id}-${match.evolution.id}`; return <MatchRow key={key} match={match} index={(data.page - 1) * data.page_size + i} favorite={favorites.includes(match.evolution.id)} onFavorite={() => void toggleFavorite(match.evolution.id)} open={open === key} onToggle={() => setParam("open", open === key ? "" : key, "", false)} />; })}</div>{data.pages > 1 && <nav className="evo-pagination" aria-label="Paginação de evoluções"><button type="button" disabled={data.page <= 1} onClick={() => setParam("page", String(data.page - 1), "1", false)}>← anterior</button><span>página {data.page} de {data.pages}</span><button type="button" disabled={data.page >= data.pages} onClick={() => setParam("page", String(data.page + 1), "1", false)}>próxima →</button></nav>}</>}
    </div>
  );
}
