import type { ReactNode } from "react";
import type { ClubPlayer, Formation } from "../types";
import "./FutPitch.css";

// FutPitch desenha o mesmo gramado em Meu time, Editor e Gauntlet. As
// coordenadas vêm do catálogo do serviço; cada tela decide se a carta é
// link, botão ou alvo de soltar.

export type FutTone = "turf" | "alert" | "cost" | "coin";
export type Spot = { left: number; top: number };

// O índice físico é a chave. Não reordenamos vagas pelo nome da posição:
// algumas formações do fut.gg entregam RM, LM, CAM nessa ordem, embora CAM
// fique no centro. O catálogo guarda x/y e preserva a identidade da carta.
export function formationLayout<T extends { index: number }>(name: string, slots: T[], catalog: Formation[]): { ordered: T[]; spots: Map<number, Spot>; lines: number; dense: boolean } | null {
  const formation = catalog.find((entry) => entry.nome === name);
  if (!formation || formation.vagas.length !== 11 || slots.length !== 11) return null;
  const byIndex = new Map(slots.map((slot) => [slot.index, slot]));
  if (byIndex.size !== 11) return null;
  const spots = new Map<number, Spot>();
  for (const position of formation.vagas) {
    if (!byIndex.has(position.index) || !Number.isFinite(position.x) || !Number.isFinite(position.y)) return null;
    spots.set(position.index, { left: position.x, top: position.y });
  }
  if (spots.size !== 11) return null;
  const ordered = slots.slice().sort((a, b) => {
    const left = spots.get(a.index)!; const right = spots.get(b.index)!;
    return left.top - right.top || left.left - right.left;
  });
  const rows = new Map<number, number>();
  for (const slot of formation.vagas) rows.set(slot.y, (rows.get(slot.y) ?? 0) + 1);
  return { ordered, spots, lines: rows.size, dense: Math.max(...rows.values()) >= 5 };
}

// ratingScale é o domínio das barras de nota (mapa por vaga, "vs média",
// queda por rodada): arredonda para múltiplos de 5 em volta dos dados, com
// no mínimo 20 pontos de largura. O design fixava 75–95, mas um clube real
// pode ter nota fora disso — uma barra cravada em 0% não diz nada.
export function ratingScale(values: number[]): { low: number; high: number; pct: (value: number) => number } {
  const finite = values.filter((value) => Number.isFinite(value));
  const low = finite.length ? Math.floor(Math.min(...finite) / 5) * 5 - 5 : 75;
  const high = Math.max(finite.length ? Math.ceil(Math.max(...finite) / 5) * 5 : 95, low + 20);
  return { low, high, pct: (value) => Math.min(100, Math.max(0, ((value - low) / (high - low)) * 100)) };
}

export function spotStyle(spot: Spot | undefined) {
  return spot ? { left: `${spot.left}%`, top: `${spot.top}%` } : undefined;
}

export function FutPitch({ formation, lines, dense = false, className, label, children }: { formation?: string; lines: number; dense?: boolean; className?: string; label?: string; children: ReactNode }) {
  // Formação de 5 linhas (4-1-2-1-2): na altura padrão as cartas encavalam.
  const classes = ["fut-pitch", lines > 5 ? "tall" : "", dense ? "dense" : "", className ?? ""].filter(Boolean).join(" ");
  return <div className={classes} aria-label={label}>
    <span className="fut-pitch-mark outline" aria-hidden="true" />
    <span className="fut-pitch-mark halfway" aria-hidden="true" />
    <span className="fut-pitch-mark circle" aria-hidden="true" />
    <span className="fut-pitch-mark spot center" aria-hidden="true" />
    <span className="fut-pitch-mark box top" aria-hidden="true" />
    <span className="fut-pitch-mark box small top" aria-hidden="true" />
    <span className="fut-pitch-mark spot penalty top" aria-hidden="true" />
    <span className="fut-pitch-mark box bottom" aria-hidden="true" />
    <span className="fut-pitch-mark box small bottom" aria-hidden="true" />
    <span className="fut-pitch-mark spot penalty bottom" aria-hidden="true" />
    {formation ? <span className="fut-pitch-note left" aria-hidden="true">{formation}</span> : null}
    <span className="fut-pitch-note right" aria-hidden="true">ataque ↑</span>
    {children}
  </div>;
}

export function CardArt({ player, className }: { player: ClubPlayer; className?: string }) {
  return <span className={`fut-art${className ? ` ${className}` : ""}${player.image_url ? " has-image" : ""}`} aria-hidden="true">{player.image_url ? <img src={player.image_url} alt="" loading="lazy" /> : null}</span>;
}

// FutCardFace é o miolo da carta: OVR e posição na coluna, arte, nome,
// nota na vaga e os 3 pips de entrosamento. Sem nação de propósito: o
// fut.gg manda o nome inteiro ("Brazil") e abreviar na mão daria "SPA"
// para Espanha. pips undefined = entrosamento não calculado (todos vazios).
export function FutCardFace({ player, position, score, pips, badge, scoreTone }: { player: ClubPlayer; position: string; score?: number; pips?: number; badge?: string | null; scoreTone?: "alert" | "cost" | "" }) {
  return <>
    <span className="fut-card-body">
      <span className="fut-card-id"><b className="fut-card-ovr">{player.rating}</b><span className="fut-card-pos">{position}</span><i /></span>
      <CardArt player={player} className="fut-card-art" />
    </span>
    <span className="fut-card-name">{player.common_name || player.name}</span>
    <span className="fut-card-foot">
      <span className={`fut-card-score${scoreTone ? ` ${scoreTone}` : ""}`}>{score !== undefined ? score.toFixed(1) : "—"}</span>
      <span className="fut-card-pips" aria-hidden="true">{[0, 1, 2].map((pip) => <span key={pip} className={pips !== undefined && pip < pips ? "on" : ""} />)}</span>
    </span>
    {badge ? <span className="fut-card-badge">{badge}</span> : null}
  </>;
}

export function FutLegend({ items }: { items: [FutTone, string][] }) {
  return <div className="fut-legend">{items.map(([tone, text]) => <span key={text}><i className={tone} /> {text}</span>)}</div>;
}
