import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { deleteGaleriaConclusao, fetchGaleriaDetalhe, saveGaleriaConclusao } from "../api";
import { useData } from "../useData";
import PageHeader from "../components/PageHeader";
import type { GalleryGrade, GalleryRecord, GalleryReward } from "../types";

const grades: GalleryGrade[] = ["D", "C", "B", "A", "S"];

function rewardLabel(reward: GalleryReward) {
  const name = reward.label || reward.type || "recompensa";
  const quantity = reward.count && reward.count > 1 ? `${reward.count}× ` : "";
  return `${quantity}${name}${reward.value ? ` · ${reward.value.toLocaleString("pt-BR")}` : ""}`;
}

function scoreProgress(data: GalleryRecord) {
  if (data.evaluation.status !== "available") return undefined;
  const current = data.evaluation.grade ? data.set.thresholds?.[data.evaluation.grade] || 0 : 0;
  const next = data.evaluation.next_threshold;
  if (!next) return 100;
  return Math.max(0, Math.min(100, Math.round(((data.evaluation.score - current) / (next - current)) * 100)));
}

export default function GaleriaDetalhe() {
  const { id = "" } = useParams();
  const { data, error, loading, refetch } = useData(() => fetchGaleriaDetalhe(id), [id]);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");
  const [grade, setGrade] = useState<GalleryGrade>("");
  const [score, setScore] = useState("");
  const [date, setDate] = useState("");

  useEffect(() => {
    if (!data?.completion) return;
    setGrade(data.completion.grade);
    setScore(data.completion.score ? String(data.completion.score) : "");
    setDate(data.completion.completed_at.slice(0, 10));
  }, [data]);
  if (loading) return <div className="wrap"><p className="hint">Carregando conjunto...</p></div>;
  if (error || !data) return <div className="wrap"><div className="banner alert" role="alert">{error?.message || "Conjunto não encontrado"}</div></div>;

  const cardsProgress = data.evaluation.required > 0 ? Math.min(100, Math.round((data.evaluation.filled / data.evaluation.required) * 100)) : 0;
  const pointsProgress = scoreProgress(data);
  const firstOwnerPending = data.evaluation.tags?.find((tag) => tag.attribute === "FIRST_OWNED" && (tag.unknown_count || 0) > 0);

  const save = async () => {
    setFormError("");
    if (!grade || !date) { setFormError("Informe a letra e a data do resultado."); return; }
    setSaving(true);
    try { await saveGaleriaConclusao(id, { grade, score: Number(score) || 0, completed_at: `${date}T12:00:00Z` }); refetch(); }
    catch (err) { setFormError(err instanceof Error ? err.message : "Não foi possível salvar o resultado."); }
    finally { setSaving(false); }
  };
  const remove = async () => {
    setFormError(""); setSaving(true);
    try { await deleteGaleriaConclusao(id); setGrade(""); setScore(""); setDate(""); refetch(); }
    catch (err) { setFormError(err instanceof Error ? err.message : "Não foi possível remover o registro."); }
    finally { setSaving(false); }
  };

  return <div className="wrap galeria-page">
    <PageHeader eyebrow={`fut gallery · ${data.set.category || "conjunto"}`} title={data.set.name} meta={`${data.evaluation.filled}/${data.evaluation.required} cartas`} actions={<Link className="btn ghost" to="/galeria">voltar</Link>} />
    <div className="detail-grid">
      <section className="panel gallery-summary">
        <div className="panel-head"><span>Previsão <span className="panel-head-sub">/ modelo FUT.GG</span></span><span className="gallery-grade">{data.evaluation.grade || "—"}</span></div>
        <div className="panel-body">
          <div className="detail-score">{data.evaluation.status === "available" ? <>{data.evaluation.score.toLocaleString("pt-BR")} <small>pontos totais</small></> : <><strong>previsão pendente</strong><small>faltam cartas ou dados verificáveis</small></>}</div>
          {data.evaluation.status === "available" && (
            <>
              <p className="gallery-equation">{data.evaluation.base_score.toLocaleString("pt-BR")} base <strong>+</strong> {data.evaluation.bonus_score.toLocaleString("pt-BR")} bônus <strong>=</strong> {data.evaluation.score.toLocaleString("pt-BR")}</p>
              <div className="detail-progress">
                <div><span>cartas confirmadas</span><strong>{data.evaluation.filled}/{data.evaluation.required}</strong></div>
                <div className="gallery-progress" role="progressbar" aria-label="Cartas confirmadas" aria-valuemin={0} aria-valuemax={100} aria-valuenow={cardsProgress}><span style={{ width: `${cardsProgress}%` }} /></div>
                {pointsProgress !== undefined && <>
                  <div><span>pontos para {data.evaluation.next_grade || "S"}</span><strong>{data.evaluation.next_grade ? `faltam ${Math.max(0, (data.evaluation.next_threshold || 0) - data.evaluation.score).toLocaleString("pt-BR")}` : "nota S prevista"}</strong></div>
                  <div className="gallery-progress score" role="progressbar" aria-label="Progresso de pontos" aria-valuemin={0} aria-valuemax={100} aria-valuenow={pointsProgress}><span style={{ width: `${pointsProgress}%` }} /></div>
                </>}
              </div>
            </>
          )}
          <p className="hint">Cobertura do pool: {data.evaluation.coverage || "desconhecida"}.{data.evaluation.max_proven ? " máximo comprovado." : " melhor combinação encontrada."} Estados avaliados: {data.evaluation.states ?? "—"}.</p>
          {data.completion && <p>Resultado registrado no jogo: <strong>{data.completion.grade}</strong> em {new Date(data.completion.completed_at).toLocaleDateString("pt-BR")}.</p>}
          {data.evaluation.next_grade && <p>Próxima nota: <strong>{data.evaluation.next_grade}</strong> em {data.evaluation.next_threshold?.toLocaleString("pt-BR")} pontos.</p>}
          {firstOwnerPending && <p className="gallery-data-callout">A previsão ainda não pode reproduzir o jogo: faltam confirmar primeiro dono de <strong>{firstOwnerPending.unknown_cards?.map((card) => card.name || `Carta ${card.card_id}`).join(", ")}</strong>. <Link to="/galeria/colecao">Corrigir dados</Link></p>}
          {data.evaluation.warnings?.map((warning) => <p className="gallery-warning" role="status" key={warning}>{warning}</p>)}
        </div>
      </section>
      <section className="panel gallery-picks"><div className="panel-head"><span>Cartas sugeridas</span><span className="panel-head-sub">combinação em avaliação</span></div><ol className="gallery-pick-grid">{data.evaluation.picks?.map((pick) => <li key={pick.card_id}><span>{pick.name || `Carta ${pick.card_id}`}</span><strong>{pick.item_score.toLocaleString("pt-BR")}</strong></li>)}</ol></section>
    </div>
    <section className="panel gallery-tags"><div className="panel-head"><span>Bônus por tag</span><span className="panel-head-sub">cada tag incide apenas nas cartas correspondentes</span></div><div className="panel-body"><div className="gallery-tag-list">{(data.evaluation.tags || []).map((tag) => <details key={`${tag.name}-${tag.attribute}`}><summary><strong>{tag.name}</strong><span>+{tag.bonus_points.toLocaleString("pt-BR")} pontos{tag.pending ? " · dados pendentes" : ""}</span></summary><p>{`${tag.matched_count} cartas confirmadas · ${tag.matched_score.toLocaleString("pt-BR")} subtotal · +${tag.bonus_percent}%`}{tag.unknown_count ? ` · ${tag.unknown_count} desconhecida(s)` : ""}{tag.next_min_items ? ` · próxima faixa em ${tag.next_min_items} cartas` : ""}</p>{tag.reason && <p className="gallery-warning">{tag.reason}</p>}{!!tag.matched_cards?.length && <ul className="gallery-tag-cards">{tag.matched_cards.map((card) => <li key={card.card_id}>{card.name || `Carta ${card.card_id}`} <span>{card.item_score.toLocaleString("pt-BR")}</span></li>)}</ul>}{!!tag.unknown_cards?.length && <p className="hint">Sem confirmação: {tag.unknown_cards.map((card) => card.name || `Carta ${card.card_id}`).join(", ")}.</p>}</details>)}</div></div></section>
    <div className="gallery-footer-grid"><section className="panel gallery-rewards"><div className="panel-head"><span>Recompensas por nota</span><span className="panel-head-sub">itens publicados pelo FUT.GG</span></div><div className="panel-body"><div className="gallery-reward-timeline">{grades.map((item) => { const rewards = data.set.rewards?.[item] || []; const recorded = !!data.completion && grades.indexOf(item) <= grades.indexOf(data.completion.grade); const predicted = !!data.evaluation.grade && grades.indexOf(item) <= grades.indexOf(data.evaluation.grade); return <div className={`gallery-reward-tier ${recorded ? "recorded" : predicted ? "predicted" : ""}`} key={item}><strong>{item}</strong><div>{rewards.length ? rewards.map((reward, index) => <span className="gallery-reward-item" key={`${reward.id || reward.label || reward.type}-${index}`}>{reward.image_url && <img src={reward.image_url} width="22" height="22" loading="lazy" alt="" />}{rewardLabel(reward)}</span>) : <span className="hint">sem recompensa publicada</span>}</div><small>{recorded ? "associada ao resultado registrado" : predicted ? "alcançável pela previsão" : "faixa futura"}</small></div>; })}</div></div></section>
    <section className="panel completion-panel"><div className="panel-head"><span>Registrar conclusão <span className="panel-head-sub">/ resultado informado no jogo</span></span></div><div className="panel-body completion-form"><label>Nota<select name="gallery-grade" autoComplete="off" value={grade} onChange={(event) => setGrade(event.target.value as GalleryGrade)}><option value="">selecione</option>{grades.map((item) => <option key={item}>{item}</option>)}</select></label><label>Data<input name="gallery-date" autoComplete="off" type="date" value={date} onChange={(event) => setDate(event.target.value)} /></label><label>Pontos (opcional)<input name="gallery-score" autoComplete="off" type="number" min="0" inputMode="numeric" value={score} onChange={(event) => setScore(event.target.value)} /></label>{formError && <p className="field-error" role="alert">{formError}</p>}<button className="btn primary" disabled={!grade || !date || saving} onClick={save}>{saving ? "salvando…" : "salvar resultado"}</button>{data.completion && <button className="btn ghost" disabled={saving} onClick={remove}>remover registro</button>}</div></section></div>
  </div>;
}
