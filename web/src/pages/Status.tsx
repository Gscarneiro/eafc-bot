import { Link } from "react-router-dom";
import { fetchStatus } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import PageHeader from "../components/PageHeader";
import StatTile from "../components/StatTile";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatDate, formatDateTime, formatSigned } from "../format";
import { useData } from "../useData";
import "../shared.css";
import "./Status.css";

// Status é a tela "/" — o briefing do dia. Abre com a tese (nota do elenco
// + a jogada recomendada, ver StatusResponse.top_move), não com uma lista
// plana de números — saldo, elo mais fraco e o resto descem pra zona
// secundária. O que o CLAUDE.md chama de status diário: saldo, nota do
// elenco, elo mais fraco, o que mudou desde ontem, e a tendência dos
// últimos 30 dias.
export default function Status() {
  const { data, error, loading, refetch } = useData(fetchStatus, []);
  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const scoreHistory = (data.history ?? []).map((h) => ({ label: formatDate(h.date), value: h.squad_score }));
  const coinsHistory = (data.history ?? []).map((h) => ({ label: formatDate(h.date), value: h.coins }));
  const scoreDelta =
    scoreHistory.length >= 2 ? scoreHistory[scoreHistory.length - 1].value - scoreHistory[scoreHistory.length - 2].value : null;

  const added = data.diff.added ?? [];
  const removed = data.diff.removed ?? [];
  const newCards = data.new_cards ?? [];
  const news = data.news ?? [];
  const sbcs = data.sbcs ?? [];
  const objectives = data.objectives ?? [];
  const errors = data.errors ?? [];
  const changed = added.length > 0 || removed.length > 0 || data.diff.coins_delta !== 0;
  const move = data.top_move;

  return (
    <div className="wrap">
      <PageHeader eyebrow={`ciclo ${data.cycle}`} title="Status diário" meta={`gerado ${formatDateTime(data.generated_at)}`} />

      {errors.length > 0 && (
        <div className="banner alert">
          {errors.length} {errors.length === 1 ? "aviso" : "avisos"} nesta coleta
          <ul>
            {errors.map((e, i) => (
              <li key={i}>{e}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="status-hero">
        <div className="hero-score">
          <div className="hero-label">Nota do elenco</div>
          <div className="hero-value">{data.squad_score.toFixed(1)}</div>
          {scoreDelta !== null && Math.abs(scoreDelta) >= 0.05 && (
            <Chip tone={scoreDelta > 0 ? "gain" : "cost"}>{formatSigned(scoreDelta)} desde ontem</Chip>
          )}
        </div>
        <div className="hero-move">
          <div className="hero-label">Jogada de hoje</div>
          {move ? (
            <>
              <div className="hero-move-headline">{move.headline}</div>
              <div className="hero-move-row">
                <Chip tone="gain">{formatSigned(move.gain)} nota</Chip>
                <span>
                  {move.kind === "upgrade"
                    ? `${formatCoins(move.net_cost)} líquido`
                    : move.net_cost > 0
                      ? `${formatCoins(move.net_cost)} moedas`
                      : "grátis"}
                </span>
              </div>
              <Link className="hero-move-link" to={move.link}>
                ver em {move.kind === "upgrade" ? "mercado" : "evoluções"} &rarr;
              </Link>
            </>
          ) : (
            <div className="hero-move-empty">
              Nada dentro do orçamento hoje. Aumente <code>market.extra_budget</code> ou espere o mercado se mexer.
            </div>
          )}
        </div>
      </div>

      <div className="stat-grid">
        <StatTile
          label="Saldo"
          value={formatCoins(data.coins)}
          tone="coin"
          sub={data.raisable > 0 ? `+${formatCoins(data.raisable)} levantável` : undefined}
        />
        <StatTile
          label="Elo mais fraco"
          value={data.weakest_name || "—"}
          tone="name"
          sub={`${data.weakest_slot}${data.weakest_gg_rating > 0 ? ` · GG ${data.weakest_gg_rating.toFixed(1)}` : ""}`}
        />
        {data.diff.coins_delta !== 0 && (
          <StatTile
            label="Saldo desde ontem"
            value={formatSigned(data.diff.coins_delta)}
            tone={data.diff.coins_delta > 0 ? "up" : "down"}
          />
        )}
      </div>

      <section>
        <h2>Nota do elenco · 30 dias</h2>
        <TrendChart data={scoreHistory} valueFormatter={(v) => v.toFixed(0)} />
      </section>

      <section>
        <h2>Saldo · 30 dias</h2>
        <TrendChart data={coinsHistory} color="var(--gain)" valueFormatter={formatCoins} />
      </section>

      {changed && (
        <section>
          <h2>Mudou desde ontem</h2>
          {added.map((p) => (
            <div className="list-row" key={`a-${p.id}`}>
              <span className="title">
                <Chip tone="gain">+</Chip> {p.common_name || p.name}
              </span>
              <span className="meta">
                {p.rating} {p.position}
              </span>
            </div>
          ))}
          {removed.map((p) => (
            <div className="list-row" key={`r-${p.id}`}>
              <span className="title">
                <Chip tone="cost">−</Chip> {p.common_name || p.name}
              </span>
              <span className="meta">
                {p.rating} {p.position}
              </span>
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
                  <span className="meta">
                    {p.rating} {p.position} · {p.version}
                  </span>
                </div>
              ))}
            </div>
          )}

          {news.length > 0 && (
            <div className="feed-card">
              <h2>Notícias</h2>
              {news.map((n) => (
                <div className="list-row" key={n.id || n.url || n.title}>
                  {n.url ? (
                    <a className="title" href={n.url} target="_blank" rel="noreferrer">
                      {n.title}
                    </a>
                  ) : (
                    <span className="title">{n.title}</span>
                  )}
                  <span className="meta">{formatDate(n.published_at)}</span>
                  {n.summary && <p className="desc">{n.summary}</p>}
                </div>
              ))}
            </div>
          )}

          {sbcs.length > 0 && (
            <div className="feed-card">
              <h2>SBCs que valem a pena</h2>
              {sbcs.map((s) => (
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
              {objectives.map((o) => (
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
