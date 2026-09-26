import { formatCoins, formatDate, formatSigned } from "../format";
import type { Attributes, FaixaOVR, PositionMapRow, SlotOutlook, StarterCard, ValorXI } from "../types";

// TimeLeitura é a "Leitura do elenco" do Meu time: cinco gráficos
// derivados do MESMO snapshot de /api/time — nenhum busca dado próprio.
// Onde o dado não prova a leitura, o gráfico diz isso em vez de desenhar
// um número inventado (ex.: o perfil do XI não traz um "ideal da função",
// porque o bot não tem essa referência publicada para comparar).

type Props = {
  starters: StarterCard[];
  formation: string;
  distribution: FaixaOVR[];
  valorXI?: ValorXI;
  outlook: SlotOutlook[];
  positionMap: PositionMapRow[];
  weakestIndex?: number;
};

export default function TimeLeitura({ starters, formation, distribution, valorXI, outlook, positionMap, weakestIndex }: Props) {
  return <section className="time-section" id="leitura">
    <div className="section-title-row">
      <div><h2>Leitura do elenco</h2><p className="section-note">Tudo derivado do mesmo snapshot do Meu time — nenhum gráfico busca dado próprio.</p></div>
    </div>
    <div className="time-reading-grid">
      <ProfilePanel starters={starters} formation={formation} />
      <DistributionPanel distribution={distribution} />
      <ValuePanel valor={valorXI} />
      <LinksPanel starters={starters} />
      <GainPanel outlook={outlook} positionMap={positionMap} weakestIndex={weakestIndex} />
    </div>
  </section>;
}

function Panel({ title, sub, meta, wide, children }: { title: string; sub?: string; meta?: React.ReactNode; wide?: boolean; children: React.ReactNode }) {
  return <div className={`panel${wide ? " time-reading-wide" : ""}`}>
    <div className="panel-head"><span>{title}{sub ? <span className="panel-head-sub"> / {sub}</span> : null}</span>{meta ? <span className="panel-head-meta">{meta}</span> : null}</div>
    <div className="panel-body">{children}</div>
  </div>;
}

const ATTRS: [keyof Attributes, string][] = [["pace", "PAC"], ["shooting", "SHO"], ["passing", "PAS"], ["dribbling", "DRI"], ["defending", "DEF"], ["physical", "FIS"]];

// ProfilePanel é a média dos seis atributos de face dos 10 de linha. O
// goleiro fica de fora: reaproveita os mesmos campos com outro significado
// (DIV, HAN, KIC, REF, SPD, POS — ver CLAUDE.md), e somar um no outro daria
// um "ritmo" que não existe.
function ProfilePanel({ starters, formation }: { starters: StarterCard[]; formation: string }) {
  const outfield = starters.filter((s) => s.position !== "GK" && s.player.position !== "GK" && s.player.attributes);
  if (outfield.length === 0) return <Panel title="Perfil do XI" sub="média dos jogadores de linha"><p className="hint">Sem atributos dos titulares nesta coleta.</p></Panel>;
  const values = ATTRS.map(([key, label]) => ({ label, value: outfield.reduce((total, s) => total + (s.player.attributes[key] ?? 0), 0) / outfield.length }));
  const lowest = values.reduce((min, item) => (item.value < min.value ? item : min), values[0]!);
  const R = 70, C = 80, floor = 40;
  const point = (index: number, radius: number) => {
    const angle = -Math.PI / 2 + (index * Math.PI) / 3;
    return `${(C + radius * Math.cos(angle)).toFixed(1)},${(C + radius * Math.sin(angle)).toFixed(1)}`;
  };
  const ring = (radius: number) => ATTRS.map((_, index) => point(index, radius)).join(" ");
  const shape = values.map((item, index) => point(index, Math.max(0, Math.min(1, (item.value - floor) / (99 - floor))) * R)).join(" ");
  return <Panel title="Perfil do XI" sub={`média dos ${outfield.length} de linha${formation ? ` · ${formation}` : ""}`}>
    <div className="time-profile">
      <svg viewBox="0 0 160 160" role="img" aria-label="Atributos médios do XI">
        <polygon points={ring(R)} className="grid" />
        <polygon points={ring(R / 2)} className="grid" />
        {ATTRS.map((_, index) => <line key={index} x1={C} y1={C} x2={point(index, R).split(",")[0]} y2={point(index, R).split(",")[1]} className="grid" />)}
        <polygon points={shape} className="shape" />
        {ATTRS.map(([, label], index) => { const [lx, ly] = point(index, R + 9).split(","); return <text key={label} x={lx} y={ly} textAnchor="middle" dominantBaseline="middle">{label}</text>; })}
      </svg>
      <div className="time-profile-list">
        <div className="time-profile-head"><span>atributo</span><span>média XI</span></div>
        {values.map((item) => <div key={item.label}><span className={item === lowest ? "low" : ""}>{item.label}</span><span>{item.value.toFixed(0)}</span></div>)}
        <p>Escala a partir de {floor}. Em vermelho, o atributo mais baixo do XI — o goleiro usa outra escala e fica fora da média.</p>
      </div>
    </div>
  </Panel>;
}

function DistributionPanel({ distribution }: { distribution: FaixaOVR[] }) {
  // Tira só as pontas vazias; uma faixa vazia no meio continua desenhada,
  // porque o buraco também é leitura.
  let first = distribution.findIndex((f) => f.total > 0);
  let last = distribution.length - 1 - [...distribution].reverse().findIndex((f) => f.total > 0);
  if (first < 0) { first = 0; last = -1; }
  const bands = distribution.slice(first, last + 1);
  const total = distribution.reduce((sum, f) => sum + f.total, 0);
  const max = Math.max(1, ...bands.map((f) => f.total));
  return <Panel title="Distribuição do clube" sub={`${total} cartas por OVR`} meta="XI em destaque">
    {bands.length === 0 ? <p className="hint">Clube vazio nesta coleta.</p> : <>
      <div className="time-bars">
        {bands.map((band) => <div key={band.faixa} className={band.no_xi > 0 ? "xi" : ""} title={`${band.total} cartas${band.no_xi ? ` · ${band.no_xi} no XI` : ""}`}>
          <span className="time-bars-value">{band.total}</span>
          <span className="time-bars-col" style={{ height: `${Math.max(2, (band.total / max) * 100)}%` }} />
          <span className="time-bars-label">{band.faixa}</span>
        </div>)}
      </div>
      <div className="time-mini-legend"><span><i className="flat" /> fora do XI</span><span><i className="turf" /> faixa com titular</span></div>
    </>}
  </Panel>;
}

function compactCoins(value: number): string {
  if (value >= 1_000_000) return `${(value / 1_000_000).toLocaleString("pt-BR", { maximumFractionDigits: 2 })}M`;
  if (value >= 10_000) return `${Math.round(value / 1000).toLocaleString("pt-BR")}k`;
  return formatCoins(value);
}

function ValuePanel({ valor }: { valor?: ValorXI }) {
  const points = valor?.pontos ?? [];
  const lastValue = points.at(-1)?.moedas;
  const firstValue = points[0]?.moedas;
  const change = points.length >= 2 && firstValue ? ((lastValue! - firstValue) / firstValue) * 100 : undefined;
  const meta = lastValue !== undefined ? <>{compactCoins(lastValue)} moedas{change !== undefined ? <> · <b className={change >= 0 ? "up" : "down"}>{change >= 0 ? "+" : ""}{change.toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%</b></> : null}</> : undefined;
  const footnote = valor ? `${valor.cotadas} titulares cotados${valor.sem_cotacao ? ` · ${valor.sem_cotacao} fora da soma (untradeable ou sem cotação)` : ""}. Só entram os dias em que todos os cotados têm preço.` : "";
  if (!valor) return <Panel title="Valor do XI" sub="30 dias" wide><p className="hint">Histórico de preço indisponível nesta leitura.</p></Panel>;
  if (points.length < 2) return <Panel title="Valor do XI" sub="30 dias" meta={meta} wide><p className="hint">Histórico curto demais para desenhar a curva — a série cresce uma coleta por dia. {footnote}</p></Panel>;
  const values = points.map((p) => p.moedas);
  const min = Math.min(...values), max = Math.max(...values);
  const span = max - min || Math.max(1, max * 0.02);
  const W = 600, H = 130;
  const x = (index: number) => (index / (points.length - 1)) * W;
  const y = (value: number) => 12 + (1 - (value - min) / span) * (H - 24);
  const line = points.map((p, index) => `${x(index).toFixed(1)},${y(p.moedas).toFixed(1)}`).join(" ");
  const labelIndexes = [...new Set([0, Math.round((points.length - 1) / 4), Math.round((points.length - 1) / 2), Math.round(((points.length - 1) * 3) / 4), points.length - 1])];
  return <Panel title="Valor do XI" sub="30 dias" meta={meta} wide>
    <svg viewBox={`0 0 ${W} ${H}`} preserveAspectRatio="none" className="time-value-chart" role="img" aria-label={`Valor do XI de ${formatCoins(firstValue!)} a ${formatCoins(lastValue!)} moedas`}>
      {[20, 65, 110].map((gy) => <line key={gy} x1="0" y1={gy} x2={W} y2={gy} className="grid" />)}
      <polygon points={`${line} ${W},${H} 0,${H}`} className="area" />
      <polyline points={line} className="line" />
    </svg>
    <div className="time-value-axis">{labelIndexes.map((index) => <span key={index}>{formatDate(`${points[index]!.dia}T12:00:00Z`)}</span>)}</div>
    <p className="time-footnote">{footnote}</p>
  </Panel>;
}

type LinkGroups = { linked: { name: string; count: number }[]; loose: number };
function linkGroups(starters: StarterCard[], field: "club" | "league" | "nation"): LinkGroups {
  const counts = new Map<string, number>();
  let loose = 0;
  for (const s of starters) {
    const value = s.player[field];
    if (!value) { loose++; continue; }
    counts.set(value, (counts.get(value) ?? 0) + 1);
  }
  const linked = [...counts].filter(([, count]) => count > 1).map(([name, count]) => ({ name, count })).sort((a, b) => b.count - a.count);
  for (const [, count] of counts) if (count === 1) loose++;
  return { linked, loose };
}

// LinksPanel mostra como os 11 se agrupam por clube, liga e nação: blocos
// claros são grupos de 2+ (somam entrosamento), cinza são cartas sem par.
function LinksPanel({ starters }: { starters: StarterCard[] }) {
  const rows: [string, "club" | "league" | "nation"][] = [["Clube", "club"], ["Liga", "league"], ["Nação", "nation"]];
  return <Panel title="Química por vínculo" sub={`${starters.length} titulares`}>
    <div className="time-links">
      {rows.map(([label, field]) => {
        const groups = linkGroups(starters, field);
        const summary = [...groups.linked.map((g) => String(g.count)), groups.loose ? `${groups.loose} soltos` : ""].filter(Boolean).join(" · ");
        return <div key={field}>
          <div className="time-links-head"><span>{label}</span><span>{summary || "—"}</span></div>
          <div className="time-links-bar">
            {groups.linked.map((g, index) => <span key={g.name} className={index === 0 ? "top" : "linked"} style={{ flex: g.count }} title={`${g.name} ×${g.count}`} />)}
            {groups.loose ? <span className="loose" style={{ flex: groups.loose }} title={`${groups.loose} sem par`} /> : null}
          </div>
        </div>;
      })}
      <p className="time-footnote">Blocos claros somam entrosamento; cinza são cartas sem vínculo no XI de hoje.</p>
    </div>
  </Panel>;
}

// GainPanel lê o mesmo SlotOutlook do mapa de posições: quanto o melhor
// reserva do clube acrescentaria em cada vaga, quais vagas só têm resposta
// no mercado sem cotação e quantas já estão no teto do clube.
function GainPanel({ outlook, positionMap, weakestIndex }: { outlook: SlotOutlook[]; positionMap: PositionMapRow[]; weakestIndex?: number }) {
  const gains = outlook.filter((o) => o.kind === "melhor_disponivel" && (o.delta ?? 0) > 0).sort((a, b) => (b.delta ?? 0) - (a.delta ?? 0));
  const unpriced = outlook.filter((o) => o.kind === "sem_cotacao");
  const ceiling = outlook.filter((o) => o.kind === "teto").length;
  const total = gains.reduce((sum, o) => sum + (o.delta ?? 0), 0);
  const max = Math.max(0.1, ...gains.map((o) => o.delta ?? 0));
  const positionOf = (o: SlotOutlook) => positionMap.find((row) => row.index === o.index)?.position ?? o.position;
  return <Panel title="Ganho disponível" sub="por vaga" meta={gains.length ? `${formatSigned(total)} no total` : undefined}>
    <div className="time-gain">
      {gains.length === 0 && unpriced.length === 0 ? <p className="hint">Nenhuma vaga tem reserva melhor no clube hoje.</p> : null}
      {gains.map((o) => <div key={o.index} className={o.index === weakestIndex ? "cost" : "alert"}>
        <span className="time-gain-pos">{positionOf(o)}</span>
        <span className="time-gain-track"><span style={{ width: `${((o.delta ?? 0) / max) * 100}%` }} /></span>
        <span className="time-gain-value">{formatSigned(o.delta ?? 0)}</span>
      </div>)}
      {unpriced.map((o) => <div key={o.index} className="unpriced">
        <span className="time-gain-pos">{positionOf(o)}</span>
        <span className="time-gain-track"><span /></span>
        <span className="time-gain-value">s/ cot.</span>
      </div>)}
      <p className="time-footnote">{ceiling > 0 ? `${ceiling} ${ceiling === 1 ? "vaga está" : "vagas estão"} no teto: nem o banco nem o mercado cotado superam o titular.` : "Nenhuma vaga no teto."}{unpriced.length ? " Hachurado: o mercado tem alvo para a vaga, mas sem cotação nesta coleta." : ""}</p>
    </div>
  </Panel>;
}
