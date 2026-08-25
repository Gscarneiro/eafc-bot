import { useState } from "react";
import { Link } from "react-router-dom";
import { fetchGauntlet } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import ExpandIcon from "../components/ExpandIcon";
import PageHeader from "../components/PageHeader";
import Pitch, { canDrawPitch } from "../components/Pitch";
import { formatCoins, formatDateTime, formatSigned, isZeroTime, styleNames } from "../format";
import { useData } from "../useData";
import type { GauntletStarterView, RosterCard, StarterCard as StarterCardData } from "../types";
import "../shared.css";
import "./Gauntlet.css";

function toStarterCards(starters: GauntletStarterView[]): StarterCardData[] {
  return starters.map((s) => ({
    player: s.player,
    card_slug: s.card_slug,
    index: s.index,
    position: s.position,
    position_gg_rating: s.rating,
  }));
}

// Gauntlet é a tela "/time/gauntlet": quatro elencos consecutivos (11
// titulares + 7 reservas cada) sem repetir carta nenhuma entre eles — a
// regra oficial do modo (ver o aviso em data.rules). Todo o cálculo vem
// pronto de GET /api/gauntlet; esta tela só exibe o plano.
export default function Gauntlet() {
  const { data, error, loading, refetch } = useData(fetchGauntlet, []);
  const [roundIndex, setRoundIndex] = useState(0);
  const [openStarter, setOpenStarter] = useState<number | null>(null);

  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

  const rounds = data.rounds ?? [];
  const objectives = data.objectives ?? [];
  const activeIndex = Math.min(roundIndex, Math.max(rounds.length - 1, 0));
  const round = rounds[activeIndex];
  const insufficient = data.status !== "ok" || rounds.length === 0;

  return (
    <div className="wrap gauntlet-page">
      <PageHeader
        eyebrow={`formação ${data.formation || "—"}`}
        title="Gauntlet"
        meta={`coletado em ${formatDateTime(data.generated_at)}`}
      />
      <div className="banner gauntlet-rules">{data.rules}</div>
      {(data.warnings?.length ?? 0) > 0 && (
        <div className="banner alert gauntlet-warnings">
          <ul>
            {(data.warnings ?? []).map((w, i) => (
              <li key={i}>{w}</li>
            ))}
          </ul>
        </div>
      )}

      {objectives.length > 0 && (
        <section className="gauntlet-objectives">
          <h2>Objetivos do Gauntlet</h2>
          <div className="card-list">
            {objectives.map((o) => (
              <div className="list-row" key={o.id}>
                <div>
                  <div className="title">{o.name}</div>
                  {(o.tasks?.length ?? 0) > 0 && <p className="desc">{(o.tasks ?? []).join(" · ")}</p>}
                </div>
                <p className="meta">
                  {o.group}
                  {!isZeroTime(o.expires_at) ? ` · expira ${formatDateTime(o.expires_at)}` : ""}
                </p>
              </div>
            ))}
          </div>
        </section>
      )}

      {insufficient ? (
        <EmptyState
          message={data.reason || "Elenco insuficiente para montar o Gauntlet."}
          hint="O Gauntlet precisa de 72 cartas do clube com GG Rating conhecido (44 titulares + 28 reservas) e da escalação titular sincronizada em fut.gg/gg-club."
        />
      ) : (
        <>
          <div className="gauntlet-round-tabs" role="tablist" aria-label="Rodadas do Gauntlet">
            {rounds.map((r, i) => (
              <button
                key={r.round}
                type="button"
                role="tab"
                aria-selected={i === activeIndex}
                className={`gauntlet-round-tab${i === activeIndex ? " active" : ""}`}
                onClick={() => {
                  setRoundIndex(i);
                  setOpenStarter(null);
                }}
              >
                <span className="gauntlet-round-tab-label">Rodada {r.round}</span>
                <span className="gauntlet-round-tab-value">{r.average_rating.toFixed(1)} GG</span>
              </button>
            ))}
          </div>

          {round && (
            <>
              <div className="gauntlet-round-meta">
                {round.chemistry ? (
                  <Chip tone={round.chemistry.verificacao.status === "diverge" ? "alert" : "turf"}>
                    química {round.chemistry.total}/{round.chemistry.maximo}
                  </Chip>
                ) : (
                  <Chip tone="flat">química indisponível</Chip>
                )}
                <span className="gauntlet-round-stats">
                  força total {round.total_rating.toFixed(1)} · média {round.average_rating.toFixed(1)} GG
                </span>
                {round.chemistry?.verificacao.status === "diverge" && (
                  <span className="gauntlet-round-note">
                    modelo calcula {round.chemistry.verificacao.calculado}, o jogo reporta {round.chemistry.verificacao.observado} — não confie neste número
                  </span>
                )}
              </div>

              <section>
                <h2>Titulares</h2>
                {canDrawPitch(data.formation || "", round.starters?.length ?? 0) ? (
                  <Pitch formation={data.formation} starters={toStarterCards(round.starters ?? [])} />
                ) : (
                  <div className="empty">Formação sem 11 slots reconhecidos — sem campo visual para esta rodada.</div>
                )}
              </section>

              <section>
                <h2>Titulares e potencial de evolução</h2>
                <p className="section-note">
                  Só caminhos com nota final confirmada pelo fut.gg para a posição escalada aparecem aqui.
                </p>
                <div className="gauntlet-starter-list">
                  {(round.starters ?? []).map((s, i) => (
                    <StarterRow
                      key={`${round.round}-${s.player.id}`}
                      starter={s}
                      open={openStarter === i}
                      onToggle={() => setOpenStarter(openStarter === i ? null : i)}
                    />
                  ))}
                </div>
              </section>

              <section>
                <div className="section-title-row">
                  <h2>Reservas</h2>
                  <span className="count-label">{round.bench?.length ?? 0} cartas</span>
                </div>
                <div className="gauntlet-bench-grid">
                  {(round.bench ?? []).map((b) => (
                    <BenchChit key={b.player.id} card={b} />
                  ))}
                </div>
              </section>
            </>
          )}
        </>
      )}
    </div>
  );
}

function StarterRow({ starter, open, onToggle }: { starter: GauntletStarterView; open: boolean; onToggle: () => void }) {
  const p = starter.player;
  const potentials = starter.potentials ?? [];
  const hasPotential = potentials.length > 0;
  return (
    <article className={`gauntlet-starter-row${open ? " open" : ""}`}>
      <div className="gauntlet-starter-main">
        <div className="gauntlet-starter-position">{starter.position}</div>
        <div className="gauntlet-starter-player">
          {p.image_url && <img src={p.image_url} alt="" loading="lazy" />}
          <div className="gauntlet-starter-player-text">
            <strong>{p.common_name || p.name}</strong>
            <span>
              {p.rating} OVR · GG {starter.rating.toFixed(1)}
            </span>
          </div>
        </div>
        {starter.card_slug && (
          <Link className="rank-link" to={`/time/${encodeURIComponent(starter.card_slug)}`}>
            abrir carta →
          </Link>
        )}
        <button
          type="button"
          className="rank-chevron"
          aria-expanded={open}
          aria-label={hasPotential ? "ver potenciais de evolução" : "sem potencial confirmado"}
          disabled={!hasPotential}
          onClick={onToggle}
        >
          {hasPotential ? <ExpandIcon expanded={open} /> : "—"}
        </button>
      </div>
      {open && hasPotential && (
        <div className="gauntlet-potential-detail">
          {potentials.map((potential, i) => (
            <div className="gauntlet-potential" key={i}>
              <div className="gauntlet-potential-head">
                <span className="gauntlet-potential-gain">
                  {formatSigned(potential.gg_rating_gain)} GG · final {potential.final_gg_rating.toFixed(1)}
                </span>
                <span>{potential.training_time || "tempo não informado"}</span>
              </div>
              {(potential.path?.chain?.length ?? 0) > 0 && (
                <div className="chain">via {potential.path.chain!.join(" → ")}</div>
              )}
              {(potential.gained_play_styles?.length ?? 0) > 0 && (
                <div className="gauntlet-potential-styles">ganha {styleNames(potential.gained_play_styles)}</div>
              )}
              <div className="gauntlet-potential-cost">
                {potential.coin_cost > 0 && <Chip tone="coin">{formatCoins(potential.coin_cost)} moedas</Chip>}
                {potential.point_cost > 0 && <Chip tone="cost">{potential.point_cost} pontos</Chip>}
                {potential.coin_cost <= 0 && potential.point_cost <= 0 && <Chip tone="gain">grátis</Chip>}
              </div>
            </div>
          ))}
        </div>
      )}
    </article>
  );
}

function BenchChit({ card }: { card: RosterCard }) {
  const p = card.player;
  const content = (
    <>
      {p.image_url && <img src={p.image_url} alt="" loading="lazy" />}
      <div className="gauntlet-bench-chit-text">
        <strong>{p.common_name || p.name}</strong>
        <span>{p.gg_rating ? `GG ${p.gg_rating.toFixed(1)}` : `${p.rating} OVR`}</span>
      </div>
    </>
  );
  return (
    <div className="gauntlet-bench-chit">
      {card.card_slug ? (
        <Link to={`/time/${encodeURIComponent(card.card_slug)}`}>{content}</Link>
      ) : (
        <div className="gauntlet-bench-chit-static">{content}</div>
      )}
    </div>
  );
}
