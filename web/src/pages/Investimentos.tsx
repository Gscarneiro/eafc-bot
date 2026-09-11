import { useSearchParams } from "react-router-dom";
import { fetchPosicoes, fetchExtrato, fetchStatus } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import ExpandIcon from "../components/ExpandIcon";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import RankingControls, { FilterSelect } from "../components/RankingControls";
import { formatCoins, formatDateTime, formatSigned, formatSignedCoins } from "../format";
import { useCollection } from "../useCollection";
import { useData } from "../useData";
import type { FodderPhase, FodderSignal, Investment, InvestmentFunnel, InvestimentosCollection, SellCandidate, SellRecommendation } from "../types";
import "../shared.css";
import "./Investimentos.css";

type CapitalTab = "investimentos" | "vendas" | "sbcs";

const RECOMMENDATION_LABEL: Record<SellRecommendation, string> = { vender: "vender", segurar_potencial: "segurar: potencial", aguardar_verificacao: "aguardar verificação", promover: "promover", nao_vendavel: "não vendável" };
const RECOMMENDATION_TONE: Record<SellRecommendation, ChipTone> = { vender: "cost", segurar_potencial: "gain", aguardar_verificacao: "alert", promover: "gain", nao_vendavel: "flat" };
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

// SaldoPanel é "Onde estão suas moedas" — a barra de alocação de
// domain.Capital (disponível / reserva protegida / comprometido). Available
// pode ser negativo DE PROPÓSITO (comprometeu mais do que tem à mão);
// renderiza o déficit em vez de recortar em zero.
function SaldoPanel() {
  const { data } = useData(fetchStatus, []);
  if (!data) return null;
  const c = data.capital;
  const total = Math.max(c.cash + c.extra_budget, 1);
  const pct = (v: number) => Math.max(0, Math.min(100, (v / total) * 100));
  return (
    <section className="panel saldo-panel">
      <div className="panel-head"><span>Saldo <span className="panel-head-sub">/ Coins</span></span></div>
      <div className="panel-body">
        <div className="saldo-grid">
          <div><span>Caixa (fonte real)</span><strong className="coin">{formatCoins(c.cash)}</strong></div>
          <div><span>Extra (manual)</span><strong className="coin">{formatCoins(c.extra_budget)}</strong></div>
          <div><span>Reserva protegida</span><strong>{formatCoins(c.reserve)}</strong></div>
          <div><span>Comprometido</span><strong>{formatCoins(c.committed)}</strong></div>
          <div><span>Disponível</span><strong className={c.available >= 0 ? "up" : "down"}>{formatCoins(c.available)}</strong></div>
        </div>
        <div className="saldo-bar">
          <span className="saldo-bar-cash" style={{ width: `${pct(c.cash)}%` }} title={`caixa ${formatCoins(c.cash)}`} />
          <span className="saldo-bar-extra" style={{ width: `${pct(c.extra_budget)}%` }} title={`extra manual ${formatCoins(c.extra_budget)}`} />
        </div>
        <p className="hint">Caixa vem da coleta; "extra" é o que você digitou em Configurações (a Community API da EA não publica saldo de moedas).</p>
      </div>
    </section>
  );
}

// PosicoesPanel é "Posições abertas" — só o que o bot pode honestamente
// afirmar: custo de aquisição PUBLICADO pelo fut.gg (ClubStats.PurchasedFor)
// contra o valor líquido de hoje. Nunca P&L realizado — o bot não lê
// histórico de transação da conta EA.
function PosicoesPanel() {
  const { data } = useData(fetchPosicoes, []);
  const posicoes = data?.posicoes ?? [];
  const planejado = data?.planejado ?? [];
  return (
    <section className="panel posicoes-panel">
      <div className="panel-head">
        <span>Posições abertas <span className="panel-head-sub">/ custo publicado x valor hoje</span></span>
        {data && <span className="panel-head-meta">comprometido {formatCoins(data.committed)}</span>}
      </div>
      <div className="panel-body">
        {posicoes.length === 0 && planejado.length === 0 ? (
          <p className="hint">O fut.gg não publicou custo de aquisição pra nenhuma carta do clube, e nada está planejado no ledger.</p>
        ) : (
          <>
            {posicoes.length > 0 && (
              <div className="tablewrap">
                <table>
                  <thead><tr><th>Carta</th><th className="num">Custo</th><th className="num">Vale hoje</th><th className="num">Não realizado</th></tr></thead>
                  <tbody>
                    {posicoes.map((p) => (
                      <tr key={p.player.club_item_id || p.player.id}>
                        <td className="namecell">{p.player.common_name || p.player.name}</td>
                        <td className="num coin">{formatCoins(p.purchased_for)}</td>
                        <td className="num coin">{formatCoins(p.current_value)}</td>
                        <td className={`num ${p.unrealized_pnl >= 0 ? "up" : "down"}`}>{formatSignedCoins(p.unrealized_pnl)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {planejado.length > 0 && (
              <ul className="record-list">
                {planejado.map((e) => (
                  <li key={e.id}><Chip tone="alert">planejado</Chip><strong>{formatCoins(e.gross_coins)}</strong><span>{e.kind}{e.note ? ` · ${e.note}` : ""}</span></li>
                ))}
              </ul>
            )}
          </>
        )}
      </div>
    </section>
  );
}

function ExtratoPanel() {
  const { data } = useData(() => fetchExtrato("$top=8"), []);
  const entries = data?.value ?? [];
  const resultado7d = data?.["@eafc.summary_7d"]?.net_cash ?? 0;
  const total = data?.["@eafc.summary"];
  return (
    <section className="panel extrato-panel">
      <div className="panel-head">
        <span>Extrato <span className="panel-head-sub">/ Ledger</span></span>
        <span className={`panel-head-meta ${resultado7d >= 0 ? "up" : "down"}`}>7d: {resultado7d >= 0 ? "+" : ""}{formatCoins(resultado7d)}</span>
      </div>
      <div className="panel-body">
        {total && (
          <div className="extrato-summary">
            <div><span>Gasto total</span><strong className="down">{formatCoins(total.spent)}</strong></div>
            <div><span>Arrecadado líq.</span><strong className="up">{formatCoins(total.raised_net)}</strong></div>
            <div><span>P&amp;L total</span><strong className={total.pnl >= 0 ? "up" : "down"}>{formatSignedCoins(total.pnl)}</strong></div>
          </div>
        )}
        {entries.length === 0 ? <p className="hint">Nenhum lançamento neste ciclo.</p> : (
          <ul className="record-list">
            {entries.map((e) => (
              <li key={e.id}>
                <Chip tone={e.status === "planejado" ? "alert" : e.kind === "venda" ? "gain" : "flat"}>{e.kind}</Chip>
                {/* ajuste é o único tipo com gross_coins já assinado (Validate
                    exige positivo nos demais) — prefixar "−" na frente do
                    formatCoins dele duplicaria o sinal num ajuste negativo. */}
                <strong className={e.kind === "venda" || (e.kind === "ajuste" && e.gross_coins >= 0) ? "up" : "down"}>
                  {e.kind === "ajuste" ? formatSignedCoins(e.gross_coins) : `${e.kind === "venda" ? "+" : "−"}${formatCoins(e.gross_coins)}`}
                </strong>
                <span>{e.status} · {formatDateTime(e.recorded_at)}{e.note ? ` · ${e.note}` : ""}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}

// Capital é a tela unificada "/capital": saldo/alocação, as três listas
// (investimentos/vendas/sbcs, agora abas locais em vez de rotas separadas
// — o rail só tem UM link "Capital", ver App.tsx), posições abertas e o
// extrato. As antigas rotas /capital/{investimentos,vendas,sbcs} continuam
// existindo (ver main.tsx) e abrem esta mesma página já na aba certa.
export default function Investimentos({ section }: { section?: CapitalTab }) {
  const [params, setParams] = useSearchParams();
  const tab: CapitalTab = section ?? (params.get("tab") as CapitalTab | null) ?? "investimentos";
  const setTab = (next: CapitalTab) => {
    const nextParams = new URLSearchParams(params);
    nextParams.set("tab", next);
    nextParams.delete("filter");
    nextParams.delete("$filter");
    nextParams.delete("$skip");
    nextParams.delete("open");
    setParams(nextParams);
  };

  const investmentCollection = useCollection<Investment>("/api/capital/investimentos", { defaultOrderBy: [{ field: "momentum_pct", desc: true }], pageSize: 20, enabled: tab === "investimentos" });
  const sellCollection = useCollection<SellCandidate>("/api/capital/vendas", { defaultOrderBy: [{ field: "net_sell_value", desc: true }], pageSize: 20, enabled: tab === "vendas" });
  const fodderCollection = useCollection<FodderSignal>("/api/capital/sbcs", { defaultOrderBy: [{ field: "cost_change_pct", desc: true }], pageSize: 20, enabled: tab === "sbcs" });
  const active = tab === "investimentos" ? investmentCollection : tab === "vendas" ? sellCollection : fodderCollection;
  const filter = params.get("filter") || "todos";
  const sort = params.get("sort") || (tab === "investimentos" ? "desconto" : tab === "vendas" ? "valor" : "pressao");
  const open = params.get("open");
  const gate = asyncGate(active.loading, active.error, active.raw !== null, active.refetch);
  if (gate) return gate;
  if (!active.raw) return null;
  const page = investmentCollection.raw as InvestimentosCollection | null;
  const setParam = (key: string, value: string, defaultValue: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === defaultValue) next.delete(key); else next.set(key, value);
    if (key === "filter") {
      const field = tab === "investimentos" ? "signal" : tab === "vendas" ? "recommendation" : "phase";
      if (!value || value === defaultValue) next.delete("$filter"); else next.set("$filter", `${field} eq '${value.replaceAll("'", "''")}'`);
      next.delete("$skip");
    }
    if (key === "sort") {
      const order = tab === "investimentos" ? (value === "valor" ? "candidate/price/coins desc" : "momentum_pct desc") : tab === "vendas" ? (value === "recomendação" ? "recommendation asc" : "net_sell_value desc") : "cost_change_pct desc";
      next.set("$orderby", order); next.delete("$skip");
    }
    setParams(next);
  };
  const clear = () => { const next = new URLSearchParams(params); ["filter", "$filter", "$skip"].forEach((key) => next.delete(key)); setParams(next); };
  const investments = investmentCollection.rows;
  const sells = sellCollection.rows;
  const fodder = fodderCollection.rows;
  const count = active.count;
  const facets = tab === "investimentos" ? investmentCollection.facets.signal : tab === "vendas" ? sellCollection.facets.recommendation : fodderCollection.facets.phase;
  return <div className="wrap capital-page">
    <PageHeader eyebrow="capital" title="Capital" meta="Saldo, o que fazer com o banco, e o que já foi decidido." />
    <div className="banner">Puramente consultivo: o bot nunca compra nem vende sozinho.</div>
    <SaldoPanel />
    <div className="toggle capital-tabs">
      <button className={tab === "investimentos" ? "active" : ""} onClick={() => setTab("investimentos")}>investimentos</button>
      <button className={tab === "vendas" ? "active" : ""} onClick={() => setTab("vendas")}>vendas do banco</button>
      <button className={tab === "sbcs" ? "active" : ""} onClick={() => setTab("sbcs")}>demanda de SBC</button>
    </div>
    {tab === "investimentos" && <section><h2>Cartas em alta · {count}</h2>{count === 0 ? <InvestmentsEmpty funnel={page?.["@eafc.funnel"] ?? { considered: 0, owned: 0, not_tradeable: 0, superseded_by_sibling: 0, below_min_momentum: 0, suggested: 0, min_momentum_pct: 0, best_rejected_pct: 0, best_rejected_name: "", has_best_rejected: false }} /> : <><RankingControls count={count} sort={sort} onSort={(value) => setParam("sort", value, "desconto")} options={[{ value: "desconto", label: "maior desconto" }, { value: "valor", label: "maior valor" }]} hasFilters={filter !== "todos"} onClear={clear}><FilterSelect label="sinal" value={filter} onChange={(value) => setParam("filter", value, "todos")} options={[{ value: "todos", label: "todos" }, ...(facets ?? []).map((item) => ({ value: item.value, label: `${item.value} (${item.count})` }))]} /></RankingControls><InvestmentRows rows={investments} open={open} onOpen={(key) => setParam("open", key, "")} /><Pagination page={investmentCollection.page} pages={investmentCollection.pages} onPage={investmentCollection.setPage} /></>}</section>}
    {tab === "vendas" && <section><h2>Vale vender do banco? · {count}</h2>{count === 0 ? <EmptyState message="Nada no banco além do XI titular hoje." /> : <><RankingControls count={count} sort={sort} onSort={(value) => setParam("sort", value, "valor")} options={[{ value: "valor", label: "maior valor líquido" }, { value: "recomendação", label: "recomendação" }]} hasFilters={filter !== "todos"} onClear={clear}><FilterSelect label="ação" value={filter} onChange={(value) => setParam("filter", value, "todos")} options={[{ value: "todos", label: "todas" }, ...(facets ?? []).map((item) => ({ value: item.value, label: `${item.value} (${item.count})` }))]} /></RankingControls><SellRows rows={sells} open={open} onOpen={(key) => setParam("open", key, "")} /><Pagination page={sellCollection.page} pages={sellCollection.pages} onPage={sellCollection.setPage} /></>}</section>}
    {tab === "sbcs" && <section><h2>SBCs puxando demanda · {count}</h2>{count === 0 ? <EmptyState message="Nenhum SBC com custo de solução resolvido hoje." /> : <><RankingControls count={count} sort={sort} onSort={(value) => setParam("sort", value, "pressao")} options={[{ value: "pressao", label: "maior pressão" }, { value: "custo", label: "maior alta de custo" }]} hasFilters={filter !== "todos"} onClear={clear}><FilterSelect label="fase" value={filter} onChange={(value) => setParam("filter", value, "todos")} options={[{ value: "todos", label: "todas" }, ...(facets ?? []).map((item) => ({ value: item.value, label: `${item.value} (${item.count})` }))]} /></RankingControls><FodderRows rows={fodder} open={open} onOpen={(key) => setParam("open", key, "")} /><Pagination page={fodderCollection.page} pages={fodderCollection.pages} onPage={fodderCollection.setPage} /></>}</section>}
    <PosicoesPanel />
    <ExtratoPanel />
  </div>;
}
