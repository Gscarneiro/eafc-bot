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
  const chem = card.chemistry;
  // Fora de posição é a ÚNICA forma de perder entrosamento sob o modelo
  // padrão (ver internal/chemistry) — é o dado mais acionável que existe
  // aqui, então ganha destaque visual próprio, não só um número a mais.
  const chemLabel = chem ? (chem.fora_de_posicao ? "fora de posição — sem entrosamento" : `entrosamento ${chem.pontos}/3`) : "";
  const label = `${player.common_name || player.name}, ${card.position}, ${rating ? `GG ${rating.toFixed(1)}` : "GG indisponível"}${chemLabel ? `, ${chemLabel}` : ""}`;
  const content = <>
    {player.image_url && <img className="chit-image" src={player.image_url} alt="" loading="lazy" />}
    {!player.image_url && <span className="chit-fallback"><strong>{player.common_name || player.name}</strong><small>{card.position}</small></span>}
    {chem && (
      <span className={`chit-chem${chem.fora_de_posicao ? " out-of-position" : ""}`} title={chemLabel} aria-hidden="true">
        {chem.fora_de_posicao ? "!" : chem.pontos}
      </span>
    )}
    <span className="chit-gg">{rating ? rating.toFixed(1) : "—"}</span>
  </>;
  return <div className="pitch-chit">
    {card.card_slug ? <Link to={`/time/${card.card_slug}`} className="chit-link" aria-label={label}>{content}</Link> : <div className="chit-link" title={label}>{content}</div>}
  </div>;
}
