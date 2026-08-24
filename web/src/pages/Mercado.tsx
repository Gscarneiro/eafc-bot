import { useSearchParams } from "react-router-dom";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import ExpandIcon from "../components/ExpandIcon";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatSigned } from "../format";
import { formatFilter, type Filter } from "../odata";
import { useCollection } from "../useCollection";
import type { MercadoCollection, Upgrade, UpgradeFunnel } from "../types";
import "../shared.css";
import "./Mercado.css";

function emptyFunnel(): UpgradeFunnel {
  return { considered: 0, owned: 0, sbc_only: 0, unpriced: 0, out_of_position: 0, below_min_gain: 0, suggested: 0, min_gain: 0, best_gain: 0, best_slot: "", best_name: "", has_best: false };
}

function MercadoEmpty({ funnel }: { funnel: UpgradeFunnel }) {
  if (funnel.considered === 0) return <EmptyState message="Nenhuma oportunidade encontrada hoje." hint="Rode uma coleta nova para atualizar o mercado." />;
  return <div className="empty" style={{ textAlign: "left" }}>
    <div>Nenhuma troca passou do ganho mínimo de +{funnel.min_gain.toFixed(1)} hoje.</div>
    <div className="hint">Das {funnel.considered} cartas: {funnel.owned} você já tem · {funnel.sbc_only} são exclusivas de SBC · {funnel.unpriced} vieram sem cotação · {funnel.out_of_position} não cabem em um titular · {funnel.below_min_gain} ficaram abaixo do ganho mínimo.</div>
    {funnel.has_best && <div className="hint">A mais perto foi <strong>{funnel.best_name}</strong> no {funnel.best_slot}, {formatSigned(funnel.best_gain)}.</div>}
  </div>;
}

export default function Mercado() {
  const collection = useCollection<Upgrade>("/api/mercado", { defaultOrderBy: [{ field: "efficiency", desc: true }], pageSize: 20 });
  const [params, setParams] = useSearchParams();
  const sort = params.get("sort") || "efficiency";
  const position = params.get("position") || "todas";
  const budget = params.get("budget") || "todas";
  const open = params.get("open");
  const data = collection.raw as MercadoCollection | null;
  const gate = asyncGate(collection.loading, collection.error, data !== null, collection.refetch);
  if (gate) return gate;
  if (!data) return null;

  const upgrades = collection.rows;
  const funnel = data["@eafc.funnel"] ?? emptyFunnel();
  const series = data["@eafc.price_series"] ?? {};
  const buildFilter = (nextPosition: string, nextBudget: string): Filter | undefined => {
    const filters: Filter[] = [];
    if (nextPosition !== "todas") filters.push({ kind: "compare", field: "slot", op: "eq", value: nextPosition });
    if (nextBudget !== "todas") filters.push({ kind: "compare", field: "affordable", op: "eq", value: nextBudget === "cabem" });
    return filters.reduce<Filter | undefined>((result, item) => result ? { kind: "and", left: result, right: item } : item, undefined);
  };
  const setParam = (key: string, value: string, defaultValue: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === defaultValue) next.delete(key); else next.set(key, value);
    if (key === "position" || key === "budget") {
      const filter = buildFilter(key === "position" ? value || "todas" : position, key === "budget" ? value || "todas" : budget);
      if (filter) next.set("$filter", formatFilter(filter)); else next.delete("$filter");
      next.delete("$skip");
    }
    if (key === "sort") {
      const field = value === "gain" ? "gain" : value === "cost" ? "net_cost" : "efficiency";
      next.set("$orderby", `${field} ${value === "cost" ? "asc" : "desc"}`);
      next.delete("$skip");
    }
    setParams(next);
  };
  const clear = () => {
    const next = new URLSearchParams(params);
    ["position", "budget", "$filter", "$skip"].forEach((key) => next.delete(key));
    setParams(next);
  };

  return <div className="wrap">
    <PageHeader eyebrow="aquisição" title="Oportunidades" meta="Escolha a troca com mais impacto pelo menor custo." />
    {collection.count === 0 ? <MercadoEmpty funnel={funnel} /> : <>
      <RankingControls count={collection.count} sort={sort} onSort={(value) => setParam("sort", value, "efficiency")} options={[{ value: "efficiency", label: "eficiência" }, { value: "gain", label: "ganho bruto" }, { value: "cost", label: "menor custo" }]} hasFilters={position !== "todas" || budget !== "todas"} onClear={clear}>
        <FilterSelect label="posição" value={position} onChange={(value) => setParam("position", value, "todas")} options={[{ value: "todas", label: "todas" }, ...(collection.facets.slot ?? []).map((facet) => ({ value: facet.value, label: `${facet.value} (${facet.count})` }))]} />
        <FilterSelect label="orçamento" value={budget} onChange={(value) => setParam("budget", value, "todas")} options={[{ value: "todas", label: "todos" }, { value: "cabem", label: "cabem agora" }, { value: "meta", label: "fora do bolso" }]} />
      </RankingControls>
      <div className="rank-list">{upgrades.map((upgrade, index) => {
        const key = `${upgrade.slot}-${upgrade.candidate.id}`;
        const expanded = open === key;
        const points = (series[String(upgrade.candidate.id)] ?? []).slice().sort((a, b) => a.observed_at.localeCompare(b.observed_at)).map((point) => ({ label: point.observed_at, value: point.coins }));
        return <div className={`rank-row${expanded ? " open" : ""}${upgrade.affordable ? "" : " unaffordable"}`} key={key}>
          <div className="rank-main"><div className="rank-number">{index + 1 + (collection.page - 1) * 20}</div><div className="rank-player">{upgrade.candidate.image_url && <img src={upgrade.candidate.image_url} alt="" loading="lazy" />}<div className="rank-player-text"><strong className="rank-player-name">{upgrade.candidate.common_name || upgrade.candidate.name}</strong><span className="rank-player-sub">{upgrade.current.common_name || upgrade.current.name} → {upgrade.slot}</span></div></div><div className="rank-metric"><span className="rank-metric-label">ganho</span><span className="rank-metric-value"><Chip tone="gain">{formatSigned(upgrade.gain)}</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">custo líquido</span><span className="rank-metric-value">{upgrade.unpriced ? "—" : formatCoins(upgrade.net_cost)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">eficiência</span><span className="rank-metric-value">{upgrade.efficiency.toFixed(2)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">situação</span><span className="rank-metric-value"><Chip tone={upgrade.affordable ? "flat" : "alert"}>{upgrade.affordable ? "cabe" : "meta"}</Chip></span></div><button className="rank-chevron" type="button" aria-expanded={expanded} aria-label={expanded ? "recolher detalhes" : "abrir detalhes"} onClick={() => setParam("open", expanded ? "" : key, "")}><ExpandIcon expanded={expanded} /></button></div>
          {expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{upgrade.current.common_name || upgrade.current.name} → {upgrade.candidate.common_name || upgrade.candidate.name}</strong><ul>{(upgrade.rationale ?? []).map((reason, reasonIndex) => <li key={reasonIndex}>{reason}</li>)}{upgrade.profit > 0 && <li>Lucro potencial: {formatCoins(upgrade.profit)}</li>}</ul></div><div className="rank-detail-side">{points.length > 1 ? <TrendChart data={points} compact height={54} /> : <span className="chart-empty-inline">histórico de preço insuficiente</span>}</div></div>}
        </div>;
      })}</div>
      <Pagination page={collection.page} pages={collection.pages} onPage={collection.setPage} />
    </>}
  </div>;
}
