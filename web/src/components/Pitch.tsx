import { Link } from "react-router-dom";
import type { StarterCard } from "../types";
import "./Pitch.css";

interface PitchProps {
  formation: string;
  starters: StarterCard[];
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

export default function Pitch({ formation, starters }: PitchProps) {
  const rows = parseRows(formation);
  if (!rows || starters.length !== 11) return null;
  const sorted = starters.slice().sort((a, b) => a.index - b.index);
  const goalkeeper = sorted[0];
  let cursor = 1;
  const lines = rows.map((size) => {
    const line = sorted.slice(cursor, cursor + size);
    cursor += size;
    return line;
  });

  return <div className="pitch"><div className="pitch-field">
    {[...lines].reverse().map((line, index) => <div className="pitch-row" key={index}>
      {line.slice().reverse().map((card) => <PlayerChit key={`${card.index}-${card.player.id}`} card={card} />)}
    </div>)}
    <div className="pitch-row pitch-row-gk"><PlayerChit card={goalkeeper} /></div>
  </div></div>;
}

function PlayerChit({ card }: { card: StarterCard }) {
  const player = card.player;
  const rating = card.position_gg_rating ?? player.gg_rating;
  const label = `${player.common_name || player.name}, ${card.position}, ${rating ? `GG ${rating.toFixed(1)}` : "GG indisponível"}`;
  const content = <>
    {player.image_url && <img className="chit-image" src={player.image_url} alt="" loading="lazy" />}
    <span className="chit-gg">{rating ? rating.toFixed(1) : "—"}</span>
  </>;
  return <div className="pitch-chit">
    {card.card_slug ? <Link to={`/time/${card.card_slug}`} className="chit-link" aria-label={label}>{content}</Link> : <div className="chit-link" title={label}>{content}</div>}
  </div>;
}
