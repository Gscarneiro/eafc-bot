import { useState } from "react";
import { Link } from "react-router-dom";
import { fetchFormations, fetchGauntlet } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import { CardArt, FutCardFace, FutLegend, FutPitch, formationLayout, ratingScale, spotStyle, type FutTone } from "../components/FutPitch";
import PageHeader from "../components/PageHeader";
import { evaluationSourceLabel, formatCoins, formatDate, formatDateTime, formatSigned, isZeroTime, styleNames } from "../format";
import { useData } from "../useData";
import type { EvoPotential, Formation, GauntletRoundView, GauntletStarterView } from "../types";
import "../shared.css";
import "./Gauntlet.css";

const VIEW_KEY = "eafc-bot:gauntlet-view";

// bestPotential é a rota de maior ganho de uma carta. As rotas vêm só com
// nota confirmada pelo fut.gg na MESMA posição da vaga (ver
// GauntletStarterView.potentials) — nunca uma estimativa.
function bestPotential(starter: GauntletStarterView): EvoPotential | undefined {
  return (starter.potentials ?? []).reduce<EvoPotential | undefined>((best, potential) => (!best || potential.gg_rating_gain > best.gg_rating_gain ? potential : best), undefined);
}

function potentialCost(potential: EvoPotential): { text: string; tone: "coin" | "cost" | "gain" } {
  if (potential.coin_cost > 0) return { text: `${formatCoins(potential.coin_cost)} moedas`, tone: "coin" };
  if (potential.point_cost > 0) return { text: `${potential.point_cost} pontos`, tone: "cost" };
  return { text: "grátis", tone: "gain" };
}

function readView(): "campo" | "tabela" {
  try { return localStorage.getItem(VIEW_KEY) === "tabela" ? "tabela" : "campo"; } catch { return "campo"; }
}

// Gauntlet é a tela "/time/gauntlet": quatro elencos consecutivos (11
// titulares + 7 reservas cada) sem repetir carta nenhuma entre eles — a
// regra oficial do modo (ver o aviso em data.rules). Todo o cálculo vem
// pronto de GET /api/gauntlet; esta tela só exibe o plano e a queda de
// força de uma rodada para a outra.
export default function Gauntlet() {
  const { data, error, loading, refetch } = useData(fetchGauntlet, []);
  const formationsData = useData(fetchFormations, []);
  const [roundIndex, setRoundIndex] = useState(0);
  const [openStarter, setOpenStarter] = useState<number | null>(null);
  const [view, setViewState] = useState<"campo" | "tabela">(readView);
  const setView = (next: "campo" | "tabela") => {
    setViewState(next);
    try { localStorage.setItem(VIEW_KEY, next); } catch { /* sem storage a tela funciona, só esquece a preferência */ }
  };

  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const rounds = data.rounds ?? [];
  const objectives = data.objectives ?? [];
  const activeIndex = Math.min(roundIndex, Math.max(rounds.length - 1, 0));
  const round = rounds[activeIndex];
  const insufficient = data.status !== "ok" || rounds.length === 0;
  const scoreLabel = evaluationSourceLabel(data.avaliacao?.fonte, true);
  const benchPerRound = rounds[0]?.bench?.length ?? 7;
  const meta = [`formação ${data.formation || "—"}`, rounds.length ? `${rounds.length} rodadas` : "", rounds.length ? `${rounds.length * (11 + benchPerRound)} cartas sem repetir` : "", `coletado em ${formatDateTime(data.generated_at)}`].filter(Boolean).join(" · ");
  const selectRound = (index: number) => { setRoundIndex(index); setOpenStarter(null); };

  return (
    <div className="wrap gauntlet-page">
      <PageHeader
        eyebrow="Elenco / Gauntlet"
        title="Gauntlet"
        meta={meta}
        actions={insufficient ? undefined : <div className="toggle">
          <button type="button" className={view === "campo" ? "active" : ""} onClick={() => setView("campo")}>campo</button>
          <button type="button" className={view === "tabela" ? "active" : ""} onClick={() => setView("tabela")}>tabela</button>
        </div>}
      />
      <div className="gauntlet-rule"><span>regra</span><p>{data.rules}</p></div>
      {insufficient ? (
        <EmptyState
          message={data.reason || "Elenco insuficiente para montar o Gauntlet."}
          hint="O Gauntlet precisa de 72 cartas cobertas pela fonte ativa (44 titulares + 28 reservas) e da escalação titular sincronizada."
        />
      ) : (
        <>
          <RoundTabs rounds={rounds} activeIndex={activeIndex} onSelect={selectRound} />
          {round && <div className="gauntlet-grid">
            <div className="gauntlet-col">
              <StartersPanel round={round} formation={data.formation} catalog={formationsData.data?.value ?? []} catalogState={formationsData.error ? "erro" : formationsData.loading ? "carregando" : "pronto"} view={view} scoreLabel={scoreLabel} openStarter={openStarter} onToggle={(index) => setOpenStarter(openStarter === index ? null : index)} />
              <BenchPanel round={round} />
            </div>
            <div className="gauntlet-col">
              {(data.warnings?.length ?? 0) > 0 && <div className="banner alert gauntlet-warnings"><ul>{(data.warnings ?? []).map((warning) => <li key={warning}>{warning}</li>)}</ul></div>}
              <DropPanel rounds={rounds} activeIndex={activeIndex} onSelect={selectRound} benchPerRound={benchPerRound} />
              <EvolutionPanel round={round} openStarter={openStarter} onToggle={(index) => setOpenStarter(openStarter === index ? null : index)} />
              {objectives.length > 0 && <div className="panel">
                <div className="panel-head"><span>Objetivos</span><span className="panel-head-meta">Gauntlet</span></div>
                {objectives.map((objective) => <div className="gauntlet-objective" key={objective.id}>
                  <span><strong>{objective.name}</strong>{(objective.tasks?.length ?? 0) > 0 && <span>{(objective.tasks ?? []).join(" · ")}</span>}</span>
                  <small>{!isZeroTime(objective.expires_at) ? `expira ${formatDate(objective.expires_at)}` : objective.group}</small>
                </div>)}
              </div>}
            </div>
          </div>}
        </>
      )}
    </div>
  );
}

function RoundTabs({ rounds, activeIndex, onSelect }: { rounds: GauntletRoundView[]; activeIndex: number; onSelect: (index: number) => void }) {
  const base = rounds[0]?.average_rating ?? 0;
  const scale = ratingScale(rounds.map((round) => round.average_rating));
  return <div className="gauntlet-rounds" role="tablist" aria-label="Rodadas do Gauntlet">
    {rounds.map((round, index) => {
      const delta = round.average_rating - base;
      return <button key={round.round} type="button" role="tab" aria-selected={index === activeIndex} className={index === activeIndex ? "active" : ""} onClick={() => onSelect(index)}>
        <span className="gauntlet-round-head"><span>rodada {round.round}</span><span className={index === 0 ? "" : delta < -0.05 ? "down" : delta > 0.05 ? "up" : ""}>{index === 0 ? "base" : formatSigned(delta)}</span></span>
        <strong>{round.average_rating.toFixed(1)}</strong>
        <span className="gauntlet-round-bar"><span style={{ width: `${scale.pct(round.average_rating)}%` }} /></span>
        <span className="gauntlet-round-meta">{round.chemistry ? `química ${round.chemistry.total}/${round.chemistry.maximo}` : "química —"} · força {round.total_rating.toFixed(0)}</span>
      </button>;
    })}
  </div>;
}

type StarterView = { starter: GauntletStarterView; tone: FutTone; weakest: boolean; potential?: EvoPotential };

function starterViews(round: GauntletRoundView): StarterView[] {
  const starters = round.starters ?? [];
  const minRating = starters.length ? Math.min(...starters.map((starter) => starter.rating)) : undefined;
  return starters.map((starter) => {
    const potential = bestPotential(starter);
    const weakest = starter.rating === minRating;
    return { starter, potential, weakest, tone: weakest ? "cost" : potential ? "alert" : "turf" };
  });
}

function StartersPanel({ round, formation, catalog, catalogState, view, scoreLabel, openStarter, onToggle }: { round: GauntletRoundView; formation: string; catalog: Formation[]; catalogState: "erro" | "carregando" | "pronto"; view: "campo" | "tabela"; scoreLabel: string; openStarter: number | null; onToggle: (index: number) => void }) {
  const views = starterViews(round);
  const byIndex = new Map(views.map((item) => [item.starter.index, item]));
  const layout = formationLayout(formation || "", round.starters ?? [], catalog);
  const spots = layout?.spots;
  const chemistryByIndex = new Map((round.chemistry?.jogadores ?? []).map((player) => [player.index, player]));
  const ordered = layout?.ordered ?? (round.starters ?? []).slice().sort((a, b) => a.index - b.index);
  const scale = ratingScale([...views.map((item) => item.starter.rating), round.average_rating]);
  const showPitch = view === "campo" && !!layout;
  return <div className="panel">
    <div className="panel-head">
      <span>Titulares <span className="panel-head-sub">/ rodada {round.round}</span></span>
      <span className="panel-head-meta">média {round.average_rating.toFixed(1)} {scoreLabel}{round.chemistry ? ` · química ${round.chemistry.total}/${round.chemistry.maximo}` : ""}</span>
    </div>
    {view === "campo" && !layout ? <p className="hint gauntlet-pitch-fallback">{catalogState === "erro" ? "Catálogo de formações indisponível" : catalogState === "carregando" ? "Carregando o desenho da formação" : "Formação desconhecida ou sem 11 vagas"} — a tabela abaixo mostra a mesma escalação.</p> : null}
    {showPitch ? <>
      <FutPitch formation={formation} lines={layout!.lines} dense={layout!.dense} label={`Titulares da rodada ${round.round}`}>
        {ordered.map((starter) => {
          const item = byIndex.get(starter.index)!;
          const chem = chemistryByIndex.get(starter.index);
          const badge = item.weakest ? "menor nota" : item.potential ? `evo ${formatSigned(item.potential.gg_rating_gain)}` : null;
          return <div className="fut-slot" style={spotStyle(spots!.get(starter.index))} key={starter.index}>
            <button type="button" className={`fut-card tone-${item.tone}${openStarter === starter.index ? " open" : ""}`} aria-expanded={openStarter === starter.index} aria-label={`${starter.position}: ${starter.player.common_name || starter.player.name} · ${scoreLabel} ${starter.rating.toFixed(1)}${badge ? ` · ${badge}` : ""}`} onClick={() => onToggle(starter.index)}>
              <FutCardFace player={starter.player} position={starter.position} score={starter.rating} pips={chem ? (chem.fora_de_posicao ? 0 : chem.pontos) : undefined} scoreTone={item.tone === "cost" ? "cost" : item.tone === "alert" ? "alert" : ""} badge={badge} />
            </button>
          </div>;
        })}
      </FutPitch>
      <FutLegend items={[["turf", "na média da rodada ou acima"], ["alert", "evolução disponível"], ["cost", "menor nota da rodada"]]} />
    </> : <div className="tablewrap flush">
      <table>
        <thead><tr><th>Vaga</th><th>Carta</th><th className="num">OVR</th><th className="num">{scoreLabel} na vaga</th><th>vs média</th><th>Evolução</th></tr></thead>
        <tbody>{ordered.map((starter) => {
          const item = byIndex.get(starter.index)!;
          return <tr key={starter.index}>
            <td><span className={`gauntlet-pos tone-${item.tone}`}>{starter.position}</span></td>
            <td className="namecell"><span className="gauntlet-card-cell"><CardArt player={starter.player} className="gauntlet-thumb" />{starter.card_slug ? <Link to={`/time/${encodeURIComponent(starter.card_slug)}`}>{starter.player.common_name || starter.player.name}</Link> : <span>{starter.player.common_name || starter.player.name}</span>}</span></td>
            <td className="num">{starter.player.rating}</td>
            <td className={`num gauntlet-score tone-${item.tone}`}>{starter.rating.toFixed(1)}</td>
            <td className="gauntlet-vs"><span className="gauntlet-vs-track"><span className={`tone-${item.tone}`} style={{ width: `${Math.max(4, scale.pct(starter.rating))}%` }} /><i style={{ left: `${scale.pct(round.average_rating)}%` }} /></span></td>
            <td className="gauntlet-evo">{item.potential ? `${formatSigned(item.potential.gg_rating_gain)} GG` : "—"}</td>
          </tr>;
        })}</tbody>
      </table>
    </div>}
  </div>;
}

function BenchPanel({ round }: { round: GauntletRoundView }) {
  const bench = round.bench ?? [];
  return <div className="panel">
    <div className="panel-head">
      <span>Reservas <span className="panel-head-sub">/ rodada {round.round}</span></span>
      <span className="panel-head-meta">{bench.length} cartas · nenhuma reaparece</span>
    </div>
    <div className="panel-body gauntlet-bench-grid">
      {bench.map((card, index) => {
        const player = card.player;
        const content = <>
          <CardArt player={player} className="gauntlet-bench-art" />
          <span className="gauntlet-bench-text">
            <strong>{player.common_name || player.name}</strong>
            <span>{player.position} · {player.rating} OVR · GG {player.gg_rating ? player.gg_rating.toFixed(1) : "—"}</span>
          </span>
        </>;
        const key = player.club_item_id || `${player.id}-${index}`;
        return card.card_slug
          ? <Link className="gauntlet-bench-chit" key={key} to={`/time/${encodeURIComponent(card.card_slug)}`}>{content}</Link>
          : <div className="gauntlet-bench-chit" key={key}>{content}</div>;
      })}
    </div>
  </div>;
}

function DropPanel({ rounds, activeIndex, onSelect, benchPerRound }: { rounds: GauntletRoundView[]; activeIndex: number; onSelect: (index: number) => void; benchPerRound: number }) {
  const scale = ratingScale(rounds.map((round) => round.average_rating));
  const drop = rounds.length > 1 ? rounds[rounds.length - 1]!.average_rating - rounds[0]!.average_rating : 0;
  return <div className="panel">
    <div className="panel-head">
      <span>Queda por rodada <span className="panel-head-sub">/ média × química</span></span>
      {rounds.length > 1 ? <span className={`panel-head-meta gauntlet-drop${drop < 0 ? " down" : ""}`}>{formatSigned(drop)} até a R{rounds[rounds.length - 1]!.round}</span> : null}
    </div>
    <div className="panel-body">
      <div className="gauntlet-drop-chart">
        {rounds.map((round, index) => <button key={round.round} type="button" className={index === activeIndex ? "active" : ""} onClick={() => onSelect(index)} aria-label={`Rodada ${round.round}: média ${round.average_rating.toFixed(1)}`}>
          <span className="gauntlet-drop-value">{round.average_rating.toFixed(1)}</span>
          <span className="gauntlet-drop-col" style={{ height: `${Math.max(4, scale.pct(round.average_rating))}%` }} />
          <span className="gauntlet-drop-label">R{round.round} · {round.chemistry ? `${round.chemistry.total}q` : "—"}</span>
        </button>)}
      </div>
      <p className="gauntlet-drop-note">Escala a partir de {scale.low}. O modo usa {rounds.length * (11 + benchPerRound)} cartas: {rounds.length * 11} titulares + {rounds.length * benchPerRound} reservas.</p>
    </div>
  </div>;
}

function EvolutionPanel({ round, openStarter, onToggle }: { round: GauntletRoundView; openStarter: number | null; onToggle: (index: number) => void }) {
  const views = starterViews(round);
  return <div className="panel">
    <div className="panel-head">
      <span>Titulares e evolução <span className="panel-head-sub">/ rodada {round.round}</span></span>
      <span className="panel-head-meta">rotas pela nota FUT.GG</span>
    </div>
    {views.map(({ starter, potential, tone }) => {
      const open = openStarter === starter.index;
      const potentials = starter.potentials ?? [];
      return <div className={`gauntlet-evo-row${open ? " open" : ""}`} key={`${round.round}-${starter.index}`}>
        <button type="button" aria-expanded={open} onClick={() => onToggle(starter.index)}>
          <span className={`gauntlet-pos tone-${tone}`}>{starter.position}</span>
          <span className="gauntlet-evo-name"><strong>{starter.player.common_name || starter.player.name}</strong><span>GG {starter.rating.toFixed(1)} · {starter.player.rating} OVR</span></span>
          {potential ? <Chip tone="alert">{formatSigned(potential.gg_rating_gain)} GG</Chip> : <span className="gauntlet-evo-none">—</span>}
          <span className="gauntlet-evo-chev" aria-hidden="true">{open ? "−" : "+"}</span>
        </button>
        {open && <div className="gauntlet-evo-detail">
          {potentials.length === 0 ? <p>Sem rota de evolução confirmada pelo fut.gg para esta vaga.</p> : potentials.map((item, index) => {
            const cost = potentialCost(item);
            return <div className="gauntlet-evo-potential" key={index}>
              <span className="gauntlet-evo-gain"><b>{formatSigned(item.gg_rating_gain)} GG · GG final {item.final_gg_rating.toFixed(1)}</b><span>{item.training_time || "tempo não informado"}</span></span>
              {(item.path?.chain?.length ?? 0) > 0 && <span className="gauntlet-evo-path">via {item.path.chain!.join(" → ")}</span>}
              {(item.gained_play_styles?.length ?? 0) > 0 && <span className="gauntlet-evo-path">ganha {styleNames(item.gained_play_styles)}</span>}
              <Chip tone={cost.tone}>{cost.text}</Chip>
            </div>;
          })}
          {starter.card_slug && <Link className="rank-link" to={`/time/${encodeURIComponent(starter.card_slug)}`}>abrir carta →</Link>}
        </div>}
      </div>;
    })}
  </div>;
}
