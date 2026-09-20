import { Link } from "react-router-dom";
import type { SlotOutlook, StarterCard } from "../types";
import GGRating, { formatGGRating } from "./GGRating";
import "./Pitch.css";

interface PitchProps {
  formation: string;
  starters: StarterCard[];
  sourceLabel?: string;
  // outlook e regua vêm de TimeResponse.slot_outlook/.regua — pra colorir
  // a borda de cada chit com a MESMA leitura que o painel "Mapa de
  // posições" usa (ver internal/analyze.SlotOutlook), nunca uma conta
  // própria da tela.
  outlook?: SlotOutlook[];
  weakestIndex?: number;
}

function parseRows(formation: string): number[] | undefined {
  const clean = formation.replace(/\(.*\)/, "").trim();
  if (!clean) return undefined;
  const parts = clean.split("-").map((part) => Number.parseInt(part, 10));
  if (parts.some((part) => !Number.isFinite(part) || part <= 0)) return undefined;
  return parts.reduce((total, part) => total + part, 0) === 10 ? parts : undefined;
}

export function canDrawPitch(formation: string, startersCount: number): boolean {
  return parseRows(formation) !== undefined && startersCount === 11;
}

export default function Pitch({ formation, starters, sourceLabel, outlook, weakestIndex }: PitchProps) {
  const rows = parseRows(formation);
  if (!rows || starters.length !== 11) return null;
  const sorted = starters.slice().sort((a, b) => a.index - b.index);
  const goalkeeper = sorted[0]!;
  let cursor = 1;
  const lines = rows.map((size) => {
    const line = sorted.slice(cursor, cursor + size);
    cursor += size;
    return line;
  });
  const outlookByIndex = new Map((outlook ?? []).map((o) => [o.index, o]));

  return (
    <div className="pitch-field">
      {[...lines].reverse().map((line, index) => (
        <div className="pitch-row" key={index}>
          {line.slice().reverse().map((card) => (
            <PlayerChit key={card.player.club_item_id || `${card.index}-${card.player.id}`} card={card} outlook={outlookByIndex.get(card.index)} isWeakest={card.index === weakestIndex} sourceLabel={sourceLabel} />
          ))}
        </div>
      ))}
      <div className="pitch-row pitch-row-gk">
        <PlayerChit card={goalkeeper} outlook={outlookByIndex.get(goalkeeper.index)} isWeakest={goalkeeper.index === weakestIndex} sourceLabel={sourceLabel} />
      </div>
    </div>
  );
}

function PlayerChit({ card, outlook, isWeakest, sourceLabel }: { card: StarterCard; outlook?: SlotOutlook; isWeakest: boolean; sourceLabel?: string }) {
  const player = card.player;
  const chem = card.chemistry;
  const tone = isWeakest ? "cost" : outlook?.kind === "melhor_disponivel" ? "alert" : "turf";
  const chemLabel = chem ? (chem.fora_de_posicao ? "fora de posição — sem entrosamento" : `entrosamento ${chem.pontos}/3`) : "sem entrosamento calculado";
  const ratingLabel = card.position_rating_unavailable ? `${sourceLabel ?? "GG Rating"} indisponível` : card.position_gg_rating ? `${sourceLabel ?? "GG Rating"} ${formatGGRating(card.position_gg_rating)}` : `GG ${formatGGRating(player.gg_rating)}`;
  const toneLabel = isWeakest ? `menor ${sourceLabel ?? "GG Rating"} na vaga` : outlook?.kind === "melhor_disponivel" ? "upgrade disponível no banco" : "acima da média do XI";
  const label = `${player.common_name || player.name}, ${card.position}, ${ratingLabel}, ${toneLabel}, ${chemLabel}`;

  const content = (
    <>
      <div className="chit-head">
        <span className="chit-pos">{card.position}</span>
        <span className="chit-ovr">{player.rating}</span>
      </div>
      <strong className="chit-name">{player.common_name || player.name}</strong>
      <div className="chit-foot">
        <span className={`chit-rating tone-${tone}`}>{card.position_rating_unavailable ? "—" : <GGRating current={player.gg_rating} currentPosition={player.gg_rating_pos} positional={card.position_gg_rating ?? undefined} positionalPosition={card.position} positionalLabel={sourceLabel} variant="pitch" />}</span>
        <span className="chit-chem" aria-hidden="true">
          {[0, 1, 2].map((i) => (
            <span key={i} className={chem && !chem.fora_de_posicao && i < chem.pontos ? "filled" : ""} />
          ))}
        </span>
      </div>
    </>
  );
  return (
    <div className={`pitch-chit tone-${tone}`}>
      {card.card_slug ? (
        <Link to={`/time/${card.card_slug}`} className="chit-link" aria-label={label}>
          {content}
        </Link>
      ) : (
        <div className="chit-link" title={label}>
          {content}
        </div>
      )}
    </div>
  );
}
