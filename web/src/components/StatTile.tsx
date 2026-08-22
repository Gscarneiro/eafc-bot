import type { ReactNode } from "react";

interface StatTileProps {
  label: string;
  value: ReactNode;
  sub?: ReactNode;
  tone?: "coin" | "up" | "down" | "name";
}

// StatTile é o retângulo "rótulo em cima, número grande embaixo" repetido
// no .stat-grid de Status — a única variação real entre usos é a cor do
// valor (dinheiro, alta, baixa) ou, pro nome do elo mais fraco, trocar o
// numeral de placar por um peso de texto normal (era .tile-name com
// !important; a variante `name` resolve isso sem sobrescrever à força).
export default function StatTile({ label, value, sub, tone }: StatTileProps) {
  return (
    <div className="stat-tile">
      <div className="label">{label}</div>
      <div className={`value${tone ? ` ${tone}` : ""}`}>{value}</div>
      {sub && <div className="sub">{sub}</div>}
    </div>
  );
}
