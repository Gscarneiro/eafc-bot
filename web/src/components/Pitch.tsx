import { Link } from "react-router-dom";
import type { StarterCard } from "../types";
import "./Pitch.css";

interface PitchProps {
  formation: string;
  starters: StarterCard[];
}

// parseRows lê "4-2-3-1" / "4-3-3(4)" como linhas de defesa pra ataque — o
// sufixo entre parênteses é uma variação tática do fut.gg que não muda a
// contagem de linhas, só é descartado. Undefined quando a notação não
// fecha em 10 jogadores de linha (+ 1 goleiro) — Time cai pra tabela nesse
// caso, sem drama, em vez de desenhar um campo torto.
function parseRows(formation: string): number[] | undefined {
  const clean = formation.replace(/\(.*\)/, "").trim();
  if (!clean) return undefined;
  const parts = clean.split("-").map((p) => Number.parseInt(p, 10));
  if (parts.length === 0 || parts.some((n) => !Number.isFinite(n) || n <= 0)) return undefined;
  if (parts.reduce((a, b) => a + b, 0) !== 10) return undefined;
  return parts;
}

// canDrawPitch deixa a tela decidir ANTES de renderizar se vale a pena
// mostrar o toggle "escalação" — evita desenhar o botão pra uma visão que
// vai cair em null.
export function canDrawPitch(formation: string, startersCount: number): boolean {
  return parseRows(formation) !== undefined && startersCount === 11;
}

// ggBand classifica a faixa de GG Rating pra tingir a lateral do chit —
// GG Rating é a nota de comparação DENTRO do elenco (ver CLAUDE.md); nunca
// usar o overall da EA aqui. Sem GG Rating (demo, ou carta nova sem dado),
// a faixa fica neutra — a dúvida não vira afirmação.
type Band = "high" | "mid" | "low" | "unknown";
function ggBand(rating: number | undefined): Band {
  if (!rating) return "unknown";
  if (rating >= 85) return "high";
  if (rating >= 75) return "mid";
  return "low";
}

// Pitch desenha o XI na formação de verdade — a peça central do /time.
// A trapaça honesta: em vez de tentar decidir "isto é lateral direito, vai
// à direita" a partir do rótulo da posição (ambíguo com escalação fora de
// posição), a ordem horizontal segue simplesmente o `index` que o fut.gg já
// devolve (0 = goleiro crescendo em direção ao ataque — ver
// report.MainSquad) fatiado pelos tamanhos de linha da própria notação da
// formação. Não afirma lado nenhum que o dado não garante.
export default function Pitch({ formation, starters }: PitchProps) {
  const rows = parseRows(formation);
  if (!rows || starters.length !== 11) return null;

  const sorted = starters.slice().sort((a, b) => a.index - b.index);
  const gk = sorted[0];
  let cursor = 1;
  const lines = rows.map((size) => {
    const line = sorted.slice(cursor, cursor + size);
    cursor += size;
    return line;
  });
  // Ataque no topo, goleiro embaixo — a leitura de quem olha para o
  // próprio gol, como uma prancheta de escalação.
  const attackFirst = [...lines].reverse();

  return (
    <div className="pitch">
      <div className="pitch-field">
        {attackFirst.map((line, i) => (
          <div className="pitch-row" key={i}>
            {line.map((s) => (
              <PlayerChit key={s.player.id} card={s} />
            ))}
          </div>
        ))}
        <div className="pitch-row pitch-row-gk">
          <PlayerChit card={gk} />
        </div>
      </div>
    </div>
  );
}

function PlayerChit({ card }: { card: StarterCard }) {
  const p = card.player;
  const band = ggBand(p.gg_rating);
  const name = p.common_name || p.name;
  const ggLabel = p.gg_rating ? `GG ${p.gg_rating.toFixed(0)}` : "GG —";

  const inner = (
    <>
      <span className="chit-rating">{p.rating}</span>
      <span className="chit-name">{name}</span>
      <span className="chit-pos">
        {card.position}
        {p.out_of_pos ? " *" : ""} · {ggLabel}
      </span>
    </>
  );

  return (
    <div className={`pitch-chit band-${band}`}>
      {card.card_slug ? (
        <Link to={`/time/${card.card_slug}`} className="chit-link">
          {inner}
        </Link>
      ) : (
        <div className="chit-link" title="Abaixo do overall mínimo analisado carta a carta">
          {inner}
        </div>
      )}
    </div>
  );
}
