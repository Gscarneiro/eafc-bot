import { useSearchParams } from "react-router-dom";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import ExpandIcon from "../components/ExpandIcon";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import { formatCoins, formatSigned } from "../format";
import { useCollection } from "../useCollection";
import type { FodderPhase, FodderSignal, Investment, InvestmentFunnel, InvestimentosCollection, SellCandidate, SellRecommendation } from "../types";
import "../shared.css";
import "./Investimentos.css";

export type CapitalSection = "investimentos" | "vendas" | "sbcs";

const RECOMMENDATION_LABEL: Record<SellRecommendation, string> = { vender: "vender", segurar_potencial: "segurar: potencial", promover: "promover", nao_vendavel: "não vendável" };
const RECOMMENDATION_TONE: Record<SellRecommendation, ChipTone> = { vender: "cost", segurar_potencial: "gain", promover: "gain", nao_vendavel: "flat" };
const PHASE_LABEL: Record<FodderPhase, string> = { recente: "recente", pico: "pico — não compre", esfriando: "esfriando", estavel: "estável", esvaziar: "esvaziar", expirado: "expirado" };
const PHASE_TONE: Record<FodderPhase, ChipTone> = { recente: "flat", pico: "alert", esfriando: "gain", estavel: "flat", esvaziar: "alert", expirado: "flat" };

function InvestmentsEmpty({ funnel }: { funnel: InvestmentFunnel }) {
  if (funnel.considered === 0) return <EmptyState message="O ciclo de momentum ainda não rodou." hint="Ele atualiza os sinais em intervalos curtos no modo serve." />;
  return <div className="empty" style={{ textAlign: "left" }}><div>Nenhuma carta passou do desconto mínimo de {funnel.min_momentum_pct.toFixed(1)}%.</div><div className="hint">Das {funnel.considered} cartas: {funnel.owned} já estão no clube · {funnel.not_tradeable} não são compráveis · {funnel.below_min_momentum} ficaram abaixo do piso.</div></div>;
}

function InvestmentRows({ rows, open, onOpen }: { rows: Investment[]; open: string | null; onOpen: (key: string) => void }) {
  return <div className="rank-list">{rows.map((item, index) => { const key = `investment-${item.candidate.id}`; const expanded = open === key; return <div className={`rank-row${expanded ? " open" : ""}`} key={key}><div className="rank-main"><div className="rank-number">{index + 1}</div><div className="rank-player">{item.candidate.image_url && <img src={item.candidate.image_url} alt="" loading="lazy" />}<div className="rank-player-text"><strong className="rank-player-name">{item.candidate.common_name || item.candidate.name}</strong><span className="rank-player-sub">{item.candidate.rating} {item.candidate.position} · {item.candidate.version}</span></div></div><div className="rank-metric"><span className="rank-metric-label">desconto</span><span className="rank-metric-value"><Chip tone="gain">-{item.momentum_pct.toFixed(1)}%</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">agora</span><span className="rank-metric-value">{formatCoins(item.candidate.price.coins)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">média</span><span className="rank-metric-value">{formatCoins(item.implied_average)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">sinal</span><span className="rank-metric-value"><Chip tone={item.signal === "out-of-packs" ? "alert" : "flat"}>{item.signal === "out-of-packs" ? "fora dos packs" : "desconto"}</Chip></span></div><button className="rank-chevron" type="button" aria-expanded={expanded} onClick={() => onOpen(expanded ? "" : key)}><ExpandIcon expanded={expanded} /></button></div>{expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{item.candidate.common_name || item.candidate.name}</strong>{item.rationale?.length ? <ul>{item.rationale.map((reason, i) => <li key={i}>{reason}</li>)}</ul> : null}</div></div>}</div>; })}</div>;
}

function SellRows({ rows, open, onOpen }: { rows: SellCandidate[]; open: string | null; onOpen: (key: string) => void }) {
  return <div className="rank-list">{rows.map((item, index) => { const key = `sell-${item.player.id}`; const expanded = open === key; return <div className={`rank-row${expanded ? " open" : ""}`} key={key}><div className="rank-main"><div className="rank-number">{index + 1}</div><div className="rank-player"><div className="rank-player-text"><strong className="rank-player-name">{item.player.common_name || item.player.name}</strong><span className="rank-player-sub">{item.player.rating} · {item.player.position}</span></div></div><div className="rank-metric"><span className="rank-metric-label">ação</span><span className="rank-metric-value"><Chip tone={RECOMMENDATION_TONE[item.recommendation]}>{RECOMMENDATION_LABEL[item.recommendation]}</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">valor líquido</span><span className="rank-metric-value">{item.net_sell_value ? formatCoins(item.net_sell_value) : "—"}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">evolução</span><span className="rank-metric-value">{item.evo_gg_gain !== undefined ? formatSigned(item.evo_gg_gain) : "—"}</span></div><button className="rank-chevron" type="button" aria-expanded={expanded} onClick={() => onOpen(expanded ? "" : key)}><ExpandIcon expanded={expanded} /></button></div>{expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{RECOMMENDATION_LABEL[item.recommendation]}</strong>{item.rationale?.length ? <ul>{item.rationale.map((reason, i) => <li key={i}>{reason}</li>)}</ul> : null}</div></div>}</div>; })}</div>;
}

function FodderRows({ rows, open, onOpen }: { rows: FodderSignal[]; open: string | null; onOpen: (key: string) => void }) {
  return <div className="rank-list">{rows.map((item, index) => { const key = `fodder-${item.sbc_id}-${item.challenge}`; const expanded = open === key; return <div className={`rank-row${expanded ? " open" : ""}`} key={key}><div className="rank-main"><div className="rank-number">{index + 1}</div><div className="rank-player"><div className="rank-player-text"><strong className="rank-player-name">{item.sbc_name}</strong><span className="rank-player-sub">{item.challenge}</span></div></div><div className="rank-metric"><span className="rank-metric-label">fase</span><span className="rank-metric-value"><Chip tone={PHASE_TONE[item.phase]}>{PHASE_LABEL[item.phase]}</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">variação</span><span className="rank-metric-value">{formatSigned(item.cost_change_pct)}%</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">custo</span><span className="rank-metric-value">{formatCoins(item.cost_coins)}</span></div><button className="rank-chevron" type="button" aria-expanded={expanded} onClick={() => onOpen(expanded ? "" : key)}><ExpandIcon expanded={expanded} /></button></div>{expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{item.requirement}</strong>{item.rationale?.length ? <ul>{item.rationale.map((reason, i) => <li key={i}>{reason}</li>)}</ul> : null}</div></div>}</div>; })}</div>;
}

export default function Investimentos({ section = "investimentos" }: { section?: CapitalSection }) {
  const investmentCollection = useCollection<Investment>("/api/capital/investimentos", { defaultOrderBy: [{ field: "momentum_pct", desc: true }], pageSize: 20, enabled: section === "investimentos" });
  const sellCollection = useCollection<SellCandidate>("/api/capital/vendas", { defaultOrderBy: [{ field: "net_sell_value", desc: true }], pageSize: 20, enabled: section === "vendas" });
  const fodderCollection = useCollection<FodderSignal>("/api/capital/sbcs", { defaultOrderBy: [{ field: "cost_change_pct", desc: true }], pageSize: 20, enabled: section === "sbcs" });
  const active = section === "investimentos" ? investmentCollection : section === "vendas" ? sellCollection : fodderCollection;
  const [params, setParams] = useSearchParams();
  const filter = params.get("filter") || "todos";
  const sort = params.get("sort") || (section === "investimentos" ? "desconto" : section === "vendas" ? "valor" : "pressao");
  const open = params.get("open");
  const gate = asyncGate(active.loading, active.error, active.raw !== null, active.refetch);
  if (gate) return gate;
  if (!active.raw) return null;
  const page = investmentCollection.raw as InvestimentosCollection | null;
  const setParam = (key: string, value: string, defaultValue: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === defaultValue) next.delete(key); else next.set(key, value);
    if (key === "filter") {
      const field = section === "investimentos" ? "signal" : section === "vendas" ? "recommendation" : "phase";
      if (!value || value === defaultValue) next.delete("$filter"); else next.set("$filter", `${field} eq '${value.replaceAll("'", "''")}'`);
      next.delete("$skip");
    }
    if (key === "sort") {
      const order = section === "investimentos" ? (value === "valor" ? "candidate/price/coins desc" : "momentum_pct desc") : section === "vendas" ? (value === "recomendação" ? "recommendation asc" : "net_sell_value desc") : "cost_change_pct desc";
      next.set("$orderby", order); next.delete("$skip");
    }
    setParams(next);
  };
  const clear = () => { const next = new URLSearchParams(params); ["filter", "$filter", "$skip"].forEach((key) => next.delete(key)); setParams(next); };
  const investments = investmentCollection.rows;
  const sells = sellCollection.rows;
  const fodder = fodderCollection.rows;
  const title = section === "investimentos" ? "Investimentos" : section === "vendas" ? "Vendas do banco" : "Demanda de SBC";
  const meta = section === "investimentos" ? "sinais de preço e cartas em alta" : section === "vendas" ? "o que fazer com as cartas fora do XI" : "SBCs que estão puxando o mercado";
  const count = active.count;
  const facets = section === "investimentos" ? investmentCollection.facets.signal : section === "vendas" ? sellCollection.facets.recommendation : fodderCollection.facets.phase;
  return <div className="wrap"><PageHeader eyebrow="capital" title={title} meta={meta} /><div className="banner">Puramente consultivo: o bot nunca compra nem vende sozinho.</div>
    {section === "investimentos" && <section><h2>Cartas em alta · {count}</h2>{count === 0 ? <InvestmentsEmpty funnel={page?.["@eafc.funnel"] ?? { considered: 0, owned: 0, not_tradeable: 0, superseded_by_sibling: 0, below_min_momentum: 0, suggested: 0, min_momentum_pct: 0, best_rejected_pct: 0, best_rejected_name: "", has_best_rejected: false }} /> : <><RankingControls count={count} sort={sort} onSort={(value) => setParam("sort", value, "desconto")} options={[{ value: "desconto", label: "maior desconto" }, { value: "valor", label: "maior valor" }]} hasFilters={filter !== "todos"} onClear={clear}><FilterSelect label="sinal" value={filter} onChange={(value) => setParam("filter", value, "todos")} options={[{ value: "todos", label: "todos" }, ...(facets ?? []).map((item) => ({ value: item.value, label: `${item.value} (${item.count})` }))]} /></RankingControls><InvestmentRows rows={investments} open={open} onOpen={(key) => setParam("open", key, "")} /><Pagination page={investmentCollection.page} pages={investmentCollection.pages} onPage={investmentCollection.setPage} /></>}</section>}
    {section === "vendas" && <section><h2>Vale vender do banco? · {count}</h2>{count === 0 ? <EmptyState message="Nada no banco além do XI titular hoje." /> : <><RankingControls count={count} sort={sort} onSort={(value) => setParam("sort", value, "valor")} options={[{ value: "valor", label: "maior valor líquido" }, { value: "recomendação", label: "recomendação" }]} hasFilters={filter !== "todos"} onClear={clear}><FilterSelect label="ação" value={filter} onChange={(value) => setParam("filter", value, "todos")} options={[{ value: "todos", label: "todas" }, ...(facets ?? []).map((item) => ({ value: item.value, label: `${item.value} (${item.count})` }))]} /></RankingControls><SellRows rows={sells} open={open} onOpen={(key) => setParam("open", key, "")} /><Pagination page={sellCollection.page} pages={sellCollection.pages} onPage={sellCollection.setPage} /></>}</section>}
    {section === "sbcs" && <section><h2>SBCs puxando demanda · {count}</h2>{count === 0 ? <EmptyState message="Nenhum SBC com custo de solução resolvido hoje." /> : <><RankingControls count={count} sort={sort} onSort={(value) => setParam("sort", value, "pressao")} options={[{ value: "pressao", label: "maior pressão" }, { value: "custo", label: "maior alta de custo" }]} hasFilters={filter !== "todos"} onClear={clear}><FilterSelect label="fase" value={filter} onChange={(value) => setParam("filter", value, "todos")} options={[{ value: "todos", label: "todas" }, ...(facets ?? []).map((item) => ({ value: item.value, label: `${item.value} (${item.count})` }))]} /></RankingControls><FodderRows rows={fodder} open={open} onOpen={(key) => setParam("open", key, "")} /><Pagination page={fodderCollection.page} pages={fodderCollection.pages} onPage={fodderCollection.setPage} /></>}</section>}
  </div>;
}
