import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { deleteGaleriaConclusao, fetchGaleriaDetalhe, saveGaleriaConclusao } from "../api";
import { asyncGate } from "../components/asyncGate";
import { formatDate } from "../format";
import {
  GRADES,
  formatPoints,
  galleryKind,
  kiloPoints,
  kindLabel,
  ladderState,
  predictedGrade,
  registeredGrade,
  rewardLabel,
  tagAbbrev,
  type LadderState,
} from "../galeria";
import { useData } from "../useData";
import type { GalleryGrade, GalleryPick, GalleryRecord, GalleryTagBreakdown } from "../types";
import { GalleryBadge } from "./Galeria";
import "../shared.css";
import "./Galeria.css";

const STATE_LABEL: Record<LadderState, string> = { reg: "registrada", pred: "prevista", fut: "futura" };
const REWARD_STATE: Record<LadderState, string> = { reg: "associada ao resultado registrado", pred: "alcançável pela previsão", fut: "faixa futura" };
const PICKS_PREVIEW = 12;

// firstOwnerLabel lê o 1º dono direto da carta sugerida: ausente é
// desconhecido — nunca "não", que tiraria o bônus sem prova.
function firstOwnerLabel(pick: GalleryPick): { text: string; tone: "sim" | "nao" | "desconhecido" } {
  if (pick.first_owner === true) return { text: "sim", tone: "sim" };
  if (pick.first_owner === false) return { text: "comprado", tone: "nao" };
  return { text: "desconhecido", tone: "desconhecido" };
}

function tagProgress(tag: GalleryTagBreakdown): number {
  if (!tag.next_min_items) return tag.matched_count > 0 ? 100 : 0;
  return Math.min(100, (tag.matched_count / tag.next_min_items) * 100);
}

// GaleriaDetalhe é "/galeria/:id": a letra prevista com a conta aberta
// (Item Score base + bônus das tags), a barra contra os limiares, as cartas
// que o motor escolheu, o bônus de cada tag, as recompensas cumulativas e o
// registro do resultado que o usuário fez no jogo.
export default function GaleriaDetalhe() {
  const { id = "" } = useParams();
  const { data, error, loading, refetch } = useData(() => fetchGaleriaDetalhe(id), [id]);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState("");
  const [grade, setGrade] = useState<GalleryGrade>("");
  const [score, setScore] = useState("");
  const [date, setDate] = useState("");
  const [allPicks, setAllPicks] = useState(false);

  useEffect(() => {
    if (!data?.completion) return;
    setGrade(data.completion.grade);
    setScore(data.completion.score ? String(data.completion.score) : "");
    setDate(data.completion.completed_at.slice(0, 10));
  }, [data]);

  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;
  if (!data) return null;

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

  const evaluation = data.evaluation;
  const kind = galleryKind(data);
  const predicted = predictedGrade(data);
  const completion = data.completion;
  const picks = (evaluation.picks ?? []).slice().sort((a, b) => b.item_score - a.item_score);
  const visiblePicks = allPicks ? picks : picks.slice(0, PICKS_PREVIEW);
  const maxPick = picks[0]?.item_score || 1;
  const tagsByCard = new Map<number, string[]>();
  for (const tag of evaluation.tags ?? []) {
    for (const cardID of tag.matched_ids ?? []) tagsByCard.set(cardID, [...(tagsByCard.get(cardID) ?? []), tag.name]);
  }
  const firstOwnerPending = evaluation.tags?.find((tag) => tag.attribute === "FIRST_OWNED" && (tag.unknown_count || 0) > 0);
  const pool = data.set.pool_size ? ` · pool ${data.set.pool_size.toLocaleString("pt-BR")} itens${data.set.pool_truncated ? " (truncado)" : ""}` : "";

  return <div className="wrap galeria-page">
    <div className="page-header gallery-detail-header">
      <div className="gallery-detail-title">
        <GalleryBadge name={data.set.name} url={data.set.badge_url} size="lg" />
        <div className="titles">
          <div className="eyebrow">FUT Gallery / {data.set.category || "conjunto"}</div>
          <h1>{data.set.name}</h1>
          <div className="meta">{evaluation.filled}/{evaluation.required} cartas confirmadas{pool} · coleção de {formatDate(evaluation.computed_at)}</div>
        </div>
      </div>
      <div className="actions">
        <Link className="btn ghost" to="/galeria/colecao">corrigir dados</Link>
        <Link className="btn ghost" to="/galeria">← voltar à Gallery</Link>
      </div>
    </div>

    <section className="panel gallery-hero">
      <div className="gallery-hero-cells">
        <div className="gallery-hero-grade">
          <div className="gallery-hero-letter"><span>prevista</span><strong>{predicted || "—"}</strong></div>
          <div className="gallery-hero-copy">
            <span className={`gallery-status ${kind}`}>{kindLabel(data, kind)}</span>
            <span>{completion ? <>Registrada <strong>{completion.grade}</strong> em {formatDate(completion.completed_at)}{completion.score ? ` com ${formatPoints(completion.score)} pontos` : ""}.</> : "Nenhum resultado registrado no jogo ainda."}</span>
            <small>cobertura {evaluation.coverage || "desconhecida"} · {evaluation.max_proven ? "máximo comprovado" : "melhor combinação encontrada"}{evaluation.states ? ` · ${formatPoints(evaluation.states)} estados avaliados` : ""}</small>
          </div>
        </div>
        <div className="gallery-hero-points">
          <span className="gallery-hero-kicker">pontos previstos</span>
          <strong>{predicted ? formatPoints(evaluation.score) : "—"}</strong>
          {predicted ? <dl>
            <dt>base (Item Score)</dt><dd>{formatPoints(evaluation.base_score)}</dd>
            <dt>bônus das tags</dt><dd className="up">+{formatPoints(evaluation.bonus_score)}</dd>
            {completion?.score ? <><dt>vs registrado</dt><dd className={evaluation.score >= completion.score ? "up" : "down"}>{evaluation.score >= completion.score ? "+" : "−"}{formatPoints(Math.abs(evaluation.score - completion.score))}</dd></> : null}
          </dl> : <p>Sem combinação avaliável ainda: {kindLabel(data, kind)}.</p>}
        </div>
        <div className="gallery-hero-thresholds">
          <span className="gallery-hero-kicker">limiares</span>
          {[...GRADES].reverse().map((item) => {
            const state = ladderState(data, item);
            return <div key={item}><span className={`gallery-grade-cell ${state}`}>{item}</span><span>{data.set.thresholds?.[item] ? formatPoints(data.set.thresholds[item]!) : "—"}</span><small className={state}>{STATE_LABEL[state]}</small></div>;
          })}
        </div>
      </div>
      <PointsBar data={data} />
    </section>

    {(evaluation.warnings?.length || firstOwnerPending) ? <div className="banner alert gallery-notes">
      {firstOwnerPending ? <p>A previsão ainda não reproduz o jogo: falta confirmar o primeiro dono de <strong>{firstOwnerPending.unknown_cards?.map((card) => card.name || `Carta ${card.card_id}`).join(", ")}</strong>. <Link to="/galeria/colecao">Corrigir dados</Link></p> : null}
      {evaluation.warnings?.length ? <ul>{evaluation.warnings.map((warning) => <li key={warning}>{warning}</li>)}</ul> : null}
    </div> : null}

    <div className="gallery-detail-grid">
      <section className="panel">
        <div className="panel-head">
          <span>Cartas sugeridas <span className="panel-head-sub">/ combinação em avaliação</span></span>
          <span className="panel-head-meta">{picks.length} cartas · ordem por Item Score</span>
        </div>
        {picks.length === 0 ? <p className="gallery-panel-empty">O motor ainda não escolheu cartas para este conjunto.</p> : <div className="tablewrap flush">
          <table className="gallery-picks">
            <thead><tr><th>#</th><th>Carta</th><th>Item Score</th><th>1º dono</th><th>Tags</th></tr></thead>
            <tbody>{visiblePicks.map((pick, index) => {
              const owner = firstOwnerLabel(pick);
              const tags = tagsByCard.get(pick.card_id) ?? [];
              const meta = [pick.position, pick.club, pick.rating ? `${pick.rating} OVR` : ""].filter(Boolean).join(" · ");
              return <tr key={pick.card_id}>
                <td className="gallery-pick-n">{index + 1}</td>
                <td><span className="gallery-pick-card"><span className="gallery-pick-art" aria-hidden="true" /><span><strong>{pick.name || `Carta ${pick.card_id}`}</strong>{meta ? <small>{meta}</small> : null}</span></span></td>
                <td className="gallery-pick-score"><span className="gallery-pick-bar"><span className={tags.length ? "tagged" : ""} style={{ width: `${(pick.item_score / maxPick) * 100}%` }} /></span><b>{formatPoints(pick.item_score)}</b></td>
                <td><span className={`gallery-owner ${owner.tone}`}>{owner.text}</span></td>
                <td><span className="gallery-pick-tags">{tags.map((tag) => <span key={tag} title={tag}>{tagAbbrev(tag)}</span>)}</span></td>
              </tr>;
            })}</tbody>
          </table>
        </div>}
        {picks.length > PICKS_PREVIEW ? <div className="gallery-panel-foot">
          <span>{allPicks ? `${picks.length} de ${picks.length}` : `${PICKS_PREVIEW} de ${picks.length}`} · {formatPoints(evaluation.base_score)} de Item Score no total</span>
          <button type="button" className="btn ghost" onClick={() => setAllPicks(!allPicks)}>{allPicks ? "mostrar menos" : `ver as ${picks.length}`}</button>
        </div> : null}
      </section>

      <div className="gallery-detail-side">
        <section className="panel">
          <div className="panel-head">
            <span>Bônus por tag <span className="panel-head-sub">/ só nas cartas da tag</span></span>
            <span className="panel-head-meta gallery-bonus-total">+{formatPoints(evaluation.bonus_score)}</span>
          </div>
          {(evaluation.tags ?? []).length === 0 ? <p className="gallery-panel-empty">Este conjunto não publica bônus por tag.</p> : (evaluation.tags ?? []).map((tag) => <div className="gallery-tag" key={`${tag.name}-${tag.attribute}`}>
            <div className="gallery-tag-head">
              <span><strong>{tag.name}</strong> <small>{tag.operator}</small></span>
              <b className={tag.pending ? "pending" : tag.bonus_points ? "up" : ""}>{tag.pending ? "pendente" : tag.bonus_points ? `+${formatPoints(tag.bonus_points)}` : "0"}</b>
            </div>
            {!tag.pending && <div className="gallery-bar tag"><span className={tag.bonus_points ? "on" : ""} style={{ width: `${tagProgress(tag)}%` }} /></div>}
            <div className="gallery-tag-line">
              <span>{tag.pending ? tag.reason || "regra ainda não interpretada — fora da previsão" : `${tag.matched_count} cartas${tag.matched_score ? ` · subtotal ${formatPoints(tag.matched_score)}` : ""} · +${tag.bonus_percent}%${tag.unknown_count ? ` · ${tag.unknown_count} sem confirmação` : ""}`}</span>
              {!tag.pending && <span className={tag.next_min_items ? "next" : "max"}>{tag.next_min_items ? `próxima faixa: ${tag.next_min_items} → +${tag.next_bonus_percent ?? "?"}%` : "faixa máxima"}</span>}
            </div>
          </div>)}
          <p className="gallery-tag-note">bônus = floor(% × subtotal das cartas da tag ÷ 100); tags somam entre si.</p>
        </section>

        <section className="panel">
          <div className="panel-head"><span>Registrar conclusão <span className="panel-head-sub">/ resultado do jogo</span></span></div>
          <div className="gallery-completion">
            <label>Nota<select name="gallery-grade" autoComplete="off" value={grade} onChange={(event) => setGrade(event.target.value as GalleryGrade)}><option value="">selecione</option>{GRADES.map((item) => <option key={item}>{item}</option>)}</select></label>
            <label>Data<input name="gallery-date" autoComplete="off" type="date" value={date} onChange={(event) => setDate(event.target.value)} /></label>
            <label>Pontos<input name="gallery-score" autoComplete="off" type="number" min="0" inputMode="numeric" placeholder="opcional" value={score} onChange={(event) => setScore(event.target.value)} /></label>
          </div>
          {formError && <p className="field-error gallery-form-error" role="alert">{formError}</p>}
          <div className="gallery-completion-actions">
            <button type="button" className="btn primary" disabled={!grade || !date || saving} onClick={save}>{saving ? "salvando…" : "salvar resultado"}</button>
            {completion && <button type="button" className="btn ghost" disabled={saving} onClick={remove}>remover registro</button>}
          </div>
        </section>
      </div>
    </div>

    <section className="panel gallery-rewards">
      <div className="panel-head"><span>Recompensas por nota <span className="panel-head-sub">/ cumulativas · publicadas pelo FUT.GG</span></span></div>
      <div className="gallery-reward-cells">{GRADES.map((item) => {
        const state = ladderState(data, item);
        const rewards = data.set.rewards?.[item] ?? [];
        return <div className={`gallery-reward-tier ${state}`} key={item}>
          <div className="gallery-reward-tier-head"><span className={`gallery-grade-cell ${state}`}>{item}</span><small>{data.set.thresholds?.[item] ? `${formatPoints(data.set.thresholds[item]!)} pts` : "—"}</small></div>
          <div className="gallery-reward-items">{rewards.length ? rewards.map((reward, index) => <span key={`${reward.id || reward.label || reward.type}-${index}`}><span className={`gallery-reward-art${reward.image_url ? " has-image" : ""}`} aria-hidden="true">{reward.image_url ? <img src={reward.image_url} alt="" loading="lazy" /> : null}</span>{rewardLabel(reward)}</span>) : <span className="gallery-reward-none">sem recompensa publicada</span>}</div>
          <small className={`gallery-reward-state ${state}`}>{REWARD_STATE[state]}</small>
        </div>;
      })}</div>
    </section>
  </div>;
}

// PointsBar é a composição da pontuação contra os limiares: a base (Item
// Score) clara, o bônus das tags cheio, cada letra como marca e o resultado
// registrado como uma linha — o mesmo número que o hero descreve em texto.
function PointsBar({ data }: { data: GalleryRecord }) {
  const evaluation = data.evaluation;
  const predicted = predictedGrade(data);
  const registeredScore = data.completion?.score;
  const top = Math.max(evaluation.score, data.set.thresholds?.S ?? 0, registeredScore ?? 0, 1) * 1.04;
  const pct = (value: number) => `${(value / top) * 100}%`;
  return <div className="gallery-points-bar">
    <div className="gallery-points-labels">{GRADES.map((item) => data.set.thresholds?.[item] ? <span key={item} className={ladderState(data, item)} style={{ left: pct(data.set.thresholds[item]!) }}>{item} {kiloPoints(data.set.thresholds[item])}</span> : null)}</div>
    <div className="gallery-points-track">
      {predicted ? <><span className="base" style={{ width: pct(evaluation.base_score) }} /><span className="bonus" style={{ left: pct(evaluation.base_score), width: pct(evaluation.bonus_score) }} /></> : null}
      {GRADES.map((item) => data.set.thresholds?.[item] ? <i key={item} style={{ left: pct(data.set.thresholds[item]!) }} /> : null)}
      {registeredScore ? <b style={{ left: pct(registeredScore) }} title={`registrado ${formatPoints(registeredScore)}`} /> : null}
    </div>
    <div className="gallery-points-legend">
      <span><i className="base" /> base {predicted ? formatPoints(evaluation.base_score) : "—"}</span>
      <span><i className="bonus" /> bônus {predicted ? formatPoints(evaluation.bonus_score) : "—"}</span>
      {registeredScore ? <span><i className="reg" /> registrado {formatPoints(registeredScore)}</span> : null}
      {!predicted && registeredGrade(data) === "" ? <span>sem pontuação prevista nem registrada</span> : null}
    </div>
  </div>;
}
