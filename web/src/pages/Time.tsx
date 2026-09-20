import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { appendFeedback, fetchCollection, fetchTime } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import GGRating, { formatGGRating } from "../components/GGRating";
import PageHeader from "../components/PageHeader";
import Pitch, { canDrawPitch } from "../components/Pitch";
import TrendChart from "../components/TrendChart";
import type { Filter } from "../odata";
import { useData } from "../useData";
import { useCollection } from "../useCollection";
import { evaluationSourceLabel, formatCoins, formatDate, formatSigned } from "../format";
import type {
  ChemistryPlayer,
  ChemistryResult,
  ClubPlayer,
  LeituraDoBot,
  Position,
  PositionMapRow,
  ReservasCollection,
  RosterCard,
  SlotOutlook,
  StarterCard,
  TopMove,
} from "../types";
import "../shared.css";
import "./Time.css";

const VIEW_KEY = "eafc-bot:time-view";

// Time é a tela "/time": o XI titular (posição do slot físico — pode
// divergir da posição natural da carta, ver domain.SquadSlot) e o banco.
// O titular padrão é o campo desenhado na formação de verdade; a tabela
// continua disponível pelo toggle — melhor pra varrer números em série.
// À direita do campo ficam três leituras derivadas do MESMO snapshot:
// jogada de hoje, mapa de posições e química — nenhuma delas busca dado
// próprio, todas vêm de /api/time.
export default function Time() {
  const [, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const [tradeable, setTradeable] = useState<"all" | "tradeable" | "untradeable">("all");
  const { data, error, loading, refetch } = useData(fetchTime, []);
  const startersData = useData(() => fetchCollection<StarterCard>("/api/elenco/titulares", { top: 50 }), []);
  const benchCollection = useCollection<RosterCard>("/api/elenco/reservas", { pageSize: 24 });
  const gate = asyncGate(loading || startersData.loading || benchCollection.loading, error ?? startersData.error ?? benchCollection.error, !!data && !!startersData.data, () => { refetch(); startersData.refetch(); benchCollection.refetch(); });

  const [view, setView] = useState<"campo" | "tabela">(() => {
    try {
      return localStorage.getItem(VIEW_KEY) === "tabela" ? "tabela" : "campo";
    } catch {
      return "campo";
    }
  });
  const [lineup, setLineup] = useState<"atual" | "sugerida">("atual");
  useEffect(() => {
    try {
      localStorage.setItem(VIEW_KEY, view);
    } catch {
      // Sem storage disponível (navegação privada, política do navegador)
      // a tela ainda funciona — só esquece a preferência ao recarregar.
    }
  }, [view]);

  if (gate) return gate;
  if (!data) return null;

  const starters = startersData.data?.value ?? [];
  const suggested = data.optimization?.moves?.length ? starters.map((s) => data.optimization.moves.find((m) => m.index === s.index)?.suggested ?? s) : starters;
  const displayedStarters = lineup === "sugerida" ? suggested : starters;
  const bench = benchCollection.rows;
  const reservas = benchCollection.raw as ReservasCollection | null;
  const priceSeries = reservas?.["@eafc.price_series"] ?? {};
  const priceStatus = reservas?.["@eafc.price_history_status"] ?? {};
  const positionMap = data.position_map ?? [];
  const slotOutlook = data.slot_outlook ?? [];
  const sourceLabel = evaluationSourceLabel(data.avaliacao?.fonte);
  const weakestIndex = positionMap.length > 0 ? positionMap.reduce((min, row) => (row.rating < min.rating ? row : min), positionMap[0]!).index : undefined;
  const pitchOK = canDrawPitch(data.formation || "", starters.length);
  const showPitch = pitchOK && view === "campo";
  const applyBenchFilters = (nextPosition: string, nextTradeable: typeof tradeable) => {
    const filters: Filter[] = [];
    if (nextPosition) filters.push({ kind: "compare", field: "player/position", op: "eq", value: nextPosition });
    if (nextTradeable !== "all") filters.push({ kind: "compare", field: "player/untradeable", op: "eq", value: nextTradeable === "untradeable" });
    const filter = filters.reduce<Filter | undefined>((result, item) => result ? { kind: "and", left: result, right: item } : item, undefined);
    benchCollection.setFilter(filter);
    benchCollection.setPage(1);
    setPage(1);
  };

  return (
    <div className="wrap time-page">
      <PageHeader
        eyebrow="Elenco / Squad"
        title="Meu time"
        meta={`formação ${data.formation || "—"} · ${starters.length} titulares · ${benchCollection.count} reservas`}
        actions={
          <div className="toggle">
            <button className={view === "campo" ? "active" : ""} onClick={() => pitchOK && setView("campo")} disabled={!pitchOK} title={!pitchOK ? "A formação ainda não tem 11 slots reconhecidos para desenhar o campo" : undefined}>
              campo{!pitchOK ? " indisponível" : ""}
            </button>
            <button className={view === "tabela" ? "active" : ""} onClick={() => setView("tabela")}>
              tabela
            </button>
            {data.optimization?.status === "improved" && <button className={lineup === "sugerida" ? "active" : ""} onClick={() => setLineup(lineup === "sugerida" ? "atual" : "sugerida")}>melhor encaixe</button>}
          </div>
        }
      />

      {data.chemistry && data.chemistry.fora_de_posicao > 0 && (
        <div className="banner alert">
          {data.chemistry.fora_de_posicao} titular{data.chemistry.fora_de_posicao > 1 ? "es" : ""} fora de posição — zera o entrosamento dele e tira o vínculo dos outros também (única forma de perder química hoje).
        </div>
      )}

      <div className="time-grid">
        <div className="panel time-pitch-panel">
          <div className="panel-head">
            <span>Titulares <span className="panel-head-sub">/ Starting XI</span></span>
          </div>
          {showPitch ? (
            <Pitch formation={data.formation} starters={displayedStarters} sourceLabel={sourceLabel} outlook={slotOutlook} weakestIndex={weakestIndex} />
          ) : (
            <div className="panel-body">
              <RosterTable rows={displayedStarters.map((s) => ({ player: s.player, cardSlug: s.card_slug, position: s.position, positionalGGRating: s.position_gg_rating, chemistry: s.chemistry }))} sourceLabel={sourceLabel} showChemistry />
            </div>
          )}
          {showPitch && (
            <div className="pitch-legend">
              <span><i className="tone-turf" /> acima da média do XI</span>
              <span><i className="tone-alert" /> upgrade disponível</span>
              <span><i className="tone-cost" /> menor {sourceLabel} na vaga</span>
            </div>
          )}
          {data.optimization?.status === "improved" && (
            <div className="panel-body top-border">
              <strong>Melhor encaixe: +{data.optimization.gain.toFixed(1)} {sourceLabel} na vaga</strong>
              {data.optimization.chemistry_note && <><br />{data.optimization.chemistry_note}</>}
            </div>
          )}
        </div>

        <div className="time-side">
          <TopMoveCard move={data.top_move} />
          <PositionMapCard rows={positionMap} regua={data.regua} outlook={slotOutlook} sourceLabel={sourceLabel} />
          <ChemistryCard chem={data.chemistry} starters={starters} />
        </div>
      </div>

      {(benchCollection.count > 0 || search || position || tradeable !== "all") && (
        <section>
          <div className="section-title-row">
            <div><h2>Reservas</h2><p className="section-note">Todo o clube fora do XI, ordenado por GG atual.</p></div>
            <span className="count-label">{benchCollection.count} encontradas</span>
          </div>
          <div className="roster-filters" aria-label="Filtrar reservas">
            <label><span>Buscar</span><input value={search} onChange={(e) => { const value = e.target.value; setSearch(value); benchCollection.setSearch(value); setPage(1); }} placeholder="Nome da carta" /></label>
            <label><span>Posição</span><select value={position} onChange={(e) => { const value = e.target.value; setPosition(value); applyBenchFilters(value, tradeable); }}><option value="">Todas</option>{["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"].map((p) => <option key={p}>{p}</option>)}</select></label>
            <label><span>Status</span><select value={tradeable} onChange={(e) => { const value = e.target.value as typeof tradeable; setTradeable(value); applyBenchFilters(position, value); }}><option value="all">Todas</option><option value="tradeable">Negociáveis</option><option value="untradeable">Inegociáveis</option></select></label>
          </div>
          <BenchTable rows={bench} priceSeries={priceSeries} priceStatus={priceStatus} sourceLabel={sourceLabel} />
          <Pagination page={benchCollection.page} pages={benchCollection.pages} onPage={benchCollection.setPage} />
        </section>
      )}
    </div>
  );
}

function TopMoveCard({ move }: { move?: TopMove }) {
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
    <div className="panel top-move-panel">
      <div className="panel-head">
        <span>Jogada de hoje <span className="panel-head-sub">/ Top move</span></span>
        {move?.kind === "upgrade" && move.efficiency ? <span className="panel-head-meta">eficiência {move.efficiency.toFixed(2)}</span> : null}
      </div>
      <div className="panel-body">
        {!move ? (
          <p className="hint">Nenhum upgrade de mercado nem evolução passou do ganho mínimo hoje.</p>
        ) : (
          <>
            <div className="top-move-headline">{move.headline}</div>
            <div className="top-move-stats">
              <div><span>Ganho</span><strong className="up">{formatSigned(move.gain)}</strong></div>
              {move.kind === "upgrade" ? (
                <>
                  <div><span>Líquido</span><strong className="coin">{formatCoins(move.net_cost)}</strong></div>
                  {move.profit ? (
                    <div><span>Sobra</span><strong className="up">{formatCoins(move.profit)}</strong></div>
                  ) : (
                    <div><span>Compra</span><strong>{formatCoins(move.gross_cost ?? 0)}</strong></div>
                  )}
                </>
              ) : (
                <div><span>Custo</span><strong className="coin">{move.net_cost > 0 ? formatCoins(move.net_cost) : "grátis"}</strong></div>
              )}
            </div>
            {move.rationale?.[0] && <p className="top-move-rationale">{move.rationale[0]}</p>}
            <div className="top-move-actions">
              <Link className="btn primary" to={move.link}>{move.kind === "upgrade" ? "abrir no mercado" : "ver evolução"}</Link>
              {sent ? <span className="hint">descartado</span> : <button className="btn ghost" type="button" onClick={descartar} disabled={sending}>descartar</button>}
            </div>
          </>
        )}
      </div>
    </div>
  );
}

// pct mapeia GG Rating (tipicamente 70-99 no XI) numa barra 0-100% — uma
// faixa razoável pra dar contraste visual entre titulares, não uma escala
// oficial de nenhum lugar.
function pct(rating: number): number {
  return Math.max(2, Math.min(100, ((rating - 70) / (99 - 70)) * 100));
}

function outlookLabel(o?: SlotOutlook): { text: string; tone: "up" | "" } {
  if (!o) return { text: "—", tone: "" };
  switch (o.kind) {
    case "melhor_disponivel":
      return { text: formatSigned(o.delta ?? 0), tone: "up" };
    case "sem_cotacao":
      return { text: "s/ cotação", tone: "" };
    case "teto":
      return { text: "teto", tone: "" };
    default:
      return { text: "—", tone: "" };
  }
}

function PositionMapCard({ rows, regua, outlook, sourceLabel }: { rows: PositionMapRow[]; regua: number; outlook: SlotOutlook[]; sourceLabel: string }) {
  const byIndex = new Map(outlook.map((o) => [o.index, o]));
  const rulerPct = regua > 0 ? pct(regua) : 0;
  return (
    <div className="panel position-map-panel">
      <div className="panel-head">
        <span>Mapa de posições <span className="panel-head-sub">/ vs média do XI</span></span>
        <span className="panel-head-meta">média {sourceLabel} {regua > 0 ? regua.toFixed(1) : "—"}</span>
      </div>
      <div className="panel-body position-map-rows">
        {rows.length === 0 && <p className="hint">Sem notas suficientes de {sourceLabel} para montar o mapa.</p>}
        {rows.map((row) => {
          const label = outlookLabel(byIndex.get(row.index));
          return (
            <div className="position-map-row" key={row.index}>
              <span className="position-map-pos">{row.position}</span>
              <span className="position-map-bar">
                <span className="position-map-fill" style={{ width: `${pct(row.rating)}%` }} />
                {regua > 0 && <span className="position-map-ruler" style={{ left: `${rulerPct}%` }} />}
              </span>
              <span className="position-map-value">{row.rating.toFixed(1)}</span>
              <span className={`position-map-outlook ${label.tone}`}>{label.text}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function topGroup(starters: StarterCard[], field: "club" | "league" | "nation"): { name: string; count: number } | null {
  const counts = new Map<string, number>();
  for (const s of starters) {
    const value = s.player[field];
    if (!value) continue;
    counts.set(value, (counts.get(value) ?? 0) + 1);
  }
  let best: { name: string; count: number } | null = null;
  for (const [name, count] of counts) {
    if (!best || count > best.count) best = { name, count };
  }
  return best;
}

function chemistryNoteText(chem: ChemistryResult): string {
  if (chem.fora_de_posicao > 0) {
    return `${chem.fora_de_posicao} titular${chem.fora_de_posicao > 1 ? "es" : ""} fora de posição — zera o entrosamento dele.`;
  }
  if (chem.verificacao.status === "diverge") {
    return `O modelo não confere com o jogo (calculado ${chem.verificacao.calculado}, o jogo reporta ${chem.verificacao.observado}) — rode \`eafcbot quimica -calibrar\`.`;
  }
  if (chem.nao_modelado?.length) {
    return `${chem.nao_modelado.length} carta(s) fora do modelo de química (Icon/Hero) — não entram na conta.`;
  }
  return "Ninguém fora de posição — o entrosamento ainda depende dos vínculos de clube, liga e nação.";
}

function ChemistryCard({ chem, starters }: { chem?: ChemistryResult; starters: StarterCard[] }) {
  const club = topGroup(starters, "club");
  const league = topGroup(starters, "league");
  const nation = topGroup(starters, "nation");
  const jogadoresByIndex = new Map((chem?.jogadores ?? []).map((j) => [j.index, j]));
  return (
    <div className="panel chemistry-panel">
      <div className="panel-head">
        <span>Química <span className="panel-head-sub">/ Chemistry</span></span>
        {chem && <span className="panel-head-meta">{chem.total} / {chem.maximo}</span>}
      </div>
      <div className="panel-body">
        {!chem ? (
          <p className="hint">Escalação não sincronizada — sem entrosamento calculado.</p>
        ) : (
          <>
            <div className="chem-segments">
              {starters.slice().sort((a, b) => a.index - b.index).map((s) => {
                const j = jogadoresByIndex.get(s.index);
                const label = `${s.player.common_name || s.player.name}: ${j ? (j.fora_de_posicao ? "fora de posição" : `${j.pontos}/3`) : "sem dado"}`;
                return <span key={s.index} className={j?.fora_de_posicao ? "out" : "ok"} title={label} />;
              })}
            </div>
            <div className="chem-groups">
              <div><span>Clube</span><strong>{club ? `${club.name} ×${club.count}` : "—"}</strong></div>
              <div><span>Liga</span><strong>{league ? `${league.name} ×${league.count}` : "—"}</strong></div>
              <div><span>Nação</span><strong>{nation ? `${nation.name} ×${nation.count}` : "—"}</strong></div>
            </div>
            <p className="chem-note">{chemistryNoteText(chem)}</p>
          </>
        )}
      </div>
    </div>
  );
}

function leituraText(l: LeituraDoBot | undefined, sourceLabel: string): string {
  if (!l || !l.kind) return "—";
  switch (l.kind) {
    case "evoluir":
      return `Evolução leva a ${(l.final_gg_rating ?? 0).toFixed(1)} GG por ${formatCoins(l.coins_cost ?? 0)}`;
    case "vender_caindo":
      return `Caindo ${Math.abs(l.change_pct_30d ?? 0).toFixed(0)}% em 30d e sem vaga no XI — vender agora`;
    case "vender":
      return "Sem vaga no XI e sem potencial de evolução — vender";
    case "promover":
      if (l.promocao?.metric === "metarank") return "Sem GG Rating confirmado para comparar nesta vaga";
      if (l.promocao) return `Escalar na ${l.promocao.position}, no lugar de ${l.promocao.starter_name} (${l.promocao.candidate_rating.toFixed(1)} vs ${l.promocao.starter_rating.toFixed(1)} ${promotionMetricLabel(l.promocao.metric, sourceLabel)} na vaga)`;
      return "Promoção apontada, mas faltam notas por vaga para confirmar a troca";
    case "fodder":
      return l.sbc_name ? `Fodder de SBC: cobre "${l.sbc_name}"` : "Fodder de SBC sem custo de oportunidade";
    case "nao_vendavel":
      return l.sbc_name ? `Untradeable — cobre "${l.sbc_name}"` : "Untradeable — serve como fodder sem custo de oportunidade";
    case "aguardar_verificacao":
      return "Evolução ainda não verificada nesta coleta";
    default:
      return "—";
  }
}

function promotionMetricLabel(metric: NonNullable<LeituraDoBot["promocao"]>["metric"], sourceLabel: string): string {
  switch (metric) {
    case "gg_rating_card": return "GG Rating do FUT.GG";
    default: return sourceLabel;
  }
}

function BenchTable({ rows, priceSeries, priceStatus, sourceLabel }: { rows: RosterCard[]; priceSeries: Record<string, { coins: number; observed_at: string }[]>; priceStatus: Record<string, string>; sourceLabel: string }) {
  return (
    <div className="tablewrap">
      <table>
        <thead>
          <tr>
            <th>Pos</th>
            <th>Carta</th>
            <th className="num">OVR</th>
            <th className="num">GG</th>
            <th>Vaga sugerida</th>
            <th className="num">Preço</th>
            <th>30d</th>
            <th>Leitura do bot</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const p = row.player;
			const promocao = row.leitura?.promocao?.metric === "metarank" ? undefined : row.leitura?.promocao;
			const promotionLabel = promocao ? promotionMetricLabel(promocao.metric, sourceLabel) : sourceLabel;
            const series = (priceSeries[p.id] ?? []).map((pt) => ({ label: formatDate(pt.observed_at), value: pt.coins }));
            const status = priceStatus[p.id];
            return (
              <tr key={p.club_item_id || p.id}>
                <td>{p.position}{p.out_of_pos ? " *" : ""}</td>
                <td className="namecell">
                  {row.card_slug ? <Link to={`/time/${row.card_slug}`}>{p.common_name || p.name}</Link> : <span title="Abaixo do overall mínimo analisado carta a carta">{p.common_name || p.name}</span>}
                  {p.untradeable && <Chip tone="flat"> untradeable</Chip>}
                </td>
                <td className="num">{p.rating}</td>
                <td className="num">{formatGGRating(p.gg_rating)}</td>
                <td className="promotion-cell">
                  {promocao ? <>
                    <strong>{promocao.position} → {promocao.starter_name}</strong>
                    <span>{promocao.candidate_rating.toFixed(1)} vs {promocao.starter_rating.toFixed(1)} <b className="up">{formatSigned(promocao.gain)}</b> {promotionLabel} na vaga</span>
                  </> : "—"}
                </td>
                <td className="num coin">{p.price?.coins ? formatCoins(p.price.coins) : "—"}</td>
                <td>{series.length >= 2 ? <TrendChart data={series} compact height={16} /> : <span className="chart-empty-inline" title={status}>—</span>}</td>
                <td className="leitura-cell">{leituraText(row.leitura, sourceLabel)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {rows.length === 0 && <div className="empty">Nenhuma carta aqui.</div>}
    </div>
  );
}

function Pagination({ page, pages, onPage }: { page: number; pages: number; onPage: (next: number) => void }) {
  if (pages <= 1) return null;
  return <nav className="pagination" aria-label="Paginação de reservas"><span>Página {page} de {pages}</span><button disabled={page <= 1} onClick={() => onPage(page - 1)}>Anterior</button><button disabled={page >= pages} onClick={() => onPage(page + 1)}>Próxima</button></nav>;
}

interface Row {
  player: ClubPlayer;
  cardSlug?: string;
  position?: Position;
  positionalGGRating?: number;
  chemistry?: ChemistryPlayer;
}

// showChemistry só é true pra tabela de TITULARES: no banco, p.chemistry é o
// valor cru que o fut.gg persiste por carta, que sobra de escalações
// passadas (46 cartas do banco carregam chem>0 mesmo fora do XI ativo, num
// retrato real) — mostrar isso confundiria com o entrosamento calculado do
// XI de hoje, que só faz sentido pra quem está escalado.
function RosterTable({ rows, sourceLabel, showChemistry = false }: { rows: Row[]; sourceLabel: string; showChemistry?: boolean }) {
  return (
    <div className="tablewrap">
      <table>
        <thead>
          <tr>
            <th>Posição</th>
            <th>Carta</th>
            <th className="num">Overall</th>
            <th className="num">GG atual · {sourceLabel} na vaga</th>
            {showChemistry && <th>Química</th>}
          </tr>
        </thead>
        <tbody>
          {rows.map(({ player: p, cardSlug, position, positionalGGRating, chemistry }, index) => (
            <tr key={p.club_item_id || `${p.id}-${index}`}>
              <td>
                {position ?? p.position}
                {p.out_of_pos ? " *" : ""}
              </td>
              <td className="namecell">
                {cardSlug ? (
                  <Link to={`/time/${cardSlug}`}>{p.common_name || p.name}</Link>
                ) : (
                  <span title="Abaixo do overall mínimo analisado carta a carta">{p.common_name || p.name}</span>
                )}
                {p.untradeable && <Chip tone="flat"> untradeable</Chip>}
              </td>
              <td className="num">{p.rating}</td>
              <td className="num"><GGRating current={p.gg_rating} currentPosition={p.gg_rating_pos} positional={positionalGGRating} positionalPosition={position} positionalLabel={sourceLabel} variant="inline" /></td>
              {showChemistry && (
                <td>
                  {chemistry ? (
                    chemistry.fora_de_posicao ? (
                      <Chip tone="alert">fora de posição</Chip>
                    ) : (
                      <span title={`base ${chemistry.base} · clube ${chemistry.clube} · liga ${chemistry.liga} · nação ${chemistry.nacao}`}>
                        {chemistry.pontos}/3
                      </span>
                    )
                  ) : (
                    "—"
                  )}
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <div className="empty">Nenhuma carta aqui.</div>}
    </div>
  );
}
