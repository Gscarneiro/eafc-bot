import { useState } from "react";
import { fetchSquadPlan } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import Pitch, { canDrawPitch } from "../components/Pitch";
import { formatSigned } from "../format";
import { useData } from "../useData";
import type { SquadPlanStarterView, StarterCard as StarterCardData } from "../types";
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

  return (
    <div className="wrap plano-elenco-page">
      <PageHeader
        eyebrow={`formação ${data.formation || "—"}`}
        title="Planejador de elenco"
        meta="cenários que trocam nota por entrosamento — o planejador nunca escolhe uma compra"
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

      {insufficient ? (
        <EmptyState
          message={data.reason || "Elenco insuficiente para montar um plano."}
          hint="O planejador precisa da escalação titular sincronizada em fut.gg/gg-club, com nota GG conhecida em cada posição."
        />
      ) : (
        <>
          <div className="plano-elenco-tabs" role="tablist" aria-label="Cenários do planejador">
            {scenarios.map((sc, i) => (
              <button
                key={i}
                type="button"
                role="tab"
                aria-selected={i === activeIndex}
                className={`plano-elenco-tab${i === activeIndex ? " active" : ""}`}
                onClick={() => setScenarioIndex(i)}
              >
                <span className="plano-elenco-tab-label">{sc.label || `cenário ${i + 1}`}</span>
                <span className="plano-elenco-tab-value">{sc.average_rating.toFixed(1)} GG</span>
              </button>
            ))}
          </div>

          {scenario && (
            <>
              <div className="plano-elenco-meta">
                {scenario.chemistry ? (
                  <Chip tone={scenario.chemistry.verificacao.status === "diverge" ? "alert" : "turf"}>
                    química {scenario.chemistry.total}/{scenario.chemistry.maximo}
                  </Chip>
                ) : (
                  <Chip tone="flat">química indisponível</Chip>
                )}
                <span className="plano-elenco-stats">
                  força total {scenario.total_rating.toFixed(1)} · média {scenario.average_rating.toFixed(1)} GG
                </span>
                {scenario.chemistry?.verificacao.status === "diverge" && (
                  <span className="plano-elenco-note">
                    modelo calcula {scenario.chemistry.verificacao.calculado}, o jogo reporta{" "}
                    {scenario.chemistry.verificacao.observado} — não confie neste número
                  </span>
                )}
              </div>

              <section>
                <h2>Titulares</h2>
                {canDrawPitch(data.formation || "", scenario.starters?.length ?? 0) ? (
                  <Pitch formation={data.formation} starters={toStarterCards(scenario.starters ?? [])} />
                ) : (
                  <div className="empty">Formação sem 11 slots reconhecidos — sem campo visual para este cenário.</div>
                )}
              </section>

              <section>
                <div className="section-title-row">
                  <h2>Movimentos em relação ao XI atual</h2>
                  <span className="count-label">{scenario.moves?.length ?? 0} trocas</span>
                </div>
                {(scenario.moves?.length ?? 0) === 0 ? (
                  <div className="empty">Nenhuma troca — este cenário já é o XI atual.</div>
                ) : (
                  <div className="card-list">
                    {(scenario.moves ?? []).map((m, i) => (
                      <div className="list-row" key={i}>
                        <div>
                          <div className="title">
                            {m.current.player.common_name || m.current.player.name} →{" "}
                            {m.suggested.player.common_name || m.suggested.player.name}
                          </div>
                          <p className="desc">{m.position}</p>
                        </div>
                        <p className="meta">{formatSigned(m.gain)} GG</p>
                      </div>
                    ))}
                  </div>
                )}
              </section>
            </>
          )}
        </>
      )}
    </div>
  );
}
