import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

export interface TrendPoint {
  label: string;
  value: number;
}

interface TrendChartProps {
  data: TrendPoint[];
  color?: string;
  height?: number;
  // compact tira eixo, grade e tooltip — o sparkline usado ao lado de uma
  // linha de tabela (preço de um alvo de mercado), não um gráfico de leitura.
  compact?: boolean;
  valueFormatter?: (v: number) => string;
}

// TrendChart é o único componente de gráfico do app — status usa para nota
// do elenco e saldo ao longo de 30 dias, a carta usa para o preço dela, o
// mercado usa em modo compacto para o preço de cada alvo. As cores vêm de
// var(--...) do theme.css de propósito: SVG presentation attribute aceita
// custom property, então o gráfico já nasce no tema claro/escuro certo sem
// duplicar paleta nenhuma aqui.
export default function TrendChart({
  data,
  color = "var(--turf)",
  height = 160,
  compact = false,
  valueFormatter,
}: TrendChartProps) {
  if (data.length < 2) {
    return compact ? (
      <span className="chart-empty-inline">—</span>
    ) : (
      <div className="chart-empty" style={{ height }}>
        sem histórico suficiente ainda
      </div>
    );
  }

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data} margin={compact ? { top: 2, right: 2, bottom: 2, left: 2 } : { top: 6, right: 14, bottom: 0, left: 0 }}>
        {!compact && <CartesianGrid stroke="var(--rule)" vertical={false} />}
        {!compact && (
          <XAxis dataKey="label" tick={{ fontSize: 11, fill: "var(--ink-3)" }} axisLine={{ stroke: "var(--rule)" }} tickLine={false} />
        )}
        {!compact && (
          <YAxis
            tick={{ fontSize: 11, fill: "var(--ink-3)" }}
            axisLine={false}
            tickLine={false}
            width={46}
            tickFormatter={valueFormatter}
          />
        )}
        {!compact && (
          <Tooltip
            contentStyle={{ background: "var(--panel)", border: "1px solid var(--rule)", borderRadius: 6, fontSize: 12 }}
            labelStyle={{ color: "var(--ink-2)" }}
            itemStyle={{ color: "var(--ink)" }}
            formatter={(v) => {
              const n = typeof v === "number" ? v : Number(v);
              return [valueFormatter ? valueFormatter(n) : n, ""];
            }}
          />
        )}
        <Line
          type="monotone"
          dataKey="value"
          stroke={color}
          strokeWidth={2}
          dot={compact ? false : { r: 2, fill: color, strokeWidth: 0 }}
          activeDot={compact ? false : { r: 4 }}
          isAnimationActive={false}
        />
      </LineChart>
    </ResponsiveContainer>
  );
}
