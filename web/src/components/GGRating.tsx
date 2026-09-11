import type { Position } from "../types";
import "./GGRating.css";

export type GGRatingVariant = "inline" | "pitch" | "detail";

interface GGRatingProps {
  current?: number | null;
  currentPosition?: Position;
  positional?: number | null;
  positionalPosition?: Position;
  variant?: GGRatingVariant;
}

export function isKnownGGRating(value?: number | null): value is number {
  return typeof value === "number" && Number.isFinite(value) && value > 0;
}

export function formatGGRating(value?: number | null): string {
  return isKnownGGRating(value) ? value.toFixed(1) : "—";
}

export function shouldShowPositionalGGRating(current?: number | null, positional?: number | null): boolean {
  if (!isKnownGGRating(positional)) return false;
  if (!isKnownGGRating(current)) return true;
  // Se a cópia já supera a referência posicional, a referência menor só
  // acrescentaria ruído. Quando a referência é maior, mantemos as duas para
  // deixar explícito o espaço que ainda pode ser ganho naquela posição.
  return positional > current && formatGGRating(current) !== formatGGRating(positional);
}

export default function GGRating({ current, currentPosition, positional, positionalPosition, variant = "inline" }: GGRatingProps) {
  // No campo, a cor "elo mais fraco" é decidida pelo GG da vaga física.
  // Mostrar ao lado o GG geral da melhor posição fazia o número contradizer a
  // própria cor do card. Fora do campo, mantemos os dois contextos visíveis.
  const visibleRating = variant === "pitch" && isKnownGGRating(positional) ? positional : current;
  const visiblePosition = variant === "pitch" && isKnownGGRating(positional) ? positionalPosition : currentPosition;
  const currentText = formatGGRating(visibleRating);
  const showPositional = variant !== "pitch" && shouldShowPositionalGGRating(current, positional);
  const positionalText = formatGGRating(positional);
  const currentLabel = `${variant === "pitch" && isKnownGGRating(positional) ? "GG na vaga" : "GG atual"} ${currentText}${visiblePosition ? ` · ${visiblePosition}` : ""}`;
  const positionalLabel = `GG posicional ${positionalText}${positionalPosition ? ` · ${positionalPosition}` : ""}`;

  return (
    <span className={`gg-rating gg-rating-${variant}`} aria-label={`${currentLabel}${showPositional ? `, ${positionalLabel}` : ""}`}>
      <span className="gg-rating-current">
        <span className="gg-rating-label">atual</span>
        <strong>{currentText}</strong>
        {variant !== "pitch" && visiblePosition && <small>{visiblePosition}</small>}
      </span>
      {showPositional && (
        <span className="gg-rating-positional">
          <span className="gg-rating-label">pos. {positionalPosition ?? "—"}</span>
          <strong>{positionalText}</strong>
        </span>
      )}
    </span>
  );
}
