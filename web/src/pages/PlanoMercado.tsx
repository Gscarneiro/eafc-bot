import { FormEvent, useState } from "react";
import { addWatchlist, appendLedger, fetchMarketPlan } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatCoins, formatDateTime } from "../format";
import type { MarketActionKind, MarketPlanResponse } from "../types";
import { useData } from "../useData";
import "../shared.css";
import "./PlanoMercado.css";

const ACTION_TONE: Record<MarketActionKind, ChipTone> = { comprar: "gain", vender: "cost", esperar: "alert", observar: "flat" };
const ACTIONS_PER_PAGE = 24;

export default function PlanoMercado() {
  const { data, loading, error, refetch } = useData<MarketPlanResponse>(fetchMarketPlan, []);
  const [saving, setSaving] = useState(false);
  const [visibleActions, setVisibleActions] = useState(ACTIONS_PER_PAGE);
  const gate = asyncGate(loading, error, data !== null, refetch);
  if (gate) return gate;
  if (!data) return null;
  const actions = data.plan.actions ?? [];
  const actionsToShow = actions.slice(0, visibleActions);
  const watchlist = data.watchlist ?? [];
  const ledger = data.ledger ?? [];
  const addWatch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const form = new FormData(event.currentTarget); setSaving(true);
    try { await addWatchlist({ ea_id: Number(form.get("ea_id")), name: String(form.get("name")), target_coins: Number(form.get("target_coins")) || undefined, protected: form.get("protected") === "on" }); event.currentTarget.reset(); refetch(); } finally { setSaving(false); }
  };
  const addEntry = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault(); const form = new FormData(event.currentTarget); setSaving(true);
    try { await appendLedger({ kind: String(form.get("kind")), status: String(form.get("status")), gross_coins: Number(form.get("gross_coins")), note: String(form.get("note")) || undefined }); event.currentTarget.reset(); refetch(); } finally { setSaving(false); }
  };
  return <div className="wrap market-plan">
    <PageHeader eyebrow="mercado · plano global" title="Mesa de decisão" meta="Uma visão do caixa, das escolhas que competem entre si e do que ainda precisa de confirmação." />
    <div className="banner">Consultivo e local: registrar aqui não compra, vende, aplica Evolução nem acessa sua conta EA.</div>
    <section className="capital-tape" aria-label="posição de capital"><div><span>disponível agora</span><strong>{formatCoins(data.plan.capital.available)}</strong></div><div><span>reserva protegida</span><strong>{formatCoins(data.plan.capital.reserve)}</strong></div><div className="committed"><span>já comprometido</span><strong>{formatCoins(data.ledger_summary.committed)}</strong></div><div><span>P&L confirmado</span><strong className={data.ledger_summary.pnl >= 0 ? "gain" : "cost"}>{data.ledger_summary.pnl >= 0 ? "+" : ""}{formatCoins(data.ledger_summary.pnl)}</strong></div></section>
    {data.plan.conflicts?.length ? <div className="banner alert"><strong>Conflitos para decidir antes:</strong><ul>{data.plan.conflicts.map((conflict) => <li key={conflict}>{conflict}</li>)}</ul></div> : null}
    <section><h2>Próximas ações · {actions.length}</h2>{actions.length === 0 ? <EmptyState message="Sem ações novas no snapshot atual." hint="Faça uma coleta quando houver novas opções de mercado." /> : <><div className="decision-list">{actionsToShow.map((action) => <article className="decision" key={`${action.kind}-${action.ea_id}-${action.name}`}><div className="decision-head"><Chip tone={ACTION_TONE[action.kind]}>{action.origin ?? action.kind}</Chip><strong>{action.name}</strong>{action.position ? <span>{action.position}</span> : null}<small>confiança {action.confidence}</small></div><div className="decision-money"><span>bruto {formatCoins(action.gross_cost)}</span><strong>líquido {formatCoins(action.net_cost)}</strong></div>{action.break_even_gross ? <p>Empata vendendo por {formatCoins(action.break_even_gross)}; não é garantia de liquidez.</p> : null}{action.rationale?.length ? <p>{action.rationale[0]}</p> : null}{action.conflicts?.length ? <p className="decision-conflict">{action.conflicts.join(" · ")}</p> : null}</article>)}</div>{actionsToShow.length < actions.length ? <div className="decision-more"><p aria-live="polite">Mostrando {actionsToShow.length} de {actions.length} decisões.</p><button className="btn" type="button" onClick={() => setVisibleActions((count) => count + ACTIONS_PER_PAGE)}>mostrar mais {Math.min(ACTIONS_PER_PAGE, actions.length - actionsToShow.length)}</button></div> : null}</>}</section>
    <section className="market-records"><div><h2>Watchlist local</h2><form onSubmit={addWatch} className="market-form"><input name="ea_id" type="number" min="1" required placeholder="ID da carta" aria-label="ID da carta" /><input name="name" required placeholder="Nome da carta" aria-label="Nome da carta" /><input name="target_coins" type="number" min="0" placeholder="Preço-alvo" aria-label="Preço-alvo" /><label><input name="protected" type="checkbox" /> proteger</label><button className="btn" disabled={saving}>adicionar</button></form>{watchlist.length === 0 ? <p className="hint">Nenhuma carta acompanhada ainda.</p> : <ul className="record-list">{watchlist.map((item) => <li key={item.id}><strong>{item.name}</strong>{item.target_coins ? <span>alvo {formatCoins(item.target_coins)}</span> : null}{item.protected ? <Chip tone="alert">protegida</Chip> : null}</li>)}</ul>}</div><div><h2>Ledger local</h2><form onSubmit={addEntry} className="market-form"><select name="kind" defaultValue="compra"><option value="compra">compra</option><option value="venda">venda</option><option value="sbc">SBC</option><option value="evolucao">Evolução</option><option value="ajuste">ajuste</option></select><select name="status" defaultValue="confirmado"><option value="confirmado">confirmado</option><option value="planejado">planejado</option></select><input name="gross_coins" type="number" required placeholder="Moedas" aria-label="Moedas" /><input name="note" placeholder="Nota opcional" aria-label="Nota" /><button className="btn" disabled={saving}>registrar</button></form>{ledger.length === 0 ? <p className="hint">Nenhum lançamento manual neste ciclo.</p> : <ul className="record-list">{ledger.slice(0, 5).map((entry) => <li key={entry.id}><Chip tone={entry.status === "planejado" ? "alert" : "flat"}>{entry.kind}</Chip><strong>{formatCoins(entry.gross_coins)}</strong><span>{entry.status} · {formatDateTime(entry.recorded_at)}</span></li>)}</ul>}</div></section>
  </div>;
}
