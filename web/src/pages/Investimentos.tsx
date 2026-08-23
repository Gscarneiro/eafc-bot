import { useMemo } from "react";
import { fetchInvestimentos } from "../api";
import { useSearchParams } from "react-router-dom";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatCoins, formatSigned } from "../format";
import { useData } from "../useData";
import type { FodderPhase, InvestmentFunnel, SellRecommendation } from "../types";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import ExpandIcon from "../components/ExpandIcon";
import "../shared.css";
import "./Investimentos.css";

// InvestmentsEmpty explica uma lista vazia carta a carta, mesmo padrão de
// MercadoEmpty (ver Mercado.tsx) — "0 sinais" sempre significa "nada
// passou do desconto mínimo", nunca "o bot não olhou".
function InvestmentsEmpty({ funnel }: { funnel: InvestmentFunnel }) {
  if (funnel.considered === 0) {
    return (
      <EmptyState
        message="O ciclo de coleta rápido (momentum) ainda não rodou."
        hint="Ele roda sozinho junto com o serve — veja serve.fast_refresh_minutes na configuração."
      />
    );
  }
  return (
    <div className="empty" style={{ textAlign: "left" }}>
      <div>Nenhuma carta passou do desconto mínimo de {funnel.min_momentum_pct.toFixed(1)}% hoje.</div>
      <div className="hint">
        Das {funnel.considered} cartas do momentum: {funnel.owned} você já tem · {funnel.not_tradeable} não são
        compráveis (SBC ou extinta) · {funnel.superseded_by_sibling} têm uma versão melhor do mesmo jogador na lista ·{" "}
        {funnel.below_min_momentum} caíram menos que o piso.
      </div>
      {funnel.has_best_rejected && (
        <div className="hint">
          A mais perto foi <strong>{funnel.best_rejected_name}</strong>, -{funnel.best_rejected_pct.toFixed(1)}%.
        </div>
      )}
    </div>
  );
}

const RECOMMENDATION_LABEL: Record<SellRecommendation, string> = {
  vender: "vender",
  segurar_potencial: "segurar: potencial",
  promover: "promover",
  nao_vendavel: "não vendável",
};

const RECOMMENDATION_TONE: Record<SellRecommendation, ChipTone> = {
  vender: "cost",
  segurar_potencial: "gain",
  promover: "gain",
  nao_vendavel: "flat",
};

const PHASE_LABEL: Record<FodderPhase, string> = {
  recente: "recente",
  pico: "pico — não compre",
  esfriando: "esfriando",
  estavel: "estável",
  esvaziar: "esvaziar",
  expirado: "expirado",
};

const PHASE_TONE: Record<FodderPhase, ChipTone> = {
  recente: "flat",
  pico: "alert",
  esfriando: "gain",
  estavel: "flat",
  esvaziar: "alert",
  expirado: "flat",
};

// Investimentos é "/investimentos": o agente de trading — cartas do
// mercado ganhando valor, o que fazer com o banco de reservas, e demanda
// de fodder de SBC esquentando (ver analyze.FindInvestments /
// FindSellCandidates / FindFodderDemand no lado Go). Puramente
// consultivo: o bot nunca compra nem vende sozinho, só aponta.
export type CapitalSection = "investimentos" | "vendas" | "sbcs";

export default function Investimentos({ section = "investimentos" }: { section?: CapitalSection }) {
  const { data, error, loading, refetch } = useData(fetchInvestimentos, []);
  const [params, setParams] = useSearchParams();
  const sort = params.get("sort") || (section === "investimentos" ? "desconto" : section === "vendas" ? "valor" : "pressao");
  const filter = params.get("filter") || "todos";
  const open = params.get("open");
  const investments = data?.investments ?? [];
  const sellCandidates = data?.sell_candidates ?? [];
  const fodderDemand = data?.fodder_demand ?? [];
  const rankedInvestments = useMemo(() => investments.filter((i) => filter === "todos" || i.signal === filter).slice().sort((a, b) => sort === "valor" ? b.candidate.price.coins - a.candidate.price.coins : a.momentum_pct - b.momentum_pct), [investments, filter, sort]);
  const rankedSells = useMemo(() => sellCandidates.filter((c) => filter === "todos" || c.recommendation === filter).slice().sort((a, b) => sort === "recomendação" ? RECOMMENDATION_LABEL[a.recommendation].localeCompare(RECOMMENDATION_LABEL[b.recommendation]) : (b.net_sell_value ?? 0) - (a.net_sell_value ?? 0)), [sellCandidates, filter, sort]);
  const rankedFodder = useMemo(() => fodderDemand.filter((f) => filter === "todos" || f.phase === filter).slice().sort((a, b) => sort === "custo" ? a.cost_change_pct - b.cost_change_pct : b.cost_change_pct - a.cost_change_pct), [fodderDemand, filter, sort]);
  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const setParam = (key: string, value: string, defaultValue: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === defaultValue) next.delete(key); else next.set(key, value);
    setParams(next);
  };
  const hasFilter = filter !== "todos";

  return (
    <div className="wrap">
      <PageHeader
        eyebrow="capital"
        title={section === "investimentos" ? "Investimentos" : section === "vendas" ? "Vendas do banco" : "Demanda de SBC"}
        meta={section === "investimentos" ? "sinais de preço e cartas em alta" : section === "vendas" ? "o que fazer com as cartas fora do XI" : "SBCs que estão puxando o mercado"}
      />

      <div className="banner">
        Moedas deste ciclo <strong>não passam pro próximo jogo</strong> — um flip só compensa se der tempo de vender
        antes da virada. Puramente consultivo: o bot nunca compra nem vende sozinho.
      </div>

      {section === "investimentos" && <section>
        <h2>Cartas em alta · {investments.length}</h2>
        {investments.length === 0 ? (
          <InvestmentsEmpty funnel={data.investment_funnel} />
        ) : (
          <>
            <RankingControls count={rankedInvestments.length} sort={sort} onSort={(v) => setParam("sort", v, "desconto")} options={[{ value: "desconto", label: "maior desconto" }, { value: "valor", label: "maior valor" }]} hasFilters={hasFilter} onClear={() => { const next = new URLSearchParams(params); next.delete("filter"); setParams(next); }}>
              <FilterSelect label="sinal" value={filter} onChange={(v) => setParam("filter", v, "todos")} options={[{ value: "todos", label: "todos" }, { value: "desconto", label: "desconto" }, { value: "out-of-packs", label: "fora dos packs" }]} />
            </RankingControls>
            <div className="rank-list">
              {rankedInvestments.map((inv, i) => {
                const key = `investment-${inv.candidate.id}`;
                const expanded = open === key;
                return <div className={`rank-row${expanded ? " open" : ""}`} key={key}>
                  <div className="rank-main"><div className="rank-number">{i + 1}</div><div className="rank-player">{inv.candidate.image_url && <img src={inv.candidate.image_url} alt="" loading="lazy" />}<div className="rank-player-text"><strong className="rank-player-name">{inv.candidate.common_name || inv.candidate.name}</strong><span className="rank-player-sub">{inv.candidate.rating} {inv.candidate.position} · {inv.candidate.version}</span></div></div><div className="rank-metric"><span className="rank-metric-label">desconto</span><span className="rank-metric-value"><Chip tone="gain">-{inv.momentum_pct.toFixed(1)}%</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">agora</span><span className="rank-metric-value">{formatCoins(inv.candidate.price.coins)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">média</span><span className="rank-metric-value">{formatCoins(inv.implied_average)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">sinal</span><span className="rank-metric-value"><Chip tone={inv.signal === "out-of-packs" ? "alert" : "flat"}>{inv.signal === "out-of-packs" ? "fora dos packs" : "desconto"}</Chip></span></div><button className="rank-chevron" type="button" aria-expanded={expanded} aria-label={expanded ? "recolher detalhes" : "abrir detalhes"} onClick={() => setParam("open", expanded ? "" : key, "")}><ExpandIcon expanded={expanded} /></button></div>
                  {expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{inv.candidate.common_name || inv.candidate.name}</strong>{(inv.rationale ?? []).length > 0 && <ul>{inv.rationale!.map((r, j) => <li key={j}>{r}</li>)}</ul>}</div></div>}
                </div>;
              })}
            </div>
          </>
        )}
      </section>}

      {section === "vendas" && <section>
        <h2>Vale vender do banco? · {sellCandidates.length}</h2>
        {sellCandidates.length === 0 ? (
          <EmptyState message="Nada no banco além do XI titular hoje." />
        ) : (
          <>
            <RankingControls count={rankedSells.length} sort={sort} onSort={(v) => setParam("sort", v, "valor")} options={[{ value: "valor", label: "maior valor líquido" }, { value: "recomendação", label: "recomendação" }]} hasFilters={hasFilter} onClear={() => { const next = new URLSearchParams(params); next.delete("filter"); setParams(next); }}>
              <FilterSelect label="ação" value={filter} onChange={(v) => setParam("filter", v, "todos")} options={[{ value: "todos", label: "todas" }, ...Object.entries(RECOMMENDATION_LABEL).map(([value, label]) => ({ value, label }))]} />
            </RankingControls>
            <div className="rank-list">
              {rankedSells.map((c, i) => {
                const key = `sell-${c.player.id}`;
                const expanded = open === key;
                return <div className={`rank-row${expanded ? " open" : ""}`} key={key}>
                  <div className="rank-main"><div className="rank-number">{i + 1}</div><div className="rank-player"><div className="rank-player-text"><strong className="rank-player-name">{c.player.common_name || c.player.name}</strong><span className="rank-player-sub">{c.player.rating} · {c.player.position}</span></div></div><div className="rank-metric"><span className="rank-metric-label">ação</span><span className="rank-metric-value"><Chip tone={RECOMMENDATION_TONE[c.recommendation]}>{RECOMMENDATION_LABEL[c.recommendation]}</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">valor líquido</span><span className="rank-metric-value">{c.net_sell_value ? formatCoins(c.net_sell_value) : "—"}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">evolução</span><span className="rank-metric-value">{c.evo_gg_gain !== undefined ? formatSigned(c.evo_gg_gain) : "—"}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">custo evo</span><span className="rank-metric-value">{c.evo_cost ? formatCoins(c.evo_cost) : "—"}</span></div><button className="rank-chevron" type="button" aria-expanded={expanded} aria-label={expanded ? "recolher detalhes" : "abrir detalhes"} onClick={() => setParam("open", expanded ? "" : key, "")}><ExpandIcon expanded={expanded} /></button></div>
                  {expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{RECOMMENDATION_LABEL[c.recommendation]}</strong>{(c.rationale ?? []).length > 0 && <ul>{c.rationale!.map((r, j) => <li key={j}>{r}</li>)}</ul>}</div></div>}
                </div>;
              })}
            </div>
          </>
        )}
      </section>}

      {section === "sbcs" && <section>
        <h2>SBCs puxando demanda · {fodderDemand.length}</h2>
        {fodderDemand.length === 0 ? (
          <EmptyState message="Nenhum SBC com custo de solução resolvido hoje." />
        ) : (
          <>
            <RankingControls count={rankedFodder.length} sort={sort} onSort={(v) => setParam("sort", v, "pressao")} options={[{ value: "pressao", label: "maior pressão" }, { value: "custo", label: "maior alta de custo" }]} hasFilters={hasFilter} onClear={() => { const next = new URLSearchParams(params); next.delete("filter"); setParams(next); }}>
              <FilterSelect label="fase" value={filter} onChange={(v) => setParam("filter", v, "todos")} options={[{ value: "todos", label: "todas" }, ...Object.entries(PHASE_LABEL).map(([value, label]) => ({ value, label }))]} />
            </RankingControls>
            <div className="rank-list">
              {rankedFodder.map((f, i) => {
                const key = `fodder-${f.sbc_id}-${f.challenge}`;
                const expanded = open === key;
                return <div className={`rank-row${expanded ? " open" : ""}`} key={key}>
                  <div className="rank-main"><div className="rank-number">{i + 1}</div><div className="rank-player"><div className="rank-player-text"><strong className="rank-player-name">{f.sbc_name}</strong><span className="rank-player-sub">{f.challenge}</span></div></div><div className="rank-metric"><span className="rank-metric-label">fase</span><span className="rank-metric-value"><Chip tone={PHASE_TONE[f.phase]}>{PHASE_LABEL[f.phase]}</Chip></span></div><div className="rank-metric"><span className="rank-metric-label">variação</span><span className="rank-metric-value">{formatSigned(f.cost_change_pct)}%</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">custo</span><span className="rank-metric-value">{formatCoins(f.cost_coins)}</span></div><div className="rank-metric optional-metric"><span className="rank-metric-label">pool</span><span className="rank-metric-value">{f.pool_size >= 0 ? f.pool_size : "—"}</span></div><button className="rank-chevron" type="button" aria-expanded={expanded} aria-label={expanded ? "recolher detalhes" : "abrir detalhes"} onClick={() => setParam("open", expanded ? "" : key, "")}><ExpandIcon expanded={expanded} /></button></div>
                  {expanded && <div className="rank-detail"><div className="rank-detail-copy"><strong>{f.requirement}</strong>{f.repeatable && <p>Este SBC é repetível.</p>}{(f.rationale ?? []).length > 0 && <ul>{f.rationale!.map((r, j) => <li key={j}>{r}</li>)}</ul>}</div></div>}
                </div>;
              })}
            </div>
          </>
        )}
      </section>}
    </div>
  );
}
