import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { appendFeedback, fetchCollection, fetchFormations, fetchTime } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import GGRating, { formatGGRating } from "../components/GGRating";
import { CardArt, FutCardFace, FutLegend, FutPitch, formationLayout, ratingScale, spotStyle, type FutTone } from "../components/FutPitch";
import PageHeader from "../components/PageHeader";
import TrendChart from "../components/TrendChart";
import type { Filter } from "../odata";
import { useData } from "../useData";
import { useCollection } from "../useCollection";
import { evaluationSourceLabel, formatCoins, formatDate, formatSigned } from "../format";
import type {
  ChemistryResult,
  LeituraDoBot,
  PositionMapRow,
  ReservasCollection,
  RosterCard,
  SlotOutlook,
  StarterCard,
  TopMove,
} from "../types";
import TimeLeitura from "./TimeLeitura";
import TimeLimpeza from "./TimeLimpeza";
import TimePlanejador from "./TimePlanejador";
import "../shared.css";
import "./Time.css";

const VIEW_KEY = "eafc-bot:time-view";

// Time é a tela "/time": o XI titular (posição do slot físico — pode
// divergir da posição natural da carta, ver domain.SquadSlot) no campo FC 27
// ou em tabela, as três leituras ao lado (jogada de hoje, mapa de posições,
// química), o planejador nota × química, as reservas, a leitura do elenco
// em gráficos e a limpeza do clube. O campo usa o catálogo local de formações;
// os dados do elenco vêm de /api/time, do planejador e das coleções do clube.
export default function Time() {
  const [search, setSearch] = useState("");
  const [position, setPosition] = useState("");
  const [tradeable, setTradeable] = useState<"all" | "tradeable" | "untradeable">("all");
  const { data, error, loading, refetch } = useData(fetchTime, []);
  const formationsData = useData(fetchFormations, []);
  const startersData = useData(() => fetchCollection<StarterCard>("/api/elenco/titulares", { top: 50 }), []);
  const benchCollection = useCollection<RosterCard>("/api/elenco/reservas", { pageSize: 24 });
  const location = useLocation();
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
  // /time/planos e /time/insights viraram seções desta tela: os redirects
  // chegam com #planejador / #limpeza, que só existem depois do carregamento.
  const loaded = !!data && !!startersData.data;
  useEffect(() => {
    if (!loaded || !location.hash) return;
    document.getElementById(location.hash.slice(1))?.scrollIntoView({ block: "start" });
  }, [loaded, location.hash]);

  if (gate) return gate;
  if (!data) return null;

  const starters = startersData.data?.value ?? [];
  const improved = data.optimization?.status === "improved" && (data.optimization.moves?.length ?? 0) > 0;
  const suggested = improved ? starters.map((s) => data.optimization.moves.find((m) => m.index === s.index)?.suggested ?? s) : starters;
  const showingSuggested = lineup === "sugerida" && improved;
  const displayedStarters = showingSuggested ? suggested : starters;
  const bench = benchCollection.rows;
  const reservas = benchCollection.raw as ReservasCollection | null;
  const priceSeries = reservas?.["@eafc.price_series"] ?? {};
  const priceStatus = reservas?.["@eafc.price_history_status"] ?? {};
  const positionMap = data.position_map ?? [];
  const slotOutlook = data.slot_outlook ?? [];
  const sourceLabel = evaluationSourceLabel(data.avaliacao?.fonte);
  const compactLabel = evaluationSourceLabel(data.avaliacao?.fonte, true);
  const weakestIndex = positionMap.length > 0 ? positionMap.reduce((min, row) => (row.rating < min.rating ? row : min), positionMap[0]!).index : undefined;
  const layout = formationLayout(data.formation || "", displayedStarters, formationsData.data?.value ?? []);
  const spots = layout?.spots;
  const showPitch = !!layout && view === "campo";
  const ordered = layout?.ordered ?? displayedStarters.slice().sort((a, b) => a.index - b.index);
  // Tom e selo só valem para o XI atual: o elo mais fraco e o "upgrade
  // disponível" foram calculados para ele, não para a sugestão.
  const outlookByIndex = new Map(slotOutlook.map((o) => [o.index, o]));
  const toneOf = (starter: StarterCard): FutTone => {
    if (showingSuggested) return "turf";
    if (starter.index === weakestIndex) return "cost";
    return outlookByIndex.get(starter.index)?.kind === "melhor_disponivel" ? "alert" : "turf";
  };
  const applyBenchFilters = (nextPosition: string, nextTradeable: typeof tradeable) => {
    const filters: Filter[] = [];
    if (nextPosition) filters.push({ kind: "compare", field: "player/position", op: "eq", value: nextPosition });
    if (nextTradeable !== "all") filters.push({ kind: "compare", field: "player/untradeable", op: "eq", value: nextTradeable === "untradeable" });
    const filter = filters.reduce<Filter | undefined>((result, item) => result ? { kind: "and", left: result, right: item } : item, undefined);
    benchCollection.setFilter(filter);
    benchCollection.setPage(1);
  };

  return (
    <div className="wrap time-page">
      <PageHeader
        eyebrow="Elenco / Squad"
        title="Meu time"
        meta={`formação ${data.formation || "—"} · ${starters.length} titulares · ${benchCollection.count} reservas${data.avaliacao?.ciclo ? ` · ciclo fc${data.avaliacao.ciclo}` : ""}`}
        actions={
          <div className="time-actions">
            <div className="toggle">
              <button type="button" className={view === "campo" ? "active" : ""} onClick={() => layout && setView("campo")} disabled={!layout} title={!layout ? "A formação ainda não tem 11 slots reconhecidos para desenhar o campo" : undefined}>
                campo{!spots ? " indisponível" : ""}
              </button>
              <button type="button" className={view === "tabela" ? "active" : ""} onClick={() => setView("tabela")}>tabela</button>
            </div>
            {improved && <button type="button" className={`btn${showingSuggested ? " primary" : ""}`} aria-pressed={showingSuggested} onClick={() => setLineup(showingSuggested ? "atual" : "sugerida")}>melhor encaixe {formatSigned(data.optimization.gain)}</button>}
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
            <span>Titulares <span className="panel-head-sub">/ {showingSuggested ? "melhor encaixe" : "Starting XI"}</span></span>
            <span className="panel-head-meta">{data.formation || "formação —"} · {compactLabel} na vaga</span>
          </div>
          {view === "campo" && !layout && <p className="hint">{formationsData.error ? "Catálogo de formações indisponível; a tabela mostra o mesmo XI." : formationsData.loading ? "Carregando o desenho da formação…" : "Formação desconhecida ou sem 11 vagas; a tabela mostra o mesmo XI."}</p>}
          {showPitch ? <>
            <FutPitch formation={data.formation} lines={layout!.lines} dense={layout!.dense} className="time-pitch" label="Titulares no campo">
              {ordered.map((starter) => {
                const tone = toneOf(starter);
                const outlook = outlookByIndex.get(starter.index);
                const badge = tone === "cost" ? "elo mais fraco" : tone === "alert" ? `upgrade ${formatSigned(outlook?.delta ?? 0)}` : null;
                const score = starter.position_rating_unavailable ? undefined : starter.position_gg_rating;
                const chem = starter.chemistry;
                const face = <FutCardFace player={starter.player} position={starter.position} score={score} pips={chem ? (chem.fora_de_posicao ? 0 : chem.pontos) : undefined} scoreTone={tone === "cost" ? "cost" : tone === "alert" ? "alert" : ""} badge={badge} />;
                const label = `${starter.player.common_name || starter.player.name}, ${starter.position}, ${score !== undefined ? `${compactLabel} ${score.toFixed(1)}` : `${compactLabel} indisponível`}${badge ? `, ${badge}` : ""}${chem ? (chem.fora_de_posicao ? ", fora de posição" : `, entrosamento ${chem.pontos}/3`) : ""}`;
                return <div className="fut-slot" style={spotStyle(spots!.get(starter.index))} key={starter.player.club_item_id || `${starter.index}-${starter.player.id}`}>
                  {starter.card_slug
                    ? <Link to={`/time/${starter.card_slug}`} className={`fut-card tone-${tone}`} aria-label={label}>{face}</Link>
                    : <div className={`fut-card tone-${tone}`} title={label}>{face}</div>}
                </div>;
              })}
            </FutPitch>
            <FutLegend items={showingSuggested ? [["turf", "escalação sugerida pelo melhor encaixe"]] : [["turf", "acima da média do XI"], ["alert", "upgrade disponível"], ["cost", `menor ${sourceLabel} na vaga`]]} />
          </> : <StartersTable starters={ordered} regua={data.regua} outlook={outlookByIndex} toneOf={toneOf} sourceLabel={compactLabel} />}
          {improved && (
            <div className="panel-body time-fit-note">
              <strong>Melhor encaixe: {formatSigned(data.optimization.gain)} {sourceLabel} na vaga</strong>
              {data.optimization.chemistry_note && <><br />{data.optimization.chemistry_note}</>}
            </div>
          )}
        </div>

        <div className="time-side">
          <TopMoveCard move={data.top_move} />
          <PositionMapCard rows={positionMap} regua={data.regua} outlook={slotOutlook} weakestIndex={weakestIndex} sourceLabel={compactLabel} />
          <ChemistryCard chem={data.chemistry} starters={starters} />
        </div>
      </div>

      <TimePlanejador regua={data.regua} />

      {(benchCollection.count > 0 || search || position || tradeable !== "all") && (
        <section className="time-section" id="reservas">
          <div className="section-title-row">
            <div><h2>Reservas</h2><p className="section-note">Todo o clube fora do XI, ordenado por GG atual.</p></div>
            <span className="count-label">{benchCollection.count} encontradas</span>
          </div>
          <div className="roster-filters" aria-label="Filtrar reservas">
            <label><span>Buscar</span><input value={search} onChange={(e) => { const value = e.target.value; setSearch(value); benchCollection.setSearch(value); }} placeholder="Nome da carta" /></label>
            <label><span>Posição</span><select value={position} onChange={(e) => { const value = e.target.value; setPosition(value); applyBenchFilters(value, tradeable); }}><option value="">Todas</option>{["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"].map((p) => <option key={p}>{p}</option>)}</select></label>
            <label><span>Status</span><select value={tradeable} onChange={(e) => { const value = e.target.value as typeof tradeable; setTradeable(value); applyBenchFilters(position, value); }}><option value="all">Todas</option><option value="tradeable">Negociáveis</option><option value="untradeable">Inegociáveis</option></select></label>
          </div>
          <BenchTable rows={bench} priceSeries={priceSeries} priceStatus={priceStatus} sourceLabel={sourceLabel} />
          <Pagination page={benchCollection.page} pages={benchCollection.pages} onPage={benchCollection.setPage} />
        </section>
      )}

      <TimeLeitura
        starters={starters}
        formation={data.formation}
        distribution={data.distribuicao_ovr ?? []}
        valorXI={data.valor_xi}
        outlook={slotOutlook}
        positionMap={positionMap}
        weakestIndex={weakestIndex}
      />

      <TimeLimpeza />
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
            <div className="top-move-headline">
              <span className="top-move-arrow" aria-hidden="true"><i className="from" /><svg viewBox="0 0 24 24"><path d="M4 12h16m-5-5 5 5-5 5" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg><i className="to" /></span>
              <span>{move.headline}<small>vaga {move.slot}</small></span>
            </div>
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

function PositionMapCard({ rows, regua, outlook, weakestIndex, sourceLabel }: { rows: PositionMapRow[]; regua: number; outlook: SlotOutlook[]; weakestIndex?: number; sourceLabel: string }) {
  const byIndex = new Map(outlook.map((o) => [o.index, o]));
  const scale = ratingScale(regua > 0 ? [...rows.map((row) => row.rating), regua] : rows.map((row) => row.rating));
  return (
    <div className="panel position-map-panel">
      <div className="panel-head">
        <span>Mapa de posições <span className="panel-head-sub">/ vs média do XI</span></span>
        <span className="panel-head-meta">média {sourceLabel} {regua > 0 ? regua.toFixed(1) : "—"}</span>
      </div>
      <div className="panel-body position-map-rows">
        {rows.length === 0 && <p className="hint">Sem notas suficientes de {sourceLabel} para montar o mapa.</p>}
        {rows.map((row) => {
          const o = byIndex.get(row.index);
          const label = outlookLabel(o);
          const tone = row.index === weakestIndex ? "cost" : o?.kind === "melhor_disponivel" ? "alert" : "turf";
          return (
            <div className={`position-map-row tone-${tone}`} key={row.index}>
              <span className="position-map-pos">{row.position}</span>
              <span className="position-map-bar">
                <span className="position-map-fill" style={{ width: `${Math.max(3, scale.pct(row.rating))}%` }} />
                {regua > 0 && <span className="position-map-ruler" style={{ left: `${scale.pct(regua)}%` }} />}
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
                  <span className="time-card-cell">
                    <CardArt player={p} className="time-thumb" />
                    {row.card_slug ? <Link to={`/time/${row.card_slug}`}>{p.common_name || p.name}</Link> : <span title="Abaixo do overall mínimo analisado carta a carta">{p.common_name || p.name}</span>}
                    {p.untradeable && <Chip tone="flat">untradeable</Chip>}
                  </span>
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

// StartersTable é o XI em tabela: a mesma leitura do campo (tom, nota na
// vaga, teto) em linhas — melhor pra varrer números em série. A nota da
// vaga vem com o GG atual da carta ao lado (GGRating inline) quando os dois
// diferem: é a mesma carta vista em dois contextos, e esconder um deles faz
// uma carta 99,0 parecer acima de uma vaga em que ela rende 98,3. A química
// por carta só aparece aqui, nunca no banco: lá p.chemistry é o valor cru
// que o fut.gg persiste por carta, sobra de escalações passadas.
function StartersTable({ starters, regua, outlook, toneOf, sourceLabel }: { starters: StarterCard[]; regua: number; outlook: Map<number, SlotOutlook>; toneOf: (starter: StarterCard) => FutTone; sourceLabel: string }) {
  const rated = starters.filter((s) => !s.position_rating_unavailable && s.position_gg_rating !== undefined).map((s) => s.position_gg_rating!);
  const scale = ratingScale(regua > 0 ? [...rated, regua] : rated);
  return (
    <div className="tablewrap flush">
      <table>
        <thead>
          <tr>
            <th>Vaga</th>
            <th>Carta</th>
            <th className="num">OVR</th>
            <th className="num">{sourceLabel} na vaga</th>
            <th>vs média</th>
            <th>Química</th>
            <th className="num">Teto</th>
          </tr>
        </thead>
        <tbody>
          {starters.map((s) => {
            const p = s.player;
            const tone = toneOf(s);
            const score = s.position_rating_unavailable ? undefined : s.position_gg_rating;
            const teto = outlookLabel(outlook.get(s.index));
            return (
              <tr key={p.club_item_id || `${p.id}-${s.index}`}>
                <td><span className={`time-pos tone-${tone}`}>{s.position}</span></td>
                <td className="namecell">
                  <span className="time-card-cell">
                    <CardArt player={p} className={`time-thumb tone-${tone}`} />
                    {s.card_slug ? <Link to={`/time/${s.card_slug}`}>{p.common_name || p.name}</Link> : <span title="Abaixo do overall mínimo analisado carta a carta">{p.common_name || p.name}</span>}
                    {p.untradeable && <Chip tone="flat">untradeable</Chip>}
                  </span>
                </td>
                <td className="num">{p.rating}</td>
                <td className={`num time-score tone-${tone}`}>{score !== undefined ? <GGRating current={p.gg_rating} currentPosition={p.gg_rating_pos} positional={score} positionalPosition={s.position} positionalLabel={sourceLabel} variant="inline" /> : "—"}</td>
                <td className="time-vs"><span className="time-vs-track"><span className={`tone-${tone}`} style={{ width: `${score !== undefined ? Math.max(3, scale.pct(score)) : 0}%` }} />{regua > 0 && <i style={{ left: `${scale.pct(regua)}%` }} />}</span></td>
                <td className="time-chem">
                  {s.chemistry ? (
                    s.chemistry.fora_de_posicao ? <Chip tone="alert">fora de posição</Chip> : <span title={`base ${s.chemistry.base} · clube ${s.chemistry.clube} · liga ${s.chemistry.liga} · nação ${s.chemistry.nacao}`}>{s.chemistry.pontos}/3</span>
                  ) : "—"}
                </td>
                <td className={`num time-teto ${teto.tone}`}>{teto.text}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
