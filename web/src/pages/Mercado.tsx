import { useMemo } from "react";
import { fetchMercado } from "../api";
import { useSearchParams } from "react-router-dom";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatSigned } from "../format";
import { useData } from "../useData";
import type { UpgradeFunnel } from "../types";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import ExpandIcon from "../components/ExpandIcon";
import "../shared.css";
import "./Mercado.css";

// MercadoEmpty explica uma lista vazia carta a carta, em vez de repetir o
// mito de que market.extra_budget resolve — ele NUNCA filtra a lista (ver
// analyze.UpgradeFunnel do lado Go): DefaultUpgradeOptions liga
// IncludeUnaffordable, então uma carta fora do bolso continua aparecendo, só
// marcada. "0 sugestões" sempre significa "nada passou do ganho mínimo".
function MercadoEmpty({ funnel }: { funnel: UpgradeFunnel }) {
  if (funnel.considered === 0) {
    // Snapshot gravado antes deste campo existir — cai no texto genérico até
    // a próxima coleta regravar o snapshot com o funil preenchido.
    return (
      <EmptyState
        message="Nenhuma oportunidade dentro do ganho mínimo hoje."
        hint="Rode uma coleta nova (botão de atualizar, ou POST /api/job) para ver o motivo carta a carta."
      />
    );
  }

  return (
    <div className="empty" style={{ textAlign: "left" }}>
      <div>Nenhuma troca passou do ganho mínimo de +{funnel.min_gain.toFixed(1)} hoje.</div>
      <div className="hint">
        Das {funnel.considered} cartas do mercado: {funnel.owned} você já tem · {funnel.sbc_only} são exclusivas de
        SBC · {funnel.unpriced} vieram sem cotação · {funnel.out_of_position} não jogam em nenhum titular seu ·{" "}
        {funnel.below_min_gain} jogariam mas não bateram o ganho mínimo.
      </div>
      {funnel.has_best && (
        <div className="hint">
          {funnel.best_gain > 0 ? (
            <>
              A mais perto foi <strong>{funnel.best_name}</strong> no {funnel.best_slot},{" "}
              {formatSigned(funnel.best_gain)} (mínimo exigido: +{funnel.min_gain.toFixed(1)}).
            </>
          ) : (
            <>
              Nenhum candidato ficou acima de um titular seu (melhor caso: {funnel.best_name} no {funnel.best_slot},{" "}
              {formatSigned(funnel.best_gain)}).
            </>
          )}
        </div>
      )}
      <div className="hint">
        Vale baixar <code>report.min_gain</code>, ligar <code>report.allow_unpriced</code> ou{" "}
        <code>report.allow_out_of_position</code> na configuração — <code>market.extra_budget</code> não muda essa
        lista, só marca o que está fora do bolso.
      </div>
    </div>
  );
}

// Mercado é "/mercado": os upgrades já ordenados por ganho por moeda gasta
// (ver analyze.FindUpgrades) — a resposta chega pronta, esta tela não
// reordena nada. Um card fora do orçamento continua visível, só esmaecido
// — é uma meta pra hoje, não um erro (ver analyze.Upgrade.Affordable).
export default function Mercado() {
  const { data, error, loading, refetch } = useData(fetchMercado, []);
  const [params, setParams] = useSearchParams();
  const sort = params.get("sort") || "efficiency";
  const position = params.get("position") || "todas";
  const budget = params.get("budget") || "todas";
  const open = params.get("open");
  const upgrades = data?.upgrades ?? [];
  const ranked = useMemo(() => {
    const visible = upgrades.filter((u) => (position === "todas" || u.slot === position) && (budget === "todas" || (budget === "cabem" ? u.affordable : !u.affordable)));
    return visible.slice().sort((a, b) => {
      if (sort === "gain") return b.gain - a.gain;
      if (sort === "cost") return a.net_cost - b.net_cost;
      return b.efficiency - a.efficiency;
    });
  }, [upgrades, position, budget, sort]);
  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const series = data.price_series ?? {};
  const setParam = (key: string, value: string, defaultValue: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === defaultValue) next.delete(key); else next.set(key, value);
    setParams(next);
  };
  const hasFilters = position !== "todas" || budget !== "todas";

  return (
    <div className="wrap">
      <PageHeader eyebrow="aquisição" title="Oportunidades" meta="Escolha a troca com mais impacto pelo menor custo." />

      {upgrades.length === 0 ? (
        <MercadoEmpty funnel={data.funnel} />
      ) : (
        <>
          <RankingControls count={ranked.length} sort={sort} onSort={(v) => setParam("sort", v, "efficiency")} options={[{ value: "efficiency", label: "eficiência" }, { value: "gain", label: "ganho bruto" }, { value: "cost", label: "menor custo" }]} hasFilters={hasFilters} onClear={() => { const next = new URLSearchParams(params); next.delete("position"); next.delete("budget"); setParams(next); }}>
            <FilterSelect label="posição" value={position} onChange={(v) => setParam("position", v, "todas")} options={[{ value: "todas", label: "todas" }, ...Array.from(new Set(upgrades.map((u) => u.slot))).sort().map((p) => ({ value: p, label: p }))]} />
            <FilterSelect label="orçamento" value={budget} onChange={(v) => setParam("budget", v, "todas")} options={[{ value: "todas", label: "todos" }, { value: "cabem", label: "cabem agora" }, { value: "meta", label: "fora do bolso" }]} />
          </RankingControls>
          <div className="rank-list">
          {ranked.map((u, i) => {
            const pts = (series[String(u.candidate.id)] ?? [])
              .slice()
              .sort((a, b) => a.observed_at.localeCompare(b.observed_at))
              .map((pt) => ({ label: pt.observed_at, value: pt.coins }));
            const expanded = open === `${u.slot}-${u.candidate.id}`;
            return <div className={`rank-row${expanded ? " open" : ""}${u.affordable ? "" : " unaffordable"}`} key={`${u.slot}-${u.candidate.id}`}>
              <div className="rank-main">
                <div className="rank-number">{i + 1}</div>
                <div className="rank-player">{u.candidate.image_url && <img src={u.candidate.image_url} alt="" loading="lazy" />}<div className="rank-player-text"><strong className="rank-player-name">{u.candidate.common_name || u.candidate.name}</strong><span className="rank-player-sub">{u.current.common_name || u.current.name} → {u.slot}</span></div></div>
                <div className="rank-metric"><span className="rank-metric-label">ganho</span><span className="rank-metric-value"><Chip tone="gain">{formatSigned(u.gain)}</Chip></span></div>
                <div className="rank-metric"><span className="rank-metric-label">custo líquido</span><span className="rank-metric-value">{u.unpriced ? "—" : formatCoins(u.net_cost)}</span></div>
                <div className="rank-metric optional-metric"><span className="rank-metric-label">eficiência</span><span className="rank-metric-value">{u.efficiency.toFixed(2)}</span></div>
                <div className="rank-metric optional-metric"><span className="rank-metric-label">situação</span><span className="rank-metric-value"><Chip tone={u.affordable ? "flat" : "alert"}>{u.affordable ? "cabe" : "meta"}</Chip></span></div>
                <button className="rank-chevron" type="button" aria-expanded={expanded} aria-label={expanded ? "recolher detalhes" : "abrir detalhes"} onClick={() => setParam("open", expanded ? "" : `${u.slot}-${u.candidate.id}`, "")}><ExpandIcon expanded={expanded} /></button>
              </div>
              {expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{u.current.common_name || u.current.name} → {u.candidate.common_name || u.candidate.name}</strong><ul>{(u.rationale ?? []).map((r, j) => <li key={j}>{r}</li>)}{u.profit > 0 && <li>Lucro potencial: {formatCoins(u.profit)}</li>}</ul></div><div className="rank-detail-side">{pts.length > 1 ? <TrendChart data={pts} compact height={54} /> : <span className="chart-empty-inline">histórico de preço insuficiente</span>}</div></div>}
            </div>;
          })}
          {ranked.length === 0 && <EmptyState message="Nenhuma oportunidade combina com estes filtros." hint="Limpe os filtros para voltar ao ranking completo." />}
          </div>
        </>
      )}
    </div>
  );
}
