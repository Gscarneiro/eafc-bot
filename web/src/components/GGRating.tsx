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
  return formatGGRating(current) !== formatGGRating(positional);
}

export default function GGRating({ current, currentPosition, positional, positionalPosition, variant = "inline" }: GGRatingProps) {
  const currentText = formatGGRating(current);
  const showPositional = shouldShowPositionalGGRating(current, positional);
  const positionalText = formatGGRating(positional);
  const currentLabel = `GG atual ${currentText}${currentPosition ? ` · ${currentPosition}` : ""}`;
  const positionalLabel = `GG posicional ${positionalText}${positionalPosition ? ` · ${positionalPosition}` : ""}`;

  return (
    <span className={`gg-rating gg-rating-${variant}`} aria-label={`${currentLabel}${showPositional ? `, ${positionalLabel}` : ""}`}>
      <span className="gg-rating-current">
        <span className="gg-rating-label">atual</span>
        <strong>{currentText}</strong>
        {variant !== "pitch" && currentPosition && <small>{currentPosition}</small>}
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
