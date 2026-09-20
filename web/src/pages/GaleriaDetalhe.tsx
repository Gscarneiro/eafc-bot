import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { deleteGaleriaConclusao, fetchGaleriaDetalhe, saveGaleriaConclusao } from "../api";
import { useData } from "../useData";
import PageHeader from "../components/PageHeader";
import type { GalleryGrade } from "../types";

const grades: GalleryGrade[] = ["D", "C", "B", "A", "S"];

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
      <section className="panel"><div className="panel-head"><span>Previsão <span className="panel-head-sub">/ modelo FUT.GG</span></span><span className="gallery-grade">{data.evaluation.grade || "—"}</span></div><div className="panel-body"><div className="detail-score">{(data.evaluation.score ?? 0).toLocaleString("pt-BR")} <small>pontos totais</small></div><p className="gallery-equation">{(data.evaluation.base_score ?? data.evaluation.score ?? 0).toLocaleString("pt-BR")} base <strong>+</strong> {(data.evaluation.bonus_score ?? 0).toLocaleString("pt-BR")} bônus <strong>=</strong> {(data.evaluation.score ?? 0).toLocaleString("pt-BR")}</p><p className="hint">Cobertura: {data.evaluation.coverage || "desconhecida"}. Estado: {data.evaluation.status}. Estados avaliados: {data.evaluation.states ?? "—"}.</p>{data.completion && <p>Resultado registrado no jogo: <strong>{data.completion.grade}</strong> em {new Date(data.completion.completed_at).toLocaleDateString("pt-BR")}.</p>}{data.evaluation.next_grade && <p>Próxima nota: <strong>{data.evaluation.next_grade}</strong> em {data.evaluation.next_threshold?.toLocaleString("pt-BR")} pontos.</p>}{data.evaluation.warnings?.map((warning) => <p className="gallery-warning" role="status" key={warning}>{warning}</p>)}</div></section>
      <section className="panel"><div className="panel-head"><span>Cartas sugeridas</span></div><div className="tablewrap"><table><thead><tr><th>Carta</th><th className="num">Item Score</th></tr></thead><tbody>{data.evaluation.picks?.map((pick) => <tr key={pick.card_id}><td>{pick.name || `Carta ${pick.card_id}`}</td><td className="num">{pick.item_score.toLocaleString("pt-BR")}</td></tr>)}</tbody></table></div></section>
    </div>
    <section className="panel gallery-tags"><div className="panel-head"><span>Bônus por tag</span><span className="panel-head-sub">cada tag incide apenas nas cartas correspondentes</span></div><div className="panel-body"><div className="gallery-tag-list">{(data.evaluation.tags || []).map((tag) => <details key={`${tag.name}-${tag.attribute}`}><summary><strong>{tag.name}</strong><span>{tag.pending ? "pendente" : `+${tag.bonus_points.toLocaleString("pt-BR")} pontos`}</span></summary><p>{tag.pending ? tag.reason : `${tag.matched_count} cartas · ${tag.matched_score.toLocaleString("pt-BR")} subtotal · +${tag.bonus_percent}%`}{tag.next_min_items ? ` · próxima faixa em ${tag.next_min_items} cartas` : ""}</p></details>)}</div></div></section>
    <section className="panel completion-panel"><div className="panel-head"><span>Registrar conclusão <span className="panel-head-sub">/ resultado informado no jogo</span></span></div><div className="panel-body completion-form"><label>Nota<select value={grade} onChange={(event) => setGrade(event.target.value as GalleryGrade)}><option value="">selecione</option>{grades.map((item) => <option key={item}>{item}</option>)}</select></label><label>Data<input type="date" value={date} onChange={(event) => setDate(event.target.value)} /></label><label>Pontos (opcional)<input type="number" min="0" value={score} onChange={(event) => setScore(event.target.value)} /></label>{formError && <p className="field-error" role="alert">{formError}</p>}<button className="btn primary" disabled={!grade || !date || saving} onClick={save}>{saving ? "salvando..." : "salvar resultado"}</button>{data.completion && <button className="btn ghost" disabled={saving} onClick={remove}>remover registro</button>}</div></section>
  </div>;
}
