import type { GalleryGrade, GalleryRecord, GalleryReward } from "./types";

// Leitura compartilhada pela lista (/galeria) e pelo detalhe (/galeria/:id)
// da FUT Gallery: em que situação o conjunto está e que letra cada faixa
// representa. "Registrada" é o que o usuário informou que fez no jogo;
// "prevista" é o que o modelo do FUT.GG consegue provar com a coleção — as
// duas nunca se confundem na tela.

export const GRADES: Exclude<GalleryGrade, "">[] = ["D", "C", "B", "A", "S"];

export function gradeIndex(grade: GalleryGrade | undefined): number {
  return grade ? GRADES.indexOf(grade as Exclude<GalleryGrade, "">) : -1;
}

export function registeredGrade(row: GalleryRecord): GalleryGrade {
  return row.completion?.grade || "";
}

// Só há previsão quando o motor avaliou o conjunto por inteiro; uma nota
// "incompleta" não vira letra prevista (na dúvida, não afirma).
export function predictedGrade(row: GalleryRecord): GalleryGrade {
  if (row.evaluation.status === "final") return "S";
  return row.evaluation.status === "available" ? row.evaluation.grade : "";
}

export type GalleryKind = "done" | "up" | "flat" | "pend" | "miss";

export function galleryKind(row: GalleryRecord): GalleryKind {
  if (registeredGrade(row) === "S" || row.evaluation.status === "final") return "done";
  if (row.evaluation.status === "missing" || row.evaluation.filled < row.evaluation.required) return "miss";
  if (row.evaluation.status !== "available" || row.evaluation.coverage !== "confirmada" || (row.evaluation.warnings?.length ?? 0) > 0) return "pend";
  return gradeIndex(predictedGrade(row)) > gradeIndex(registeredGrade(row)) ? "up" : "flat";
}

export function kindLabel(row: GalleryRecord, kind = galleryKind(row)): string {
  switch (kind) {
    case "done": return "finalizada em S";
    case "up": return `avançar ${registeredGrade(row) || "—"} → ${predictedGrade(row)}`;
    case "flat": return predictedGrade(row) ? "sem avanço" : "previsão pendente";
    case "pend": return row.evaluation.status === "unavailable" ? "catálogo incompleto" : "cobertura parcial";
    case "miss": return `faltam ${Math.max(0, row.evaluation.required - row.evaluation.filled)} cartas`;
  }
}

// Estado de cada letra na escada D→S: registrada (cheia), alcançável pela
// previsão (contorno) ou futura (apagada).
export type LadderState = "reg" | "pred" | "fut";
export function ladderState(row: GalleryRecord, grade: GalleryGrade): LadderState {
  const index = gradeIndex(grade);
  if (index <= gradeIndex(registeredGrade(row))) return "reg";
  if (index <= gradeIndex(predictedGrade(row))) return "pred";
  return "fut";
}

export function scoreLine(row: GalleryRecord): string {
  if (!predictedGrade(row)) return "previsão pendente";
  if (row.evaluation.next_grade && row.evaluation.next_threshold) return `faltam ${formatPoints(Math.max(0, row.evaluation.next_threshold - row.evaluation.score))} para ${row.evaluation.next_grade}`;
  return registeredGrade(row) === "S" ? "S registrada" : "S prevista";
}

// topThreshold é o fim da barra de pontos: o limiar de S, ou a própria
// pontuação quando ela passa dele (ou quando o catálogo não publicou S).
export function topThreshold(row: GalleryRecord): number {
  return Math.max(row.set.thresholds?.S ?? 0, row.evaluation.score, 1);
}

export function rewardLabel(reward: GalleryReward): string {
  const name = reward.label || reward.type || "recompensa";
  const amount = reward.count && reward.count > 1 ? `${reward.count}× ` : "";
  return `${amount}${name}${reward.value ? ` · ${reward.value.toLocaleString("pt-BR")}` : ""}`;
}

// nextReward é a primeira faixa acima do resultado registrado que tem
// recompensa publicada — a que o usuário ganha se avançar.
export function nextReward(row: GalleryRecord): { grade: GalleryGrade; reward: GalleryReward } | undefined {
  const registered = gradeIndex(registeredGrade(row));
  for (const grade of GRADES) {
    if (gradeIndex(grade) <= registered) continue;
    const rewards = row.set.rewards?.[grade] ?? [];
    if (rewards.length) return { grade, reward: rewards[0]! };
  }
  return undefined;
}

export function formatPoints(value: number): string {
  return value.toLocaleString("pt-BR");
}

export function kiloPoints(value: number | undefined): string {
  if (value === undefined) return "—";
  return value >= 1000 ? `${Math.round(value / 1000)}k` : String(value);
}

// tagAbbrev encurta o nome da tag para o selo da carta sugerida ("First
// Owner" → FO, "Multiples" → MUL); o nome inteiro vai no title.
export function tagAbbrev(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  if (words.length > 1) return words.map((word) => word[0]!.toUpperCase()).join("").slice(0, 3);
  return (words[0] ?? "").slice(0, 3).toUpperCase();
}
