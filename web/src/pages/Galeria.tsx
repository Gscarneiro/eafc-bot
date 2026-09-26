import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { fetchGaleria } from "../api";
import { asyncGate } from "../components/asyncGate";
import PageHeader from "../components/PageHeader";
import { formatDate } from "../format";
import {
  GRADES,
  formatPoints,
  galleryKind,
  gradeIndex,
  kiloPoints,
  kindLabel,
  ladderState,
  nextReward,
  predictedGrade,
  registeredGrade,
  rewardLabel,
  scoreLine,
  topThreshold,
  type GalleryKind,
} from "../galeria";
import { useData } from "../useData";
import type { GalleryRecord } from "../types";
import "../shared.css";
import "./Galeria.css";

const VIEW_KEY = "eafc-bot:galeria-view";
type Filter = "todos" | GalleryKind;
const FILTERS: { key: Filter; label: string }[] = [
  { key: "todos", label: "todos" },
  { key: "up", label: "avanço" },
  { key: "pend", label: "pendentes" },
  { key: "miss", label: "faltam cartas" },
  { key: "done", label: "finalizadas" },
  { key: "flat", label: "sem avanço" },
];

function categoryLabel(category: string | undefined) {
  return category?.trim() || "Outros conjuntos";
}

function readView(): "cartoes" | "lista" {
  try { return localStorage.getItem(VIEW_KEY) === "lista" ? "lista" : "cartoes"; } catch { return "cartoes"; }
}

export function GalleryBadge({ name, url, size }: { name: string; url?: string; size?: "sm" | "lg" }) {
  const [failed, setFailed] = useState(false);
  const fallback = !url || failed;
  return <span className={`gallery-badge${size ? ` ${size}` : ""}`} aria-hidden="true">
    {fallback
      ? <span className="gallery-badge-fallback">{name.slice(0, 1).toUpperCase()}</span>
      : <img src={url} alt="" loading="lazy" onError={() => setFailed(true)} />}
  </span>;
}

// Galeria é "/galeria": todos os conjuntos da FUT Gallery com a escada
// D→S (registrada no jogo × prevista pelo modelo FUT.GG), a barra de pontos
// com os limiares e a próxima recompensa. A API já entrega o catálogo
// inteiro (até 500 conjuntos), então busca e filtro são locais — é o que
// deixa o resumo e as contagens dos filtros valendo para a coleção toda.
export default function Galeria() {
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState<Filter>("todos");
  const [view, setViewState] = useState<"cartoes" | "lista">(readView);
  const setView = (next: "cartoes" | "lista") => {
    setViewState(next);
    try { localStorage.setItem(VIEW_KEY, next); } catch { /* sem storage a tela funciona, só esquece a preferência */ }
  };
  const { data, loading, error, refetch } = useData(() => fetchGaleria(), []);
  const all = data?.value ?? [];
  const kinds = useMemo(() => new Map(all.map((row) => [row.set.id, galleryKind(row)])), [all]);
  const rows = useMemo(() => {
    const query = search.trim().toLocaleLowerCase("pt-BR");
    return all.filter((row) => (filter === "todos" || kinds.get(row.set.id) === filter) && (!query || row.set.name.toLocaleLowerCase("pt-BR").includes(query)));
  }, [all, kinds, filter, search]);
  const groups = useMemo(() => {
    const byCategory = new Map<string, { name: string; order: number; rows: GalleryRecord[] }>();
    rows.forEach((row) => {
      const name = categoryLabel(row.set.category);
      const current = byCategory.get(name);
      if (current) {
        current.rows.push(row);
        current.order = Math.min(current.order, row.set.category_id || Number.MAX_SAFE_INTEGER);
      } else {
        byCategory.set(name, { name, order: row.set.category_id || Number.MAX_SAFE_INTEGER, rows: [row] });
      }
    });
    return [...byCategory.values()].sort((a, b) => a.order - b.order || a.name.localeCompare(b.name, "pt-BR"));
  }, [rows]);

  const gate = asyncGate(loading, error, !!data, refetch);
  if (gate) return gate;

  const count = (kind: GalleryKind) => all.filter((row) => kinds.get(row.set.id) === kind).length;
  const gradesToGain = all.reduce((total, row) => total + (predictedGrade(row) ? Math.max(0, gradeIndex(predictedGrade(row)) - gradeIndex(registeredGrade(row))) : 0), 0);
  const computedAt = all.reduce((latest, row) => (row.evaluation.computed_at > latest ? row.evaluation.computed_at : latest), "");
  const totalInCategory = (name: string) => all.filter((row) => categoryLabel(row.set.category) === name);

  return <div className="wrap galeria-page">
    <PageHeader
      eyebrow="Elenco / FUT Gallery"
      title="FUT Gallery"
      meta={`${all.length} conjuntos · previsão pelo modelo FUT.GG${computedAt ? ` · coleção de ${formatDate(computedAt)}` : ""}`}
      actions={<Link className="btn ghost" to="/galeria/colecao">minha coleção</Link>}
    />

    <div className="gallery-stats">
      <div><span>Finalizadas em S</span><strong className="turf">{count("done")}</strong><small> / {all.length}</small></div>
      <div><span>Avanço disponível</span><strong>{count("up")}</strong><small> conjuntos</small></div>
      <div><span>Dados pendentes</span><strong className="alert">{count("pend")}</strong><small> conjuntos</small></div>
      <div><span>Faltam cartas</span><strong className="cost">{count("miss")}</strong><small> conjuntos</small></div>
      <div><span>Letras a ganhar</span><strong>{gradesToGain}</strong><small> pela previsão</small></div>
    </div>

    <div className="gallery-toolbar">
      <input aria-label="Buscar conjuntos" placeholder="Buscar conjunto…" value={search} onChange={(event) => setSearch(event.target.value)} />
      <div className="gallery-segment" role="group" aria-label="Filtrar conjuntos">
        {FILTERS.map((item) => <button key={item.key} type="button" aria-pressed={filter === item.key} className={filter === item.key ? "active" : ""} onClick={() => setFilter(item.key)}>{item.label} <span>{item.key === "todos" ? all.length : count(item.key)}</span></button>)}
      </div>
      <div className="toggle gallery-view-toggle">
        <button type="button" className={view === "cartoes" ? "active" : ""} onClick={() => setView("cartoes")}>cartões</button>
        <button type="button" className={view === "lista" ? "active" : ""} onClick={() => setView("lista")}>lista</button>
      </div>
    </div>

    <div className="gallery-legend">
      <span><i className="reg" /> registrada no jogo</span>
      <span><i className="pred" /> alcançável pela previsão</span>
      <span><i className="fut" /> faixa futura</span>
      <span><i className="pts" /> pontos previstos</span>
    </div>

    {all.length === 0 ? <div className="gallery-empty">Nenhum conjunto avaliado ainda. Rode uma coleta (<code>eafcbot run</code>) para carregar o catálogo da Gallery.</div>
      : rows.length === 0 ? <div className="gallery-empty">Nenhum conjunto neste filtro.</div>
      : view === "cartoes" ? <div className="gallery-categories">
        {groups.map((group) => {
          const inCategory = totalInCategory(group.name);
          return <section className="gallery-category" key={group.name}>
            <div className="gallery-category-head">
              <div>
                <p className="gallery-category-kicker">categoria</p>
                <h2>{group.name}</h2>
              </div>
              <span>{group.rows.length} {group.rows.length === 1 ? "conjunto" : "conjuntos"} · {inCategory.filter((row) => registeredGrade(row) === "S").length}/{inCategory.length} em S</span>
            </div>
            <div className="gallery-grid">{group.rows.map((row) => <GalleryCard row={row} kind={kinds.get(row.set.id)!} key={row.set.id} />)}</div>
          </section>;
        })}
      </div>
      : <div className="tablewrap gallery-table">
        <table>
          <thead><tr><th>Conjunto</th><th>Letras</th><th className="num">Pontos</th><th>Para a próxima</th><th className="num">Cartas</th><th>Próxima recompensa</th><th>Status</th></tr></thead>
          <tbody>{rows.map((row) => {
            const kind = kinds.get(row.set.id)!;
            const reward = nextReward(row);
            const predicted = predictedGrade(row);
            return <tr key={row.set.id}>
              <td><span className="gallery-table-set"><GalleryBadge name={row.set.name} url={row.set.badge_url} size="sm" /><span><Link to={`/galeria/${encodeURIComponent(row.set.id)}`}>{row.set.name}</Link><small>{categoryLabel(row.set.category)}</small></span></span></td>
              <td><span className="gallery-ladder mini">{GRADES.map((grade) => <span key={grade} className={ladderState(row, grade)}>{grade}</span>)}</span></td>
              <td className="num gallery-points">{predicted ? formatPoints(row.evaluation.score) : "—"}</td>
              <td className="gallery-next-cell"><span className="gallery-bar thin"><span style={{ width: `${predicted ? Math.min(100, (row.evaluation.score / topThreshold(row)) * 100) : 0}%` }} /></span><small>{scoreLine(row)}</small></td>
              <td className="num">{row.evaluation.filled}/{row.evaluation.required}</td>
              <td className="gallery-reward-cell">{reward ? <><small>próxima recompensa · {reward.grade}</small>{rewardLabel(reward.reward)}</> : <><small>recompensas</small>todas as faixas associadas ao resultado</>}</td>
              <td><span className={`gallery-status ${kind}`}>{kindLabel(row, kind)}</span></td>
            </tr>;
          })}</tbody>
        </table>
      </div>}
  </div>;
}

function GalleryCard({ row, kind }: { row: GalleryRecord; kind: GalleryKind }) {
  const predicted = predictedGrade(row);
  const registered = registeredGrade(row);
  const top = topThreshold(row);
  const reward = nextReward(row);
  const tags = (row.evaluation.tags ?? []).filter((tag) => !tag.pending && tag.bonus_percent > 0);
  const warning = row.evaluation.warnings?.[0];
  return <Link className={`gallery-card kind-${kind}`} to={`/galeria/${encodeURIComponent(row.set.id)}`}>
    <div className="gallery-card-body">
      <div className="gallery-card-head">
        <GalleryBadge name={row.set.name} url={row.set.badge_url} />
        <div className="gallery-card-title">
          <strong>{row.set.name}</strong>
          <span>{row.evaluation.filled}/{row.evaluation.required} cartas · registrada {registered || "—"}</span>
        </div>
        <div className="gallery-predicted" aria-label={`Previsão ${predicted || "indisponível"}`}><span>prevista</span><strong>{predicted || "—"}</strong></div>
      </div>

      <div className="gallery-ladder">{GRADES.map((grade) => <span key={grade} className={ladderState(row, grade)}><b>{grade}</b><small>{kiloPoints(row.set.thresholds?.[grade])}</small></span>)}</div>

      <div className="gallery-score">
        <div className="gallery-score-head"><strong>{predicted ? formatPoints(row.evaluation.score) : "—"} <span>pts</span></strong><span>{scoreLine(row)}</span></div>
        <div className="gallery-bar" role="progressbar" aria-label="Pontos previstos" aria-valuemin={0} aria-valuemax={top} aria-valuenow={predicted ? row.evaluation.score : 0}>
          <span style={{ width: `${predicted ? Math.min(100, (row.evaluation.score / top) * 100) : 0}%` }} />
          {GRADES.slice(0, 4).map((grade) => row.set.thresholds?.[grade] ? <i key={grade} style={{ left: `${(row.set.thresholds[grade]! / top) * 100}%` }} /> : null)}
        </div>
        <span className="gallery-equation">{predicted ? `${formatPoints(row.evaluation.base_score)} base + ${formatPoints(row.evaluation.bonus_score)} bônus` : "sem combinação avaliável ainda"}</span>
      </div>

      {tags.length > 0 && <div className="gallery-tags">{tags.map((tag) => <span key={`${tag.name}-${tag.attribute}`}>{tag.name} <b>+{tag.bonus_percent}%</b></span>)}</div>}

      <div className="gallery-reward">
        <span className={`gallery-reward-art${reward?.reward.image_url ? " has-image" : ""}`} aria-hidden="true">{reward?.reward.image_url ? <img src={reward.reward.image_url} alt="" loading="lazy" /> : null}</span>
        <div>
          <span>{reward ? `próxima recompensa · ${reward.grade}` : "recompensas"}</span>
          <strong>{reward ? rewardLabel(reward.reward) : "todas as faixas associadas ao resultado"}</strong>
        </div>
      </div>

      <div className="gallery-card-status">
        <span className={`gallery-status ${kind}`}>{kindLabel(row, kind)}</span>
        {warning && <span className="gallery-warning">{warning}</span>}
      </div>
    </div>
  </Link>;
}
