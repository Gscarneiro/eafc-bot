import { fetchEvolucoes } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatCoins, formatSigned } from "../format";
import { useData } from "../useData";
import "../shared.css";
import "./Evolucoes.css";

// Evolucoes é "/evolucoes": quais evoluções do dia valem a pena NO seu
// elenco — distinto de "/time/:slug" (ver CLAUDE.md), que é "até onde ESTA
// carta chega" em vez de "o que lançou hoje que serve pra mim".
export default function Evolucoes() {
  const { data, error, loading, refetch } = useData(fetchEvolucoes, []);
  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const matches = data.matches ?? [];

  return (
    <div className="wrap">
      <PageHeader title="Evoluções que valem a pena" meta={`${matches.length} evoluções ativas encaixam no seu elenco hoje`} />

      {matches.length === 0 ? (
        <EmptyState
          message="Nenhuma evolução ativa compensa no seu elenco hoje."
          hint="Volte depois que a EA soltar evoluções novas, ou revise o orçamento configurado."
        />
      ) : (
        <div className="card-list">
          {matches.map((m, i) => {
            const objectives = (m.evolution.levels ?? []).flatMap((l) => l.objectives ?? []);
            return (
              <div className="tx-card" key={i}>
                <div className="tx-head">
                  <Chip tone="turf">{m.slot}</Chip>
                  {m.result.image_url && <img className="tx-thumb" src={m.result.image_url} alt="" loading="lazy" />}
                  <strong>{m.evolution.name}</strong>
                  <span>em {m.player.common_name || m.player.name}</span>
                  {m.beats_starter && <Chip tone="gain">vira titular</Chip>}
                </div>
                <div className="tx-row">
                  <span>{m.before.toFixed(1)}</span>
                  <span className="arrow">&rarr;</span>
                  <span>{m.after.toFixed(1)}</span>
                  <Chip tone="gain">{formatSigned(m.gain)}</Chip>
                  <span>{m.cost ? `${formatCoins(m.cost)} moedas` : "grátis"}</span>
                </div>
                {m.highlights && m.highlights.length > 0 && (
                  <ul className="rationale">
                    {m.highlights.map((h, j) => (
                      <li key={j}>{h}</li>
                    ))}
                  </ul>
                )}
                {objectives.length > 0 && (
                  <div className="evo-objectives">
                    <span className="evo-objectives-label">Objetivos:</span> {objectives.join(" · ")}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
