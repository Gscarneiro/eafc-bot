import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { addWatchlist, fetchCollection, fetchWatchlist, updateWatchlist } from "../api";
import { CardArt } from "../components/FutPitch";
import { formatCoins } from "../format";
import { useData } from "../useData";
import type { CollectionCard } from "../types";

// TimeLimpeza é a seção "Limpeza do clube" do Meu time (antes a tela
// /time/insights): as cópias paradas fora do XI e os untradeables que
// servem de fodder. Nenhuma ação é automática — o bot nunca toca na conta
// EA. "vender" e "SBC" levam às telas de Capital; "proteger" só grava a
// carta na watchlist local, e a proteção já tira a carta das sugestões de
// venda e de SBC do resto do bot.

const STALE_DAYS = 30;
const QUEUE_PAGE = 20;
type Recommendation = "sell" | "sbc" | "keep" | "protected";
type Row = {
  card: CollectionCard;
  untradeable: boolean;
  days: number;
  net?: number;
  rec: Recommendation;
  reason: string;
};

// net é o que entra no saldo: o preço menos os 5% da EA, na mesma conta
// inteira de domain.ClubPlayer.NetSellValue.
function netOf(card: CollectionCard): number | undefined {
  const coins = card.player.price?.coins ?? 0;
  if (card.player.untradeable || coins <= 0) return undefined;
  return Math.floor((coins * 95) / 100);
}

function toRow(card: CollectionCard): Row {
  const untradeable = card.player.untradeable;
  const days = card.permanence_days;
  const net = netOf(card);
  if (card.protected) return { card, untradeable, days, net, rec: "protected", reason: "Protegida na watchlist — fica fora das sugestões de venda e de SBC." };
  if (untradeable) {
    return { card, untradeable, days, net, rec: "sbc", reason: card.count > 1 ? `${card.count} cópias iguais untradeable — fodder de SBC sem custo de oportunidade` : "Untradeable fora do XI — serve como fodder de SBC" };
  }
  if (net === undefined) return { card, untradeable, days, net, rec: "keep", reason: "Sem cotação nesta coleta — confira o preço antes de vender" };
  if (days > STALE_DAYS) return { card, untradeable, days, net, rec: "sell", reason: `${days} dias no clube sem entrar no XI` };
  return { card, untradeable, days, net, rec: "keep", reason: days > 0 ? `${days} dias no clube — ainda dentro do corte de ${STALE_DAYS} dias` : "Primeira coleta com esta carta — sem tempo de permanência ainda" };
}

const DAY_BUCKETS: { label: string; min: number; max: number }[] = [
  { label: "0-7d", min: 0, max: 7 },
  { label: "8-14d", min: 8, max: 14 },
  { label: "15-30d", min: 15, max: 30 },
  { label: "31-60d", min: 31, max: 60 },
  { label: "60d+", min: 61, max: Infinity },
];
const OVR_BANDS: { label: string; min: number; max: number }[] = [
  { label: "≤81", min: 0, max: 81 },
  { label: "82-83", min: 82, max: 83 },
  { label: "84-85", min: 84, max: 85 },
  { label: "86-87", min: 86, max: 87 },
  { label: "88+", min: 88, max: Infinity },
];

function compactCoins(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toLocaleString("pt-BR", { maximumFractionDigits: 2 })}M`;
  return formatCoins(value);
}

export default function TimeLimpeza() {
  const { data, error, loading, refetch } = useData(() => fetchCollection<CollectionCard>("/api/clube/colecao", { top: 500 }), []);
  const [filter, setFilter] = useState<"all" | "stale" | "untradeable">("all");
  const [visible, setVisible] = useState(QUEUE_PAGE);
  const [busy, setBusy] = useState<number | null>(null);
  const [actionError, setActionError] = useState("");
  const cards = data?.value ?? [];
  const fodder = useMemo(() => cards.filter((card) => card.fodder_candidate).map(toRow).sort((a, b) => b.days - a.days || b.card.player.rating - a.card.player.rating), [cards]);

  const head = <div className="section-title-row">
    <div><h2>Limpeza do clube</h2><p className="section-note">Cartas paradas fora do XI e untradeables para SBC. Nenhuma ação é automática.</p></div>
  </div>;
  if (loading) return <section className="time-section" id="limpeza">{head}<p className="hint">Lendo a coleção do clube…</p></section>;
  if (error || !data) return <section className="time-section" id="limpeza">{head}<div className="banner alert">{error?.message || "Não foi possível ler a coleção do clube."} <button type="button" className="btn ghost" onClick={refetch}>tentar novamente</button></div></section>;

  const copies = (rows: Row[]) => rows.reduce((total, row) => total + row.card.count, 0);
  const stale = fodder.filter((row) => row.days > STALE_DAYS);
  const untradeables = fodder.filter((row) => row.untradeable);
  const tradeables = fodder.filter((row) => !row.untradeable);
  const netTotal = tradeables.reduce((total, row) => total + (row.net ?? 0) * row.card.count, 0);
  const unpriced = tradeables.filter((row) => row.net === undefined).length;
  const protectedCount = cards.filter((card) => card.protected).length;
  const truncated = (data["@odata.count"] ?? cards.length) > cards.length;

  const dayBuckets = DAY_BUCKETS.map((bucket) => {
    const inBucket = fodder.filter((row) => row.days >= bucket.min && row.days <= bucket.max);
    return { ...bucket, tradeable: copies(inBucket.filter((row) => !row.untradeable)), untradeable: copies(inBucket.filter((row) => row.untradeable)) };
  });
  const dayMax = Math.max(1, ...dayBuckets.map((bucket) => bucket.tradeable + bucket.untradeable));
  const ovrBands = OVR_BANDS.map((band) => ({ ...band, count: copies(untradeables.filter((row) => row.card.player.rating >= band.min && row.card.player.rating <= band.max)) }));
  const ovrMax = Math.max(1, ...ovrBands.map((band) => band.count));
  const upgradeFodder = ovrBands.filter((band) => band.min >= 84).reduce((total, band) => total + band.count, 0);

  const queue = fodder.filter((row) => filter === "all" || (filter === "stale" ? row.days > STALE_DAYS : row.untradeable));

  const toggleProtect = async (row: Row) => {
    const player = row.card.player;
    setBusy(player.id); setActionError("");
    try {
      const watchlist = await fetchWatchlist();
      const existing = watchlist.value.find((item) => item.entry.ea_id === player.id)?.entry;
      if (existing) await updateWatchlist({ ...existing, protected: !row.card.protected });
      else await addWatchlist({ ea_id: player.id, name: player.common_name || player.name, protected: true });
      refetch();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Não foi possível atualizar a proteção.");
    } finally {
      setBusy(null);
    }
  };

  return <section className="time-section" id="limpeza">
    {head}
    <div className="time-clean-stats">
      <div><span>Fora do XI</span><strong>{copies(fodder)}</strong><small> cópias</small></div>
      <div><span>Paradas &gt;{STALE_DAYS}d</span><strong className="alert">{copies(stale)}</strong><small> cópias</small></div>
      <div><span>Líquido negociável</span><strong className="coin">{compactCoins(netTotal)}</strong><small>{unpriced ? ` · ${unpriced} s/ cot.` : ""}</small></div>
      <div><span>Untradeables</span><strong>{copies(untradeables)}</strong><small> cópias</small></div>
      <div><span>Protegidas</span><strong>{protectedCount}</strong></div>
    </div>
    {truncated ? <p className="hint">A coleção passou de {cards.length} versões — os números acima cobrem as {cards.length} de maior overall.</p> : null}

    <div className="time-reading-grid time-clean-grid">
      <div className="panel">
        <div className="panel-head"><span>Tempo parado <span className="panel-head-sub">/ cópias por dias no clube</span></span></div>
        <div className="panel-body">
          <div className="time-stack">
            {dayBuckets.map((bucket, index) => <div key={bucket.label} className={`${bucket.min > STALE_DAYS ? "stale" : ""}${index === 3 ? " cut" : ""}`} title={`${bucket.tradeable} negociáveis · ${bucket.untradeable} untradeable`}>
              <div className="time-stack-col">
                <span className="untradeable" style={{ height: `${(bucket.untradeable / dayMax) * 100}%` }} />
                <span className="tradeable" style={{ height: `${(bucket.tradeable / dayMax) * 100}%` }} />
              </div>
              <span className="time-stack-label">{bucket.label}</span>
            </div>)}
          </div>
          <div className="time-mini-legend"><span><i className="coin" /> negociável</span><span><i className="hatch" /> untradeable</span><span><i className="cut" /> corte de {STALE_DAYS} dias</span></div>
        </div>
      </div>
      <div className="panel">
        <div className="panel-head"><span>Fodder de SBC <span className="panel-head-sub">/ untradeables por OVR</span></span><span className="panel-head-meta">{upgradeFodder} cópias 84+</span></div>
        <div className="panel-body time-fodder">
          {ovrBands.map((band) => <div key={band.label} className={band.min >= 84 ? "upgrade" : ""}>
            <span className="time-fodder-label">{band.label}</span>
            <span className="time-fodder-track"><span style={{ width: `${(band.count / ovrMax) * 100}%` }} /></span>
            <span className="time-fodder-value">{band.count}</span>
          </div>)}
          <p className="time-footnote">Em verde, faixas que costumam cobrir SBCs de upgrade.</p>
        </div>
      </div>
    </div>

    <div className="panel time-queue">
      <div className="panel-head">
        <span>Fila de limpeza <span className="panel-head-sub">/ sugestão do bot por carta</span></span>
        <div className="toggle time-queue-filter">
          <button type="button" className={filter === "all" ? "active" : ""} onClick={() => { setFilter("all"); setVisible(QUEUE_PAGE); }}>todas</button>
          <button type="button" className={filter === "stale" ? "active" : ""} onClick={() => { setFilter("stale"); setVisible(QUEUE_PAGE); }}>paradas &gt;{STALE_DAYS}d</button>
          <button type="button" className={filter === "untradeable" ? "active" : ""} onClick={() => { setFilter("untradeable"); setVisible(QUEUE_PAGE); }}>untradeable</button>
        </div>
      </div>
      {actionError ? <p className="time-queue-error" role="alert">{actionError}</p> : null}
      {queue.length === 0 ? <p className="time-queue-empty">Nenhuma carta neste filtro.</p> : <div className="tablewrap flush">
        <table>
          <thead><tr><th>Carta</th><th className="num">OVR</th><th className="num">Cópias</th><th className="num">Parada há</th><th>Tipo</th><th className="num">Líquido</th><th>Motivo</th><th>Ação</th></tr></thead>
          <tbody>{queue.slice(0, visible).map((row) => {
            const player = row.card.player;
            return <tr key={`${player.id}-${player.version}`}>
              <td className="namecell"><span className="time-card-cell"><CardArt player={player} className="time-thumb" /><span>{player.common_name || player.name}</span><small>{player.position}</small></span></td>
              <td className="num">{player.rating}</td>
              <td className="num">×{row.card.count}</td>
              <td className={`num time-days${row.days > STALE_DAYS ? " stale" : ""}`}>{row.days > 0 ? `${row.days}d` : "—"}</td>
              <td><span className={`chip ${row.untradeable ? "flat" : "coin"}`}>{row.untradeable ? "untradeable" : "negociável"}</span></td>
              <td className="num coin">{row.net !== undefined ? formatCoins(row.net) : "—"}</td>
              <td className="time-reason">{row.reason}</td>
              <td>
                <span className="time-queue-actions">
                  {!row.untradeable ? <Link className={`btn${row.rec === "sell" ? " primary" : " ghost"}`} to="/capital?tab=vendas">vender</Link> : null}
                  <Link className={`btn${row.rec === "sbc" ? " primary" : " ghost"}`} to="/capital?tab=sbcs">SBC</Link>
                  <button type="button" className={`btn ghost${row.card.protected ? " protected" : ""}`} aria-pressed={row.card.protected} disabled={busy === player.id} onClick={() => toggleProtect(row)}>{row.card.protected ? "protegida" : "proteger"}</button>
                </span>
              </td>
            </tr>;
          })}</tbody>
        </table>
      </div>}
      {queue.length > visible ? <div className="time-queue-more"><span>Mostrando {visible} de {queue.length} cartas.</span><button type="button" className="btn ghost" onClick={() => setVisible((count) => count + QUEUE_PAGE)}>mostrar mais</button></div> : null}
    </div>
  </section>;
}
