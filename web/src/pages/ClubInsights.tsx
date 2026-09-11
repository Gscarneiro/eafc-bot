import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatCoins, formatDateTime, formatSigned } from "../format";
import type { ClubInsight, CollectionCard } from "../types";
import { useCollection } from "../useCollection";
import "../shared.css";
import "./ClubInsights.css";

const CONFIDENCE_TONE: Record<string, ChipTone> = { confirmada: "gain", estimada: "alert", incompleta: "alert" };

export default function ClubInsights() {
  const insights = useCollection<ClubInsight>("/api/clube/insights", { defaultOrderBy: [{ field: "kind", desc: false }], pageSize: 20 });
  const collection = useCollection<CollectionCard>("/api/clube/colecao", { defaultOrderBy: [{ field: "player/rating", desc: true }], pageSize: 12 });
  const error = insights.error ?? collection.error;
  const gate = asyncGate(insights.loading || collection.loading, error, insights.raw !== null && collection.raw !== null, () => { insights.refetch(); collection.refetch(); });
  if (gate) return gate;
  const rows = collection.rows;
  const scoreInsight = insights.rows.find((item) => item.avaliacao);
  const components = scoreInsight?.avaliacao?.componentes ?? [];
  const max = Math.max(...components.map((component) => Math.abs(component.valor)), 1);
  return <div className="wrap club-insights">
    <PageHeader eyebrow="elenco · memória" title="O que o clube prova" meta="A nota usa a fonte ativa, com memória da coleção e valor observado fora do XI." />
    <div className="banner">A fonte escolhida governa esta leitura. Cada componente mostra como a nota foi formada na posição avaliada.</div>
    {scoreInsight?.avaliacao ? <section className="score-ledger"><div className="score-ledger-title"><span>nota reproduzível</span><h2>{scoreInsight.headline}</h2><p>{scoreInsight.detail}</p><div><Chip tone={CONFIDENCE_TONE[scoreInsight.confidence] ?? "flat"}>{scoreInsight.confidence}</Chip><Chip tone="flat">{scoreInsight.avaliacao.contexto.fonte}{scoreInsight.avaliacao.contexto.perfil ? ` · ${scoreInsight.avaliacao.contexto.perfil}` : ""}</Chip></div></div><div className="score-total"><span>nota ativa</span><strong>{scoreInsight.avaliacao.nota?.toFixed(1) ?? "—"}</strong><small>{scoreInsight.avaliacao.contexto.ciclo || "ciclo não informado"} · {scoreInsight.avaliacao.contexto.posicao}</small></div><div className="score-components" aria-label="componentes da nota ativa">{components.map((component) => <div className="score-component" key={component.chave}><span>{component.rotulo}</span><div className="score-bar"><i className={component.valor < 0 ? "negative" : ""} style={{ width: `${Math.abs(component.valor) / max * 100}%` }} /></div><strong>{formatSigned(component.valor)}</strong></div>)}</div>{scoreInsight.avaliacao.dados_ausentes?.length ? <p className="score-missing">Dados faltando: {scoreInsight.avaliacao.dados_ausentes.join(", ")}.</p> : null}{scoreInsight.avaliacao.limitacoes?.length ? <p className="score-missing">Limitações: {scoreInsight.avaliacao.limitacoes.join(", ")}.</p> : null}</section> : <EmptyState message="Ainda não há nota ativa disponível neste retrato." hint="Uma coleta com a cobertura da fonte escolhida libera o breakdown." />}
    <section className="insight-notes"><h2>Leituras do retrato</h2><div className="insight-grid">{insights.rows.filter((item) => item !== scoreInsight).map((item) => <article key={item.kind} className="insight-note"><div><Chip tone={CONFIDENCE_TONE[item.confidence] ?? "flat"}>{item.confidence}</Chip><strong>{item.headline}</strong></div><p>{item.detail}</p>{item.fodder_value ? <small>{formatCoins(item.fodder_value.net_coins)} líquidos observados · {item.fodder_value.missing_prices} sem cotação</small> : null}{item.source && item.observed_at ? <small>fonte {item.source} · {formatDateTime(item.observed_at)}</small> : null}</article>)}</div></section>
    <section className="collection-section"><div className="collection-heading"><div><h2>Coleção aproximada · {collection.count}</h2><p>Contagem por versão da carta; cópias físicas sem identidade confirmada permanecem aproximadas.</p></div></div>{rows.length === 0 ? <EmptyState message="Nenhuma carta no último retrato." /> : <div className="collection-grid">{rows.map((item) => <article className="collection-card" key={item.player.id}><div className="collection-card-top"><div><strong>{item.player.common_name || item.player.name}</strong><span>{item.player.rating} · {item.player.position} · {item.player.version}</span></div><b>×{item.count}</b></div><div className="collection-meta"><Chip tone={item.identity === "confirmada" ? "gain" : "alert"}>{item.identity === "confirmada" ? "cópia confirmada" : "cópia aproximada"}</Chip>{item.protected ? <Chip tone="alert">protegida</Chip> : null}{item.fodder_candidate ? <Chip tone="flat">fora do XI</Chip> : null}</div><p>{item.permanence_days > 0 ? `observada há ${item.permanence_days} dias` : "primeiro retrato observado"} · origem {item.origin}</p></article>)}</div>}</section>
  </div>;
}
