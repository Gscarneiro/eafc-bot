import { useState } from "react";
import { Link } from "react-router-dom";
import { applySquadPlanReference, fetchSquadPlan, saveSquadPlan } from "../api";
import { evaluationSourceLabel, formatSigned } from "../format";
import { useData } from "../useData";
import type { SavedSquadPlanInput, SquadPlanNeed, SquadPlanResponse, SquadPlanScenario } from "../types";

// TimePlanejador é a seção "Planejador" do Meu time (antes a tela
// /time/planos): de 3 a 5 cenários que trocam nota por entrosamento, lado a
// lado, só com cartas do clube. O planejador APONTA necessidades, nunca
// escolhe uma compra — isso é o mercado. Todo o cálculo vem pronto de
// POST /api/planos/elenco; esta seção só compara e deixa salvar o cenário
// como um plano do Editor.
export default function TimePlanejador({ regua }: { regua: number }) {
  const { data, error, loading, refetch } = useData(fetchSquadPlan, []);
  const [scenarioIndex, setScenarioIndex] = useState(0);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [saveError, setSaveError] = useState("");

  const head = <div className="panel-head time-planner-head">
    <span>Planejador <span className="panel-head-sub">/ cenários que trocam nota por química · só cartas do clube</span></span>
    <span className="panel-head-meta">o planejador nunca escolhe uma compra</span>
  </div>;
  if (loading) return <section className="panel time-planner" id="planejador">{head}<p className="time-planner-note">Calculando cenários…</p></section>;
  if (error || !data) return <section className="panel time-planner" id="planejador">{head}<p className="time-planner-note error">{error?.message || "Não foi possível montar os cenários."} <button type="button" className="btn ghost" onClick={refetch}>tentar novamente</button></p></section>;

  const scenarios = data.scenarios ?? [];
  if (data.status !== "ok" || scenarios.length === 0) {
    return <section className="panel time-planner" id="planejador">{head}<p className="time-planner-note">{data.reason || "Elenco insuficiente para montar cenários."} {data.reason === "escalação titular não sincronizada" ? "Sincronize a escalação titular no fut.gg e rode uma coleta nova (eafcbot run)." : "A fonte ativa não publicou uma nota compatível para a vaga indicada."}</p></section>;
  }

  const activeIndex = Math.min(scenarioIndex, scenarios.length - 1);
  const scenario = scenarios[activeIndex]!;
  const scoreLabel = evaluationSourceLabel(data.avaliacao?.fonte, true);
  // A base do delta é o cenário sem troca (o próprio XI atual) quando o
  // planejador o devolve; senão, a média do XI do mapa de posições.
  const base = scenarios.find((item) => (item.moves?.length ?? 0) === 0)?.average_rating ?? (regua > 0 ? regua : undefined);
  const moves = scenario.moves ?? [];
  const needs = data.needs ?? [];
  const canSave = (scenario.starters?.length ?? 0) === 11 && !!data.formation;

  const save = async (reference: boolean) => {
    if (!canSave) return;
    setSaving(true); setMessage(""); setSaveError("");
    const nome = `Planejador · ${scenario.label || `cenário ${activeIndex + 1}`}`;
    const input: SavedSquadPlanInput = {
      nome, formacao: data.formation, origem_formacao: "confirmada",
      vagas: (scenario.starters ?? []).map((starter) => ({ index: starter.index, posicao: starter.position, carta: { club_item_id: starter.player.club_item_id || undefined, player_id: starter.player.id } })),
    };
    try {
      const saved = await saveSquadPlan(input);
      if (reference) await applySquadPlanReference(saved.plano.id);
      setMessage(`“${nome}” salvo como plano${reference ? " e marcado como referência ativa" : ""}.`);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Não foi possível salvar o cenário.");
    } finally {
      setSaving(false);
    }
  };

  return <section className="panel time-planner" id="planejador">
    {head}
    <div className="time-planner-tabs" role="tablist" aria-label="Cenários do planejador">
      {scenarios.map((item, index) => {
        const delta = base !== undefined ? item.average_rating - base : undefined;
        const count = item.moves?.length ?? 0;
        return <button key={index} type="button" role="tab" aria-selected={index === activeIndex} className={index === activeIndex ? "active" : ""} onClick={() => { setScenarioIndex(index); setMessage(""); setSaveError(""); }}>
          <span className="time-planner-label">{item.label || `cenário ${index + 1}`}</span>
          <span className="time-planner-value"><strong>{item.average_rating.toFixed(1)}</strong>{delta !== undefined ? <span className={count === 0 ? "" : delta > 0.05 ? "up" : delta < -0.05 ? "down" : ""}>{count === 0 ? "base" : formatSigned(delta)}</span> : null}</span>
          <span className="time-planner-meta">{item.chemistry ? `química ${item.chemistry.total}/${item.chemistry.maximo}` : "química —"} · {count ? `${count} ${count === 1 ? "troca" : "trocas"}` : "sem trocas"}{item.rated_starters < (item.starters?.length ?? 0) ? ` · ${item.rated_starters}/${item.starters?.length ?? 0} avaliadas` : ""}</span>
        </button>;
      })}
    </div>
    <div className="time-planner-body">
      <div className="time-planner-chart">
        <div className="time-planner-kicker"><span>nota média × química</span><span>{scoreLabel} na vaga</span></div>
        <TradeChart scenarios={scenarios} activeIndex={activeIndex} onSelect={setScenarioIndex} />
      </div>
      <div className="time-planner-moves">
        <div className="time-planner-kicker"><span>movimentos vs XI atual · {(scenario.label || `cenário ${activeIndex + 1}`).toLowerCase()}</span><span>{moves.length} {moves.length === 1 ? "troca" : "trocas"}</span></div>
        {moves.length === 0 ? <p className="time-planner-empty">Nenhuma troca — este cenário já é o XI atual.</p> : moves.map((move, index) => <div className="time-planner-move" key={move.current.player.club_item_id || move.suggested.player.club_item_id || `${move.position}-${index}`}>
          <span className="time-planner-pos">{move.position}</span>
          <span className="time-planner-move-text">
            <strong>{move.current.player.common_name || move.current.player.name} → {move.suggested.player.common_name || move.suggested.player.name}</strong>
            <span>{scoreLabel} {move.current_rating.toFixed(1)} → {move.suggested_rating.toFixed(1)}</span>
          </span>
          <span className={`time-planner-gain ${move.gain >= 0 ? "up" : "down"}`}>{formatSigned(move.gain)}</span>
        </div>)}
        <PlannerNotes data={data} scenario={scenario} />
        <div className="time-planner-foot">
          <span className="time-planner-needs">{needs.length ? needsByReason(needs).map(([reason, positions]) => `Necessidade em ${positions.join(", ")}: ${reason}.`).join(" ") : "Nenhuma necessidade apontada para este XI."}</span>
          <span className="time-planner-actions">
            <button type="button" className="btn ghost" disabled={!canSave || saving} onClick={() => save(false)}>salvar como plano</button>
            <button type="button" className="btn primary" disabled={!canSave || saving} onClick={() => save(true)}>usar como referência</button>
          </span>
        </div>
        {message ? <p className="time-planner-message" role="status">{message} <Link to="/time/editor">abrir no editor →</Link></p> : null}
        {saveError ? <p className="time-planner-message error" role="alert">{saveError}</p> : null}
      </div>
    </div>
  </section>;
}

// needsByReason junta as vagas com o mesmo motivo: num clube raso quase
// toda vaga repete "sem alternativa no elenco", e uma frase por vaga vira
// um parágrafo que ninguém lê.
function needsByReason(needs: SquadPlanNeed[]): [string, string[]][] {
  const byReason = new Map<string, string[]>();
  for (const need of needs) byReason.set(need.reason, [...(byReason.get(need.reason) ?? []), need.position]);
  return [...byReason];
}

function PlannerNotes({ data, scenario }: { data: SquadPlanResponse; scenario: SquadPlanScenario }) {
  const notes = [...(data.warnings ?? [])];
  if (scenario.chemistry?.verificacao.status === "diverge") notes.push(`modelo calcula ${scenario.chemistry.verificacao.calculado}, o jogo reporta ${scenario.chemistry.verificacao.observado} — não confie na química deste cenário`);
  if (!notes.length) return null;
  return <ul className="time-planner-warnings">{notes.map((note) => <li key={note}>{note}</li>)}</ul>;
}

// TradeChart plota nota média × química de cada cenário — a mesma fronteira
// que as abas descrevem em texto, como curva. Só entram cenários com química
// calculada; com menos de 2 pontos não há curva para desenhar.
function TradeChart({ scenarios, activeIndex, onSelect }: { scenarios: SquadPlanScenario[]; activeIndex: number; onSelect: (index: number) => void }) {
  const points = scenarios
    .map((item, index) => ({ index, label: item.label || `cenário ${index + 1}`, rating: item.average_rating, chem: item.chemistry?.total }))
    .filter((point): point is { index: number; label: string; rating: number; chem: number } => point.chem !== undefined)
    .sort((a, b) => a.chem - b.chem || a.rating - b.rating);
  if (points.length < 2) return <p className="time-planner-empty">Química insuficiente nos cenários para desenhar a curva nota × química.</p>;
  const W = 360, H = 150, L = 34, R = 14, T = 20, B = 26;
  const chemMin = Math.min(...points.map((p) => p.chem)) - 1;
  const chemMax = Math.max(...points.map((p) => p.chem)) + 1;
  const ratingMin = Math.min(...points.map((p) => p.rating));
  const ratingMax = Math.max(...points.map((p) => p.rating));
  const ratingSpan = ratingMax - ratingMin || 1;
  const x = (chem: number) => L + ((chem - chemMin) / (chemMax - chemMin)) * (W - L - R);
  const y = (rating: number) => T + (1 - (rating - ratingMin) / ratingSpan) * (H - T - B);
  const ticks = Array.from({ length: chemMax - chemMin + 1 }, (_, i) => chemMin + i).filter((tick, _, all) => all.length <= 8 || tick % 2 === 0);
  return <svg viewBox={`0 0 ${W} ${H}`} className="time-trade-chart" role="img" aria-label="nota média por química entre os cenários do planejador">
    <line x1={L} y1={T - 6} x2={L} y2={H - B} className="axis" />
    <line x1={L} y1={H - B} x2={W - R} y2={H - B} className="axis" />
    <polyline points={points.map((p) => `${x(p.chem)},${y(p.rating)}`).join(" ")} className="curve" />
    <text x={L - 6} y={T + 3} textAnchor="end" className="tick">{ratingMax.toFixed(1)}</text>
    <text x={L - 6} y={H - B + 3} textAnchor="end" className="tick">{ratingMin.toFixed(1)}</text>
    {ticks.map((tick) => <text key={tick} x={x(tick)} y={H - B + 12} textAnchor="middle" className="tick">{tick}</text>)}
    <text x={(L + W - R) / 2} y={H - 2} textAnchor="middle" className="tick">química do XI</text>
    {points.map((p) => {
      const on = p.index === activeIndex;
      const rightEdge = x(p.chem) > W - R - 90;
      return <g key={p.index} className={`point${on ? " active" : ""}`} onClick={() => onSelect(p.index)}>
        <circle cx={x(p.chem)} cy={y(p.rating)} r={on ? 5.5 : 4} />
        <text x={x(p.chem) + (rightEdge ? -8 : 8)} y={y(p.rating) - 7} textAnchor={rightEdge ? "end" : "start"}>{p.label} {p.rating.toFixed(1)}</text>
      </g>;
    })}
  </svg>;
}
