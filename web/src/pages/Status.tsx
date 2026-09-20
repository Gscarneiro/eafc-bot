import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import {
  appendFeedback,
  fetchAgenda,
  fetchCollection,
  fetchEvolutionProgressList,
  fetchExtrato,
  fetchResumo,
  fetchStatus,
  fetchWatchlist,
} from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import PageHeader from "../components/PageHeader";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatDate, formatDateTime, formatSigned, formatSignedCoins } from "../format";
import { useData } from "../useData";
import type {
  AgendaResponse,
  Aviso,
  EvolutionProgressItem,
  ExtratoResponse,
  LedgerEntry,
  NewsItem,
  Objective,
  ResumoResponse,
  SBC,
  StatusResponse,
  TickerRow,
  TopMove,
  Upgrade,
  WatchlistRow,
} from "../types";
import "../shared.css";
import "./Status.css";

const PAINEL_KEY = "eafc-bot:painel";

// Status é a tela "/" — o briefing do dia, em dois modos: 1a ("decisão")
// abre com UMA jogada em destaque e o resto em segundo plano; 1b
// ("terminal") mostra tudo numa grade densa, sem nada enterrado. Os dois
// leem os MESMOS dados — só a hierarquia visual muda; ver CLAUDE.md sobre
// status diário.
export default function Status() {
  const [painel, setPainel] = useState<"decisao" | "terminal">(() => {
    try {
      return localStorage.getItem(PAINEL_KEY) === "terminal" ? "terminal" : "decisao";
    } catch {
      return "decisao";
    }
  });
  useEffect(() => {
    try {
      localStorage.setItem(PAINEL_KEY, painel);
    } catch {
      // Sem storage disponível, a tela ainda funciona — só esquece a
      // preferência ao recarregar.
    }
  }, [painel]);

  const { data, error, loading, refetch } = useData(fetchStatus, []);
  const resumoData = useData(fetchResumo, []);
  const agendaData = useData(fetchAgenda, []);
  const progressoData = useData(fetchEvolutionProgressList, []);
  const extratoData = useData(() => fetchExtrato("$top=5"), []);
  const watchlistData = useData(fetchWatchlist, []);
  const oportunidadesData = useData(() => fetchCollection<Upgrade>("/api/mercado", { top: 5, orderBy: [{ field: "efficiency", desc: true }] }), []);

  const gate = asyncGate(
    loading,
    error,
    !!data,
    () => { refetch(); resumoData.refetch(); agendaData.refetch(); progressoData.refetch(); extratoData.refetch(); watchlistData.refetch(); oportunidadesData.refetch(); },
  );
  if (gate) return gate;
  if (!data) return null;

  const resumo = resumoData.data ?? undefined;
  const agenda = agendaData.data ?? undefined;
  const progresso = progressoData.data?.items ?? [];
  const extrato = extratoData.data ?? undefined;
  const watchlist = watchlistData.data?.value ?? [];
  const oportunidades = oportunidadesData.data?.value ?? [];

  const history = data.history ?? [];
  const scoreHistory = history.map((h) => ({ label: formatDate(h.date), value: h.squad_score }));
  const coinsHistory = history.map((h) => ({ label: formatDate(h.date), value: h.coins }));

  const added = data.diff.added ?? [];
  const removed = data.diff.removed ?? [];
  const newCards = data.new_cards ?? [];
  const news = data.news ?? [];
  const sbcs = data.sbcs ?? [];
  const objectives = data.objectives ?? [];
  const errors = data.errors ?? [];
  const changed = added.length > 0 || removed.length > 0 || data.diff.coins_delta !== 0;

  return (
    <div className="wrap hoje-page">
      <PageHeader
        eyebrow={`briefing · ciclo ${data.cycle}`}
        title="Hoje"
        meta={`gerado ${formatDateTime(data.generated_at)}`}
        actions={
          <div className="toggle">
            <button className={painel === "decisao" ? "active" : ""} onClick={() => setPainel("decisao")}>decisão</button>
            <button className={painel === "terminal" ? "active" : ""} onClick={() => setPainel("terminal")}>terminal</button>
          </div>
        }
      />

      {errors.length > 0 && (
        <div className="banner alert">
          {errors.length} {errors.length === 1 ? "aviso" : "avisos"} nesta coleta
          <ul>{errors.map((e, i) => <li key={i}>{e}</li>)}</ul>
        </div>
      )}

      {painel === "decisao" ? (
        <HojeDecisao data={data} resumo={resumo} agenda={agenda} progresso={progresso} extrato={extrato} scoreHistory={scoreHistory} coinsHistory={coinsHistory} />
      ) : (
        <HojeTerminal data={data} resumo={resumo} agenda={agenda} progresso={progresso} extrato={extrato} watchlist={watchlist} oportunidades={oportunidades} scoreHistory={scoreHistory} coinsHistory={coinsHistory} />
      )}

      {changed && (
        <section>
          <h2>Mudou desde ontem</h2>
          {added.map((p) => (
            <div className="list-row" key={`a-${p.id}`}>
              <span className="title"><Chip tone="gain">+</Chip> {p.common_name || p.name}</span>
              <span className="meta">{p.rating} {p.position}</span>
            </div>
          ))}
          {removed.map((p) => (
            <div className="list-row" key={`r-${p.id}`}>
              <span className="title"><Chip tone="cost">−</Chip> {p.common_name || p.name}</span>
              <span className="meta">{p.rating} {p.position}</span>
            </div>
          ))}
        </section>
      )}

      {(newCards.length > 0 || news.length > 0 || sbcs.length > 0 || objectives.length > 0) && (
        <section className="feed-grid">
          {newCards.length > 0 && (
            <div className="feed-card">
              <h2>Cartas novas no mercado</h2>
              {newCards.map((p) => (
                <div className="list-row" key={p.id}>
                  <span className="title">{p.common_name || p.name}</span>
                  <span className="meta">{p.rating} {p.position} · {p.version}</span>
                </div>
              ))}
            </div>
          )}
          {news.length > 0 && (
            <div className="feed-card">
              <h2>Notícias</h2>
              {news.map((n: NewsItem) => (
                <div className="list-row" key={n.id || n.url || n.title}>
                  {n.url ? <a className="title" href={n.url} target="_blank" rel="noreferrer">{n.title}</a> : <span className="title">{n.title}</span>}
                  <span className="meta">{formatDate(n.published_at)}</span>
                  {n.summary && <p className="desc">{n.summary}</p>}
                </div>
              ))}
            </div>
          )}
          {sbcs.length > 0 && (
            <div className="feed-card">
              <h2>SBCs que valem a pena</h2>
              {sbcs.map((s: SBC) => (
                <div className="list-row" key={s.id}>
                  <span className="title">{s.name}</span>
                  <Chip tone="coin">{formatCoins(s.solution_cost)}</Chip>
                </div>
              ))}
            </div>
          )}
          {objectives.length > 0 && (
            <div className="feed-card">
              <h2>Objetivos</h2>
              {objectives.map((o: Objective) => (
                <div className="list-row" key={o.id}>
                  <span className="title">{o.name}</span>
                  <span className="meta">{o.group}</span>
                </div>
              ))}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

interface ModeProps {
  data: StatusResponse;
  resumo?: ResumoResponse;
  agenda?: AgendaResponse;
  progresso: EvolutionProgressItem[];
  extrato?: ExtratoResponse;
}

// HojeDecisao (1a): uma jogada em destaque, o resto em segundo plano — a
// tese é "o que eu faço agora".
function HojeDecisao({ data, resumo, agenda, progresso, extrato, scoreHistory, coinsHistory }: ModeProps & { scoreHistory: { label: string; value: number }[]; coinsHistory: { label: string; value: number }[] }) {
  return (
    <>
      <div className="decisao-grid">
        <TopMoveHero move={data.top_move} />
        <TarefasCard agenda={agenda} />
      </div>
      <div className="sparkline-grid">
        <SparklineCard
          label="Nota do elenco · 30 dias"
          value={data.squad_score.toFixed(1)}
          delta={resumo && Math.abs(resumo.squad_score_delta ?? 0) >= 0.05 ? formatSigned(resumo.squad_score_delta ?? 0) : undefined}
          data={scoreHistory}
          color="var(--turf)"
        />
        <SparklineCard
          label="Saldo · 30 dias"
          value={formatCoins(data.coins)}
          delta={resumo?.coins_delta ? formatSignedCoins(resumo.coins_delta) : undefined}
          data={coinsHistory}
          color="var(--coin)"
        />
      </div>
      <div className="triple-grid">
        <AvisosCard avisos={resumo?.avisos} />
        <GaleriaMini opportunities={resumo?.gallery_opportunities ?? 0} />
        <EvolucoesPipelineCard items={progresso} />
        <CaixaCard extrato={extrato} />
      </div>
    </>
  );
}

// HojeTerminal (1b): grade densa, mesa de operação — a tese é "como está
// tudo", nada enterrado atrás de um toggle.
function HojeTerminal({ data, resumo, agenda, progresso, extrato, watchlist, oportunidades, scoreHistory, coinsHistory }: ModeProps & { watchlist: WatchlistRow[]; oportunidades: Upgrade[]; scoreHistory: { label: string; value: number }[]; coinsHistory: { label: string; value: number }[] }) {
  return (
    <>
      <TickerStrip rows={resumo?.ticker ?? []} />
      <KPIBand data={data} resumo={resumo} />
      <div className="terminal-grid">
        <div className="terminal-col">
          <OportunidadesMini rows={oportunidades} />
        </div>
        <div className="terminal-col">
          <SparklineCard label="Nota do XI · 30d" value={data.squad_score.toFixed(1)} data={scoreHistory} color="var(--turf)" />
          <SparklineCard label="Saldo · 30d" value={formatCoins(data.coins)} data={coinsHistory} color="var(--coin)" />
          <VigiadasCard rows={watchlist} />
        </div>
        <div className="terminal-col">
          <AvisosCard avisos={resumo?.avisos} />
          <GaleriaMini opportunities={resumo?.gallery_opportunities ?? 0} />
          <TarefasCard agenda={agenda} />
          <EvolucoesPipelineCard items={progresso} />
          <CaixaCard extrato={extrato} />
        </div>
      </div>
    </>
  );
}

function GaleriaMini({ opportunities }: { opportunities: number }) {
  return <div className="panel"><div className="panel-head"><span>FUT Gallery <span className="panel-head-sub">/ oportunidades</span></span><span className="panel-head-meta">{opportunities}</span></div><div className="panel-body"><p className="hint">{opportunities > 0 ? `${opportunities} conjunto${opportunities === 1 ? "" : "s"} pode${opportunities === 1 ? "" : "m"} ser concluído${opportunities === 1 ? "" : "s"} ou melhorar de letra.` : "Nenhuma melhoria de letra pendente."}</p><Link className="btn ghost" to="/galeria">abrir Gallery</Link></div></div>;
}

function TopMoveHero({ move }: { move?: TopMove }) {
  const [sent, setSent] = useState(false);
  const [sending, setSending] = useState(false);
  const descartar = async () => {
    if (!move || sending || sent) return;
    setSending(true);
    try {
      await appendFeedback({ action_id: move.action_id, status: "descartada" });
      setSent(true);
    } finally {
      setSending(false);
    }
  };
  return (
    <div className="panel hero-move-panel">
      <div className="panel-head">
        <span>Jogada de hoje <span className="panel-head-sub">/ Top move</span></span>
        {move?.kind === "upgrade" && move.efficiency ? <span className="panel-head-meta">eficiência {move.efficiency.toFixed(2)}</span> : null}
      </div>
      <div className="panel-body">
        {!move ? (
          <p className="hint">Nenhum upgrade de mercado nem evolução passou do ganho mínimo hoje.</p>
        ) : (
          <>
            <div className="hero-move-headline">{move.headline}</div>
            <div className="hero-move-stats">
              <div><span>Ganho</span><strong className="up">{formatSigned(move.gain)}</strong></div>
              {move.kind === "upgrade" ? (
                <>
                  <div><span>Custo líq.</span><strong className="coin">{formatCoins(move.net_cost)}</strong></div>
                  <div><span>Compra</span><strong>{formatCoins(move.gross_cost ?? 0)}</strong></div>
                  <div><span>Venda</span><strong>{formatCoins(move.recoup ?? 0)}</strong></div>
                </>
              ) : (
                <div><span>Custo</span><strong className="coin">{move.net_cost > 0 ? formatCoins(move.net_cost) : "grátis"}</strong></div>
              )}
            </div>
            {move.rationale?.[0] && <p className="hero-move-rationale">{move.rationale[0]}</p>}
            <div className="hero-move-actions">
              <Link className="btn primary" to={move.link}>{move.kind === "upgrade" ? "abrir no mercado" : "ver evolução"}</Link>
              <Link className="btn ghost" to="/mercado/plano">comparar</Link>
              {sent ? <span className="hint">descartado</span> : <button className="btn" type="button" onClick={descartar} disabled={sending}>descartar</button>}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

function TarefasCard({ agenda }: { agenda?: AgendaResponse }) {
  const [localDone, setLocalDone] = useState<Record<string, boolean>>({});
  const items = agenda ? [...(agenda.agenda.agora ?? []), ...(agenda.agenda.esta_semana ?? [])].slice(0, 5) : [];
  const feedback = agenda?.feedback ?? {};
  const doneCount = items.filter((a) => localDone[a.id] || feedback[a.id] === "aceita").length;
  const toggle = async (a: { id: string }) => {
    if (localDone[a.id] || feedback[a.id] === "aceita") return;
    setLocalDone((d) => ({ ...d, [a.id]: true }));
    try {
      await appendFeedback({ action_id: a.id, status: "aceita" });
    } catch {
      setLocalDone((d) => ({ ...d, [a.id]: false }));
    }
  };
  return (
    <div className="panel tarefas-panel">
      <div className="panel-head">
        <span>Tarefas de hoje <span className="panel-head-sub">/ Checklist</span></span>
        <span className="panel-head-meta">{doneCount}/{items.length}</span>
      </div>
      <div className="tarefas-list">
        {items.length === 0 && <p className="hint" style={{ padding: "10px 12px" }}>Nada pendente na agenda hoje.</p>}
        {items.map((a) => {
          const done = localDone[a.id] || feedback[a.id] === "aceita";
          return (
            <label key={a.id} className={`tarefa-item${done ? " done" : ""}`}>
              <input type="checkbox" checked={done} onChange={() => toggle(a)} disabled={done} />
              <span className="tarefa-text">
                <span className="tarefa-title">{a.alvo}</span>
                <span className="tarefa-meta">{a.impacto || a.proveniencia}{a.moedas ? ` · ${formatCoins(a.moedas)}` : ""}</span>
              </span>
            </label>
          );
        })}
      </div>
      <div className="panel-body top-border"><Link className="btn ghost" to="/agenda">ver agenda completa</Link></div>
    </div>
  );
}

function AvisosCard({ avisos }: { avisos?: Aviso[] }) {
  return (
    <div className="panel avisos-panel">
      <div className="panel-head">
        <span>Avisos <span className="panel-head-sub">/ Alerts</span></span>
        {avisos && avisos.length > 0 && <span className="panel-head-meta">{avisos.length}</span>}
      </div>
      <div className="avisos-list">
        {(!avisos || avisos.length === 0) ? (
          <p className="hint" style={{ padding: "10px 12px" }}>Nada pendente.</p>
        ) : (
          avisos.map((a, i) => (
            <div key={i} className={`aviso-row severity-${a.severity}`}>
              <span className="aviso-bar" />
              <span>
                {a.link ? <Link className="aviso-headline" to={a.link}>{a.headline}</Link> : <span className="aviso-headline">{a.headline}</span>}
                {a.detail && <span className="aviso-detail">{a.detail}</span>}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function EvolucoesPipelineCard({ items }: { items: EvolutionProgressItem[] }) {
  return (
    <div className="panel">
      <div className="panel-head"><span>Evoluções <span className="panel-head-sub">/ Pipeline</span></span></div>
      <div className="panel-body pipeline-list">
        {items.length === 0 && <p className="hint">Nenhum path salvo em progresso.</p>}
        {items.slice(0, 5).map((item) => (
          <div className="pipeline-row" key={item.card_slug + item.name}>
            <div className="pipeline-head"><span>{item.name}</span><span>{item.completed}/{item.total}</span></div>
            <span className="pipeline-track"><span style={{ width: `${item.total > 0 ? (item.completed / item.total) * 100 : 0}%` }} /></span>
          </div>
        ))}
      </div>
    </div>
  );
}

function ledgerLabel(e: LedgerEntry): string {
  return e.note || `${e.kind} · ${e.status}`;
}

function CaixaCard({ extrato }: { extrato?: ExtratoResponse }) {
  const entries = extrato?.value ?? [];
  const resultado = extrato?.["@eafc.summary_7d"]?.net_cash ?? 0;
  return (
    <div className="panel">
      <div className="panel-head">
        <span>Caixa 7d <span className="panel-head-sub">/ Ledger</span></span>
        <span className={`panel-head-meta ${resultado >= 0 ? "up" : "down"}`}>{formatSignedCoins(resultado)}</span>
      </div>
      <div className="caixa-list">
        {entries.length === 0 && <p className="hint" style={{ padding: "10px 12px" }}>Nenhum lançamento recente.</p>}
        {entries.slice(0, 5).map((e) => (
          <div className="caixa-row" key={e.id}>
            <span>{ledgerLabel(e)}</span>
            {/* ajuste é o único tipo com gross_coins já assinado (Validate
                exige positivo nos demais) — prefixar "−" duplicaria o sinal
                num ajuste negativo, ver o mesmo ajuste em Investimentos.tsx. */}
            <span className={e.kind === "venda" || (e.kind === "ajuste" && e.gross_coins >= 0) ? "up" : "down"}>
              {e.kind === "ajuste" ? formatSignedCoins(e.gross_coins) : `${e.kind === "venda" ? "+" : "−"}${formatCoins(e.gross_coins)}`}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

function SparklineCard({ label, value, delta, data, color }: { label: string; value: string; delta?: string; data: { label: string; value: number }[]; color?: string }) {
  return (
    <div className="panel sparkline-panel">
      <div className="panel-head">
        <span>{label}</span>
        <span className="panel-head-meta">{value}{delta ? ` ${delta}` : ""}</span>
      </div>
      <div className="panel-body">
        <TrendChart data={data} compact height={70} color={color} />
      </div>
    </div>
  );
}

function TickerStrip({ rows }: { rows: TickerRow[] }) {
  if (rows.length === 0) return null;
  return (
    <div className="ticker-strip">
      <span className="ticker-label">Mercado</span>
      {rows.map((r, i) => (
        <span key={i} className="ticker-item">{r.name} <span className={r.trend.change_pct >= 0 ? "up" : "down"}>{formatSigned(r.trend.change_pct)}%</span></span>
      ))}
    </div>
  );
}

function KPIBand({ data, resumo }: { data: StatusResponse; resumo?: ResumoResponse }) {
  const items: { label: string; value: string; sub?: string }[] = [
    { label: "Saldo", value: formatCoins(data.coins) },
    { label: "Levantável", value: formatCoins(data.raisable) },
    { label: "Nota do XI", value: data.squad_score.toFixed(1) },
    { label: "Química", value: resumo?.chemistry ? `${resumo.chemistry.total}/${resumo.chemistry.maximo}` : "—" },
    { label: "Elo mais fraco", value: data.weakest_name || "—", sub: data.weakest_slot || undefined },
    { label: "Trocas viáveis", value: resumo ? String(resumo.trocas_viaveis) : "—" },
  ];
  return (
    <div className="kpi-band">
      {items.map((item) => (
        <div className="kpi-tile" key={item.label}>
          <span className="kpi-label">{item.label}</span>
          <span className="kpi-value">{item.value}</span>
          {item.sub && <span className="kpi-sub">{item.sub}</span>}
        </div>
      ))}
    </div>
  );
}

function OportunidadesMini({ rows }: { rows: Upgrade[] }) {
  return (
    <div className="panel">
      <div className="panel-head"><span>Oportunidades <span className="panel-head-sub">/ por eficiência</span></span></div>
      <div className="tablewrap">
        <table>
          <thead><tr><th>Troca</th><th className="num">GG</th><th className="num">Líq.</th></tr></thead>
          <tbody>
            {rows.map((u) => (
              <tr key={`${u.slot}-${u.candidate.id}`}>
                <td><strong>{u.current.common_name || u.current.name} → {u.candidate.common_name || u.candidate.name}</strong><span className="hint">{u.slot}</span></td>
                <td className="num up">{formatSigned(u.gain)}</td>
                <td className="num coin">{u.unpriced ? "—" : formatCoins(u.net_cost)}</td>
              </tr>
            ))}
            {rows.length === 0 && <tr><td colSpan={3} className="hint">Nada bateu o ganho mínimo hoje.</td></tr>}
          </tbody>
        </table>
      </div>
      <div className="panel-body top-border"><Link className="btn ghost" to="/mercado">ver todas</Link></div>
    </div>
  );
}

function VigiadasCard({ rows }: { rows: WatchlistRow[] }) {
  return (
    <div className="panel">
      <div className="panel-head"><span>Vigiadas <span className="panel-head-sub">/ Watchlist</span></span></div>
      <div className="watchlist-list">
        {rows.length === 0 && <p className="hint" style={{ padding: "10px 12px" }}>Nada na watchlist ainda.</p>}
        {rows.map((row) => (
          <div className="watchlist-row" key={row.entry.id}>
            <span>{row.entry.name}</span>
            <span className={row.has_trend && row.trend.change_pct < 0 ? "down" : "up"}>{row.has_trend ? `${formatSigned(row.trend.change_pct)}%` : "—"}</span>
          </div>
        ))}
      </div>
      <div className="panel-body top-border"><Link className="btn ghost" to="/mercado/plano">gerenciar</Link></div>
    </div>
  );
}
