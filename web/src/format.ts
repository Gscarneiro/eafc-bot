// Os mesmos formatos que o relatório em html/template usava (funcs "coins",
// "signed", "styleNames" em internal/report/report.go e no extinto
// internal/cards/render.go) — só que em TypeScript agora.

export function formatCoins(v: number): string {
  const neg = v < 0;
  const digits = Math.abs(Math.trunc(v)).toString();
  let out = "";
  for (let i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 === 0) out += ".";
    out += digits[i];
  }
  return neg ? `-${out}` : out;
}

export function formatSigned(v: number): string {
  const sign = v >= 0 ? "+" : "";
  return `${sign}${v.toFixed(1)}`;
}

export function styleName(name: string, plus: boolean): string {
  return plus ? `${name}+` : name;
}

// Aceita null de propósito: o Go serializa slice zero-value como `null`,
// não `[]` (nenhum desses campos tem `omitempty`) — uma carta sem PlayStyle
// nenhum manda `"play_styles":null`, e é assim que ela chega aqui.
export function styleNames(styles: { name: string; plus: boolean }[] | null | undefined): string {
  return (styles ?? []).map((s) => styleName(s.name, s.plus)).join(" · ");
}

// formatDate mostra "21/08" — o suficiente para um eixo de gráfico ou uma
// linha de tabela; formatDateTime acrescenta a hora para quando o instante
// exato importa (o carimbo do job, a última coleta).
export function formatDate(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit" });
}

export function formatDateTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

// isZeroTime reconhece o zero-valor de time.Time do Go quando ele escapa
// sem "omitempty" — nenhum instante real cai no ano 1.
export function isZeroTime(iso: string | undefined | null): boolean {
  return !iso || iso.startsWith("0001-01-01");
}
