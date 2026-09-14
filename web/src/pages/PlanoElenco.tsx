import { useState } from "react";
import { fetchSquadPlan } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import Pitch, { canDrawPitch } from "../components/Pitch";
import { evaluationSourceLabel, formatSigned } from "../format";
import { useData } from "../useData";
import type { SquadPlanScenario, SquadPlanStarterView, StarterCard as StarterCardData } from "../types";
import "../shared.css";
import "./PlanoElenco.css";

function toStarterCards(starters: SquadPlanStarterView[]): StarterCardData[] {
  return starters.map((s) => ({
    player: s.player,
    card_slug: s.card_slug,
    index: s.index,
    position: s.position,
    position_gg_rating: s.rating,
  }));
}

// PlanoElenco é a tela "/time/planos": a fronteira nota×química do elenco —
// de 3 a 5 cenários que trocam GG Rating por entrosamento, lado a lado.
// Diferente de "Meu time" (uma sugestão só) e do Gauntlet (quatro elencos
// consecutivos), aqui o usuário ESCOLHE entre pontos de um trade-off. Todo
// o cálculo vem pronto de POST /api/planos/elenco; esta tela só compara.
export default function PlanoElenco() {
  const { data, error, loading, refetch } = useData(fetchSquadPlan, []);
  const [scenarioIndex, setScenarioIndex] = useState(0);

  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const scenarios = data.scenarios ?? [];
  const needs = data.needs ?? [];
  const activeIndex = Math.min(scenarioIndex, Math.max(scenarios.length - 1, 0));
  const scenario = scenarios[activeIndex];
  const insufficient = data.status !== "ok" || scenarios.length === 0;
	const scoreLabel = evaluationSourceLabel(data.avaliacao?.fonte, true);

  return (
    <div className="wrap plano-elenco-page">
      <PageHeader
        eyebrow={`formação ${data.formation || "—"}`}
        title="Planejador de elenco"
        meta="cenários que trocam nota por entrosamento — o planejador nunca escolhe uma compra"
        actions={
          scenario ? (
            <div className="plano-elenco-meta">
              {scenario.chemistry ? (
                <Chip tone={scenario.chemistry.verificacao.status === "diverge" ? "alert" : "turf"}>
                  química {scenario.chemistry.total}/{scenario.chemistry.maximo}
                </Chip>
              ) : (
                <Chip tone="flat">química indisponível</Chip>
              )}
              <span className="plano-elenco-stats">
                força total {scenario.total_rating.toFixed(1)} · média {scenario.average_rating.toFixed(1)} {scoreLabel} posicional
              </span>
            </div>
          ) : undefined
        }
      />

      {needs.length > 0 && (
        <div className="banner alert plano-elenco-needs">
          <strong>Necessidades apontadas</strong> — sem escolher qual carta comprar:
          <ul>
            {needs.map((n, i) => (
              <li key={i}>
                {n.position}: {n.reason}
              </li>
            ))}
          </ul>
        </div>
      )}

      {(data.warnings?.length ?? 0) > 0 && (
        <div className="banner alert">
          <ul>
            {(data.warnings ?? []).map((w, i) => (
              <li key={i}>{w}</li>
            ))}
          </ul>
        </div>
      )}

      {scenario?.chemistry?.verificacao.status === "diverge" && (
        <div className="banner alert">
          modelo calcula {scenario.chemistry.verificacao.calculado}, o jogo reporta {scenario.chemistry.verificacao.observado} — não confie neste número
        </div>
      )}

      {insufficient ? (
        <EmptyState
          message={data.reason || "Elenco insuficiente para montar um plano."}
		  hint="O planejador precisa da escalação titular sincronizada e da cobertura da fonte ativa em cada vaga."
        />
      ) : (
        <div className="plano-elenco-grid">
          <div className="panel plano-elenco-pitch-panel">
            <div className="panel-head">
              <span>Titulares <span className="panel-head-sub">· cenário {scenario?.label || `cenário ${activeIndex + 1}`}</span></span>
              <span className="panel-head-meta">{data.formation} · {scenario?.starters?.length ?? 0} vagas reconhecidas</span>
            </div>
            {scenario && canDrawPitch(data.formation || "", scenario.starters?.length ?? 0) ? (
              <Pitch formation={data.formation} starters={toStarterCards(scenario.starters ?? [])} />
            ) : (
              <div className="panel-body"><div className="empty">Formação sem 11 slots reconhecidos — sem campo visual para este cenário.</div></div>
            )}
          </div>

          <div className="plano-elenco-side">
            <div className="panel">
              <div className="panel-head"><span>A troca</span><span className="panel-head-meta">nota × química</span></div>
              <div className="panel-body">
                <ScenarioTradeChart scenarios={scenarios} activeIndex={activeIndex} />
              </div>
              <div className="tab-strip flush" role="tablist" aria-label="Cenários do planejador">
                {scenarios.map((sc, i) => (
                  <button
                    key={i}
                    type="button"
                    role="tab"
                    aria-selected={i === activeIndex}
                    className={`tab-strip-cell${i === activeIndex ? " active" : ""}`}
                    onClick={() => setScenarioIndex(i)}
                  >
                    <span className="tab-strip-label">{sc.label || `cenário ${i + 1}`}</span>
                    <span className="tab-strip-value">{sc.average_rating.toFixed(1)}</span>
                  </button>
                ))}
              </div>
            </div>

            <div className="panel plano-elenco-moves-panel">
              <div className="panel-head">
                <span>Movimentos vs. XI atual</span>
                <span className="panel-head-meta">{scenario?.moves?.length ?? 0} trocas</span>
              </div>
              <div className="panel-body">
                {(scenario?.moves?.length ?? 0) === 0 ? (
                  <div className="empty">Nenhuma troca — este cenário já é o XI atual.</div>
                ) : (
                  <div className="card-list">
                    {(scenario?.moves ?? []).map((m, i) => (
                      <div className="list-row" key={m.current.player.club_item_id || m.suggested.player.club_item_id || `${m.position}-${i}`}>
                        <div>
                          <div className="title">
                            {m.current.player.common_name || m.current.player.name} →{" "}
                            {m.suggested.player.common_name || m.suggested.player.name}
                          </div>
                          <p className="desc">{m.position}</p>
                          <div className="plano-elenco-move-ratings">
                            <span>{scoreLabel} {m.current_rating.toFixed(1)}</span>
                            <span aria-hidden="true">→</span>
                            <span>{scoreLabel} {m.suggested_rating.toFixed(1)}</span>
                          </div>
                        </div>
                        <p className="meta">{formatSigned(m.gain)} {scoreLabel} posicional</p>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ScenarioTradeChart plota nota média × química total de cada cenário — a
// mesma fronteira que os cartões de cima já descrevem em texto, aqui como
// curva. Só entram cenários com química calculada (scenario.chemistry
// definido); com menos de 2 pontos não há curva pra desenhar.
function ScenarioTradeChart({ scenarios, activeIndex }: { scenarios: SquadPlanScenario[]; activeIndex: number }) {
  const points = scenarios
    .map((sc, i) => ({ i, rating: sc.average_rating, chem: sc.chemistry?.total }))
    .filter((p): p is { i: number; rating: number; chem: number } => p.chem !== undefined);
  if (points.length < 2) {
    return <p className="hint">Química insuficiente nos cenários para desenhar a curva nota × química.</p>;
  }
  const width = 376;
  const height = 136;
  const padL = 34;
  const padR = 8;
  const padT = 10;
  const padB = 26;
  const chems = points.map((p) => p.chem);
  const ratings = points.map((p) => p.rating);
  const chemMin = Math.min(...chems);
  const chemMax = Math.max(...chems);
  const ratingMin = Math.min(...ratings);
  const ratingMax = Math.max(...ratings);
  const chemSpan = chemMax - chemMin || 1;
  const ratingSpan = ratingMax - ratingMin || 1;
  const x = (chem: number) => padL + ((chem - chemMin) / chemSpan) * (width - padL - padR);
  const y = (rating: number) => padT + (1 - (rating - ratingMin) / ratingSpan) * (height - padT - padB);
  const coords = points.map((p) => ({ ...p, x: x(p.chem), y: y(p.rating) }));

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="scenario-chart" role="img" aria-label="nota média por química entre os cenários do planejador">
      <line x1={padL} y1={padT} x2={padL} y2={height - padB} className="scenario-chart-axis" />
      <line x1={padL} y1={height - padB} x2={width - padR} y2={height - padB} className="scenario-chart-axis" />
      <polyline points={coords.map((p) => `${p.x},${p.y}`).join(" ")} className="scenario-chart-line" />
      {coords.map((p) => (
        <g key={p.i}>
          <circle cx={p.x} cy={p.y} r={p.i === activeIndex ? 5 : 4} className={`scenario-chart-dot${p.i === activeIndex ? " active" : ""}`} />
          <text x={p.x} y={p.y - 10} textAnchor="middle" className={`scenario-chart-label${p.i === activeIndex ? " active" : ""}`}>{p.rating.toFixed(1)}</text>
          <text x={p.x} y={height - padB + 13} textAnchor="middle" className="scenario-chart-axis-label">{p.chem}</text>
        </g>
      ))}
      <text x={padL - 6} y={padT + 4} textAnchor="end" className="scenario-chart-axis-label">{ratingMax.toFixed(0)}</text>
      <text x={padL - 6} y={height - padB + 3} textAnchor="end" className="scenario-chart-axis-label">{ratingMin.toFixed(0)}</text>
      <text x={(padL + width - padR) / 2} y={height - 2} textAnchor="middle" className="scenario-chart-axis-title">química do XI</text>
    </svg>
  );
}
