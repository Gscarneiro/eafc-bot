import type { ReactNode } from "react";

// As mesmas 6 variantes de .chip (theme.css) — cada uma com UM
// significado: turf é posição/estrutura, coin é dinheiro parado, gain/cost
// são variação, alert é atenção, flat é neutro. Nenhuma tela deveria
// escrever `className="chip X"` na mão a partir daqui.
export type ChipTone = "turf" | "coin" | "gain" | "cost" | "alert" | "flat";

export default function Chip({ tone, children }: { tone: ChipTone; children: ReactNode }) {
  return <span className={`chip ${tone}`}>{children}</span>;
}
