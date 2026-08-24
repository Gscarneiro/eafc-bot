import { Link, useParams } from "react-router-dom";
import { ApiError, fetchCard } from "../api";
import { asyncGate } from "../components/asyncGate";
import AttributeFace from "../components/AttributeFace";
import Chip from "../components/Chip";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatDate, formatSigned, styleNames } from "../format";
import { useData } from "../useData";
import type { EvoPotential } from "../types";
import "../shared.css";
import "./CardDetail.css";

function PathTimeline({ potential }: { potential: EvoPotential }) {
  const steps = potential.path?.steps ?? [];
  return <div className="detail-timeline" aria-label="Linha do tempo do path">{steps.map((step, index) => <div className="detail-step" key={`${step.id}-${index}`}><span className="detail-step-index">{String(index + 1).padStart(2, "0")}</span>{step.image_url && <img src={step.image_url} alt="" loading="lazy" />}<div><strong>{step.rating} {step.position}</strong><span>{step.gg_rating ? `GG ${step.gg_rating.toFixed(1)}` : index === 0 ? "atual" : "projetado"}</span></div>{index < steps.length - 1 && <span className="detail-step-arrow" aria-hidden="true">→</span>}</div>)}</div>;
}

// CardDetail é "/time/:slug" — atual x potencial de uma carta específica,
// com o histórico de preço dela e a face de atributos (PAC/SHO/PAS/DRI/DEF/
// PHY, ou DIV/HAN/KIC/REF/SPD/POS pro goleiro — ver AttributeFace).
// Distinto de "/evolucoes" (ver CLAUDE.md): aqui é "até onde ESTA carta
// chega", lá é "que evolução lançou hoje que serve pro meu elenco".
export default function CardDetail() {
  const { slug = "" } = useParams<{ slug: string }>();
  const { data: report, error, loading, refetch } = useData(() => fetchCard(slug), [slug]);

  // 404 aqui é um caso comum, não uma falha de rede: a carta caiu abaixo do
  // cards_min_rating na última coleta, ou o link é de um snapshot antigo.
  if (error instanceof ApiError && error.status === 404) {
    return (
      <div className="wrap narrow">
        <Link className="back-link" to="/time">
          &larr; voltar para o time
        </Link>
        <p>
          Carta &quot;{slug}&quot; não encontrada na última coleta — pode ter caído abaixo do overall mínimo
          analisado, ou o link é de um dia anterior.
        </p>
      </div>
    );
  }

  const gate = asyncGate(loading, error, !!report, refetch);
  if (gate) return gate;
  if (!report) return null;

  const p = report.player;
  const byPosition = report.by_position ?? [];
  const alternates = report.alternates ?? [];
  const altPositions = p.alt_positions ?? [];
  const priceSeries = (report.price_series ?? [])
    .slice()
    .sort((a, b) => a.observed_at.localeCompare(b.observed_at))
    .map((pt) => ({ label: formatDate(pt.observed_at), value: pt.coins }));

  return (
    <div className="wrap narrow">
      <Link className="back-link" to="/time">
        &larr; voltar para o time
      </Link>

      <div className="head">
        {p.image_url && <img src={p.image_url} alt="" />}
        <div>
          <h1>{p.common_name || p.name}</h1>
          <div className="sub">
            {p.version} · {p.club}
          </div>
          <div className="statline">
            <Chip tone="turf">
              {p.rating} {p.position}
              {altPositions.length > 0 ? ` (${altPositions.join(", ")})` : ""}
            </Chip>
            {p.gg_rating ? <Chip tone="flat">GG Rating {p.gg_rating.toFixed(1)}</Chip> : null}
          </div>
        </div>
      </div>

      <section>
        <h2>Atributos</h2>
        <AttributeFace attributes={p.attributes} position={p.position} />
        <div className="attr-facts">
          <span>pé {p.foot || "—"}</span>
          <span>pé fraco {p.weak_foot}/5</span>
          <span>habilidades {p.skill_moves}/5</span>
        </div>
      </section>

      <section>
        <h2>PlayStyles</h2>
        {p.play_styles && p.play_styles.length > 0 ? (
          <div className="playstyle-grid">
            {p.play_styles.map((style) => <Chip key={`${style.name}-${style.plus}`} tone={style.plus ? "gain" : "flat"}>{styleNames([style])}</Chip>)}
          </div>
        ) : <div className="empty">Esta carta não tem PlayStyles informados na última coleta.</div>}
      </section>

      {priceSeries.length > 1 && (
        <section>
          <h2>Preço · 30 dias</h2>
          <TrendChart data={priceSeries} valueFormatter={formatCoins} height={140} />
        </section>
      )}

      {byPosition.length > 0 && (
        <section>
          <h2>Funções por posição</h2>
          {byPosition.map((pr) => {
            const plusPlus = pr.plus_plus ?? [];
            const plus = pr.plus ?? [];
            return (
              <div className="pos-block" key={pr.position}>
                <div className="pos-name">{pr.position}</div>
                <div className="roles">
                  {plusPlus.length > 0 && (
                    <>
                      <span className="lvl">Role++</span> {plusPlus.join(", ")}
                    </>
                  )}
                  {plusPlus.length > 0 && plus.length > 0 && <br />}
                  {plus.length > 0 && (
                    <>
                      <span className="lvl">Role+</span> {plus.join(", ")}
                    </>
                  )}
                </div>
              </div>
            );
          })}
        </section>
      )}

      <section>
        <h2>Potencial</h2>
        {report.best ? (
          <div className="potential-card">
            <div className="potential-row">
              <span className="big">
                {p.rating}
                {p.gg_rating ? ` · GG ${p.gg_rating.toFixed(1)}` : ""}
              </span>
              <span className="arrow">&rarr;</span>
              <span className="big">
                {report.best.final_overall} · GG {report.best.final_gg_rating.toFixed(1)}
              </span>
              <Chip tone="gain">{formatSigned(report.best.gg_rating_gain)} GG</Chip>
            </div>
            {report.best.gained_play_styles && report.best.gained_play_styles.length > 0 && (
              <p className="why">Novos PlayStyles: {styleNames(report.best.gained_play_styles)}</p>
            )}
            <div className="chain">via {(report.best.path.chain ?? []).join(" → ")}</div>
            <PathTimeline potential={report.best} />
            <div className="money">
              <span>{report.best.coin_cost ? `${formatCoins(report.best.coin_cost)} moedas` : "grátis"}</span>
              {report.best.point_cost > 0 && <span>{report.best.point_cost} pontos</span>}
              {report.best.training_time && <span>{report.best.training_time} de treino</span>}
            </div>
          </div>
        ) : (
          <div className="empty">
            Já está no teto das evoluções ativas — nenhum caminho disponível hoje leva essa carta mais longe.
          </div>
        )}
      </section>

      {alternates.length > 0 && (
        <section>
          <h2>Outros caminhos</h2>
          {alternates.map((alt, i) => (
            <div className="alt" key={i}>
              <div className="row1">
                <strong>
                  {alt.final_overall} · GG {alt.final_gg_rating.toFixed(1)}
                </strong>
                <Chip tone="gain">{formatSigned(alt.gg_rating_gain)} GG</Chip>
                <span>{alt.coin_cost ? `${formatCoins(alt.coin_cost)} moedas` : "grátis"}</span>
              </div>
              <div className="chain">via {(alt.path.chain ?? []).join(" → ")}</div>
              <PathTimeline potential={alt} />
            </div>
          ))}
        </section>
      )}
    </div>
  );
}
