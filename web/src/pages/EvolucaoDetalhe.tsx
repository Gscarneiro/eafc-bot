import { useEffect, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { fetchEvolutionAnalysis, fetchEvolutionCatalogDetail, requestEvolutionAnalysis } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip, { type ChipTone } from "../components/Chip";
import PageHeader from "../components/PageHeader";
import { formatCoins } from "../format";
import { useData } from "../useData";
import type { EvolutionAnalysis, EvolutionCatalogDetailResponse, EvolutionNumberChange } from "../types";
import "../shared.css";
import "./Evolucoes.css";

function costLabel(data: EvolutionCatalogDetailResponse): string {
  const cost = data.item.cost;
  if (cost.free) return "grátis";
  const values: string[] = [];
  if (cost.coins > 0) values.push(`${formatCoins(cost.coins)} moedas`);
  if (cost.points > 0) values.push(`${formatCoins(cost.points)} pontos`);
  if (cost.tokens > 0) values.push(`${formatCoins(cost.tokens)} tokens`);
  return values.join(" · ") || "custo não informado";
}

function verdictLabel(verdict?: string): string {
  switch (verdict) {
    case "recomendada": return "recomendada";
    case "nao_recomendada": return "não recomendada";
    case "dados_insuficientes": return "dados insuficientes";
    default: return "situacional";
  }
}

function verdictTone(verdict?: string): ChipTone {
  if (verdict === "recomendada") return "gain";
  if (verdict === "nao_recomendada") return "alert";
  return "flat";
}

function ChangeRow({ change }: { change: EvolutionNumberChange }) {
  const before = Math.max(0, Math.min(99, change.before));
  const after = Math.max(0, Math.min(99, change.after));
  const style = { "--before": `${before}%`, "--after": `${after}%` } as React.CSSProperties;
  return <div className={`evo-change${change.available ? "" : " missing"}`}>
    <span className="evo-change-label" title={change.label}>{change.label}</span>
    <div className="evo-rail" style={style} aria-label={`${change.label}: ${change.available ? `${change.before} para ${change.after}` : "indisponível"}`}><span className="evo-rail-before" /><span className="evo-rail-after" /></div>
    <span className="evo-change-values">{change.available ? <><span>{change.before}</span> <strong>{change.after}</strong></> : "—"}</span>
    <span className="evo-delta">{change.available && change.delta > 0 ? `+${change.delta}` : ""}</span>
  </div>;
}

function grouped(changes: EvolutionNumberChange[]): [string, EvolutionNumberChange[]][] {
  const groups = new Map<string, EvolutionNumberChange[]>();
  for (const change of changes) groups.set(change.group, [...(groups.get(change.group) ?? []), change]);
  return [...groups.entries()];
}

export default function EvolucaoDetalhe() {
  const { slug = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const playerKey = searchParams.get("player_key") || "";
  const state = useData(() => fetchEvolutionCatalogDetail(slug, playerKey || undefined), [slug, playerKey]);
  const [analysis, setAnalysis] = useState<EvolutionAnalysis | undefined>();
  const [analysisError, setAnalysisError] = useState<string>("");
  const [requesting, setRequesting] = useState(false);
  const gate = asyncGate(state.loading, state.error, state.data !== null, state.refetch);
  const mainGroups = useMemo(() => grouped(state.data?.projection?.main_attributes ?? []), [state.data?.projection]);
  const detailedGroups = useMemo(() => grouped(state.data?.projection?.detailed_attributes ?? []), [state.data?.projection]);

  useEffect(() => {
    const latest = state.data?.analyses?.[0];
    if (latest && (!analysis || new Date(latest.updated_at).getTime() > new Date(analysis.updated_at).getTime())) setAnalysis(latest);
  }, [state.data, analysis]);

  useEffect(() => {
    if (!analysis || (analysis.status !== "queued" && analysis.status !== "running")) return;
    let cancelled = false;
    const poll = async () => {
      try {
        const result = await fetchEvolutionAnalysis(analysis.id);
        if (cancelled) return;
        setAnalysis(result.analysis);
        if (result.analysis.status === "queued" || result.analysis.status === "running") window.setTimeout(poll, 1400);
      } catch (error) {
        if (!cancelled) setAnalysisError(error instanceof Error ? error.message : String(error));
      }
    };
    const timer = window.setTimeout(poll, 900);
    return () => { cancelled = true; window.clearTimeout(timer); };
  }, [analysis?.id, analysis?.status]);

  if (gate) return gate;
  const data = state.data as EvolutionCatalogDetailResponse | null;
  if (!data) return null;
  const activeKey = playerKey || data.selected_player_key || "";
  const selected = data.players.find((entry) => entry.key === activeKey);
  const projection = data.projection;
  const playstyles = projection?.playstyles ?? [];
  const positionsAdded = projection?.positions_added ?? [];
  const setPlayer = (next: string) => {
    const params = new URLSearchParams(searchParams);
    if (next) params.set("player_key", next); else params.delete("player_key");
    setSearchParams(params);
    setAnalysis(undefined);
    setAnalysisError("");
  };
  const runAnalysis = async (force = false) => {
    if (!activeKey || !selected?.eligible || !data.agent_enabled) return;
    setRequesting(true);
    setAnalysisError("");
    try {
      const result = await requestEvolutionAnalysis(slug, activeKey, force);
      setAnalysis(result.analysis);
    } catch (error) {
      setAnalysisError(error instanceof Error ? error.message : String(error));
    } finally {
      setRequesting(false);
    }
  };
  return (
    <div className="wrap evo-detail-page">
      <Link className="evo-detail-crumb" to="/evolucoes">← voltar ao catálogo</Link>
      <PageHeader eyebrow={data.item.category_label || "evolução"} title={data.item.evolution.name} meta="Laboratório carta a carta · projeção estimada + evidência do fut.gg." />
      <section className="evo-detail-hero">
        <div className="evo-detail-title">
          <div className="evo-detail-badges"><Chip tone="turf">{data.item.category_label}</Chip><Chip tone={data.item.expired ? "alert" : "gain"}>{data.item.expired ? "expirada" : "ativa"}</Chip>{data.item.origin_label && <Chip tone="flat">{data.item.origin_label}</Chip>}</div>
          <h1>{data.item.evolution.name}</h1>
          <p>{data.item.evolution.description || "A evolução não trouxe descrição adicional; consulte os upgrades abaixo."}</p>
        </div>
        <div className="evo-detail-cost"><span className="evo-detail-cost-label">custo de entrada</span><strong>{costLabel(data)}</strong><small>{data.item.eligible_count} carta{data.item.eligible_count === 1 ? "" : "s"} elegível{data.item.eligible_count === 1 ? "" : "eis"}<br />{data.item.repeatable ? "evolução repetível" : "uma utilização por slot"}</small></div>
      </section>
      <div className="evo-detail-layout">
        <div className="evo-detail-main">
          <section className="evo-panel">
            <div className="evo-panel-head"><h2>Escolha a carta do seu elenco</h2><span>{data.players.filter((entry) => entry.eligible).length} válidas</span></div>
            <div className="evo-panel-body">
              <div className="evo-player-selector"><label htmlFor="evo-player">jogador válido</label><select id="evo-player" value={activeKey} onChange={(event) => setPlayer(event.target.value)}><option value="">selecione uma carta…</option>{data.players.map((entry) => <option key={entry.key} value={entry.key} disabled={!entry.eligible}>{entry.player.common_name || entry.player.name} · {entry.player.rating} OVR · {entry.player.position}{entry.eligible ? "" : " — indisponível"}</option>)}</select></div>
              {selected && <p className="evo-player-note">{selected.identity_complete ? "Identidade física confirmada pelo ClubItemID." : "Identidade física aproximada: a fonte não forneceu ClubItemID."}{selected.card_slug && <> · <Link to={`/time/${encodeURIComponent(selected.card_slug)}`}>abrir carta</Link></>}</p>}
              {(data.warnings ?? []).map((warning) => <p className="evo-warning" key={warning}>{warning}</p>)}
            </div>
          </section>
          {projection && <>
            <section className="evo-panel">
              <div className="evo-panel-head"><h2>O que muda na carta</h2><span>antes → depois</span></div>
              <div className="evo-panel-body">
                {mainGroups.map(([group, changes]) => <div className="evo-change-group" key={group}><h3>face da carta · overall {projection.before.rating} → {projection.after.rating} {projection.overall_delta > 0 ? `(+${projection.overall_delta})` : ""}</h3>{changes.map((change) => <ChangeRow key={change.key} change={change} />)}</div>)}
                {detailedGroups.map(([group, changes]) => <div className="evo-change-group" key={group}><h3>{group}</h3>{changes.map((change) => <ChangeRow key={change.key} change={change} />)}</div>)}
                {(projection.warnings ?? []).map((warning) => <p className="evo-warning" key={warning}>{warning}</p>)}
              </div>
            </section>
            <section className="evo-panel">
              <div className="evo-panel-head"><h2>PlayStyles e posições</h2><span>{playstyles.length} estilos no final</span></div>
              <div className="evo-panel-body"><div className="evo-style-list">{playstyles.map((style) => <span key={`${style.name}-${style.plus}`} className={`evo-style-chip ${style.status === "adicionado" ? "added" : style.status === "elevado" ? "elevated" : ""}`}><span>{style.name}{style.plus ? "+" : ""}</span><small>{style.status === "adicionado" ? "novo" : style.status === "elevado" ? "já tinha · virou +" : "já possui"}</small></span>)}</div>{positionsAdded.length > 0 && <div className="evo-position-list">{positionsAdded.map((position) => <span key={position}>posição +{position}</span>)}</div>}</div>
            </section>
          </>}
          {data.paths && data.paths.length > 0 && <section className="evo-panel"><div className="evo-panel-head"><h2>Evidência de caminho</h2><span>confirmado pelo fut.gg</span></div><div className="evo-panel-body"><div className="evo-path-list">{data.paths.map((path, index) => <div className="evo-path-row" key={`${path.card_slug}-${index}`}><div><strong>{path.confirmed ? "caminho confirmado" : "projeção"}</strong><div className="evo-player-note">{path.path.chain?.join(" → ") || "evolução única"} · final {path.final_overall} OVR</div></div><span>{path.final_gg_rating > 0 ? `GG ${path.final_gg_rating.toFixed(1)} (+${path.gg_rating_gain.toFixed(1)})` : "sem GG"}</span></div>)}</div></div></section>}
        </div>
        <aside className="evo-detail-sidebar">
          <section className="evo-panel"><div className="evo-panel-head"><h2>Requisitos</h2><span>fonte</span></div><div className="evo-panel-body"><div className="evo-side-copy">{data.item.evolution.requirements?.length ? <ul>{data.item.evolution.requirements.map((requirement, index) => <li key={`${requirement.kind}-${index}`}>{requirement.raw || `${requirement.kind}: ${requirement.int_value}`}</li>)}</ul> : "A fonte não publicou requisitos estruturados."}</div></div></section>
          <section className="evo-panel evo-agent-panel"><div className="evo-panel-head"><h2>Opinião do especialista</h2><span>agente</span></div><div className="evo-panel-body"><p className="evo-agent-status">{data.agent_enabled ? "Envie esta projeção ao seu agente configurado. A resposta fica salva localmente para reutilizar depois." : "Agente não configurado. Defina EAFC_EVO_AGENT_URL no ambiente do servidor para habilitar."}</p><button className="evo-agent-button" type="button" disabled={!data.agent_enabled || !selected?.eligible || requesting || !activeKey} onClick={() => void runAnalysis(Boolean(analysis?.status === "completed"))}>{requesting ? "enviando…" : analysis?.status === "completed" ? "pedir nova análise" : "analisar esta evolução"}</button>{analysisError && <p className="evo-warning">{analysisError}</p>}{analysis && <div className="evo-agent-result">{analysis.status === "queued" || analysis.status === "running" ? <p>O agente está analisando a carta…</p> : analysis.status === "failed" ? <p className="evo-warning">A análise falhou: {analysis.error || "erro não informado"}</p> : <><Chip tone={verdictTone(analysis.verdict)}>{verdictLabel(analysis.verdict)}</Chip><p>{analysis.summary}</p><div className="evo-agent-columns"><div><h4>pontos fortes</h4><ul>{(analysis.strengths ?? []).map((item) => <li key={item}>{item}</li>)}</ul></div><div><h4>riscos</h4><ul>{(analysis.risks ?? []).map((item) => <li key={item}>{item}</li>)}</ul></div></div>{analysis.best_positions && analysis.best_positions.length > 0 && <p><strong>melhores posições:</strong> {analysis.best_positions.join(" · ")}</p>}<ul className="evo-source-list">{(analysis.sources ?? []).map((source) => <li key={source.url}><a href={source.url} target="_blank" rel="noreferrer"><span>{source.title}</span><small>abrir ↗</small></a></li>)}</ul></>}</div>}</div></section>
          <section className="evo-panel"><div className="evo-panel-head"><h2>Fontes</h2><span>obrigatórias</span></div><div className="evo-panel-body"><ul className="evo-source-list">{data.sources.map((source) => <li key={source.url}><a href={source.url} target="_blank" rel="noreferrer"><span>{source.title}</span><small>abrir ↗</small></a></li>)}</ul></div></section>
        </aside>
      </div>
    </div>
  );
}
