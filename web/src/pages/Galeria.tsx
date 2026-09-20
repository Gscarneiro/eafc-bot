import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { fetchGaleria } from "../api";
import { useData } from "../useData";
import type { GalleryGrade, GalleryRecord, GalleryReward } from "../types";
import PageHeader from "../components/PageHeader";
import "./Galeria.css";

const rank: Record<string, number> = { "": 0, D: 1, C: 2, B: 3, A: 4, S: 5 };
const grades: GalleryGrade[] = ["D", "C", "B", "A", "S"];

function label(row: GalleryRecord) {
  if (row.evaluation.status === "missing") return "faltam cartas";
  if (row.completion?.grade === "S") return "finalizada";
  if (row.evaluation.status === "incomplete" || row.evaluation.coverage !== "confirmada") return "cobertura parcial";
  if (row.completion && rank[row.evaluation.grade] <= rank[row.completion.grade]) return "sem avanço";
  return row.evaluation.grade || "pendente";
}

function categoryLabel(category: string | undefined) {
  return category?.trim() || "Outros conjuntos";
}

function rewardLabel(reward: GalleryReward) {
  const name = reward.label || reward.type || "recompensa";
  const amount = reward.count && reward.count > 1 ? `${reward.count}× ` : "";
  return `${amount}${name}${reward.value ? ` · ${reward.value.toLocaleString("pt-BR")}` : ""}`;
}

function nextReward(row: GalleryRecord) {
  const completionRank = rank[row.completion?.grade || ""];
  for (const grade of grades) {
    if (rank[grade] <= completionRank) continue;
    const rewards = row.set.rewards?.[grade] || [];
    if (rewards.length) return { grade, reward: rewards[0] };
  }
  return undefined;
}

function cardProgress(row: GalleryRecord) {
  const required = row.evaluation.required || row.set.required_cards;
  return required > 0 ? Math.min(100, Math.round((row.evaluation.filled / required) * 100)) : 0;
}

function scoreProgress(row: GalleryRecord) {
  if (row.evaluation.status !== "available") return undefined;
  const score = row.evaluation.score || 0;
  const current = row.evaluation.grade ? row.set.thresholds?.[row.evaluation.grade] || 0 : 0;
  const next = row.evaluation.next_threshold;
  if (!next) return 100;
  return Math.max(0, Math.min(100, Math.round(((score - current) / (next - current)) * 100)));
}

function GalleryBadge({ name, url }: { name: string; url?: string }) {
  const [failed, setFailed] = useState(false);
  const fallback = !url || failed;
  return <span className="gallery-badge" aria-hidden="true">
    {fallback
      ? <span className="gallery-badge-fallback">{name.slice(0, 1).toUpperCase()}</span>
      : <img src={url} alt="" loading="lazy" onError={() => setFailed(true)} />}
  </span>;
}

export default function Galeria() {
  const [search, setSearch] = useState("");
  const [filter, setFilter] = useState("todos");
  const { data, loading, error, refetch } = useData(
    () => fetchGaleria(search ? `$search=${encodeURIComponent(search)}` : ""),
    [search],
  );
  const rows = useMemo(
    () => (data?.value ?? []).filter((row) => {
      if (filter === "todos") return true;
      if (filter === "disponiveis") return row.evaluation.status === "available" && row.evaluation.grade !== "";
      if (filter === "concluidas") return !!row.completion;
      if (filter === "pendentes") return row.evaluation.status === "incomplete" || row.evaluation.status === "missing" || row.evaluation.coverage !== "confirmada" || !!row.evaluation.warnings?.length;
      return row.evaluation.status === "missing";
    }),
    [data, filter],
  );
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

  return <div className="wrap galeria-page">
    <PageHeader
      eyebrow="elenco · fut gallery"
      title="FUT Gallery"
      meta={data ? `${data["@odata.count"]} conjuntos` : "catálogo e histórico"}
      actions={<Link className="btn ghost" to="/galeria/colecao">minha coleção</Link>}
    />
    <div className="gallery-toolbar">
      <input aria-label="Buscar conjuntos" placeholder="Buscar conjunto..." value={search} onChange={(event) => setSearch(event.target.value)} />
      <select aria-label="Filtrar conjuntos" value={filter} onChange={(event) => setFilter(event.target.value)}>
        <option value="todos">todos</option>
        <option value="disponiveis">disponíveis</option>
        <option value="pendentes">dados pendentes</option>
        <option value="concluidas">concluídas</option>
        <option value="faltam">faltam cartas</option>
      </select>
    </div>
    {loading && <p className="hint">Carregando catálogo Gallery...</p>}
    {error && <div className="banner alert">{error.message}<button className="btn ghost" onClick={refetch}>tentar novamente</button></div>}
    {!loading && !error && rows.length === 0 && <div className="panel"><p className="hint">Nenhum conjunto avaliado ainda. Rode uma sincronização para carregar o catálogo.</p></div>}
    <div className="gallery-categories">
      {groups.map((group) => <section className="gallery-category" key={group.name}>
        <div className="gallery-category-head">
          <div>
            <p className="gallery-category-kicker">categoria</p>
            <h2>{group.name}</h2>
          </div>
          <span>{group.rows.length} {group.rows.length === 1 ? "conjunto" : "conjuntos"}</span>
        </div>
        <div className="gallery-grid">
          {group.rows.map((row) => {
            const cards = cardProgress(row);
            const points = scoreProgress(row);
            const reward = nextReward(row);
            return <Link className="gallery-card" to={`/galeria/${encodeURIComponent(row.set.id)}`} key={row.set.id}>
            <div className="gallery-card-head">
              <div className="gallery-card-title">
                <GalleryBadge name={row.set.name} url={row.set.badge_url} />
                <strong>{row.set.name}</strong>
              </div>
              <span className="gallery-grade" aria-label={`Previsão ${row.evaluation.grade || "indisponível"}`}>{row.evaluation.grade || "—"}</span>
            </div>
            {row.completion && <div className="gallery-registered">registrada {row.completion.grade}</div>}
            <div className="gallery-progress-copy"><span>cartas confirmadas</span><strong>{row.evaluation.filled}/{row.evaluation.required}</strong></div>
            <div className="gallery-progress" role="progressbar" aria-label="Cartas confirmadas" aria-valuemin={0} aria-valuemax={100} aria-valuenow={cards}><span style={{ width: `${cards}%` }} /></div>
            {points === undefined
              ? <div className="gallery-score pending-score">previsão pendente <span>{label(row)}</span></div>
              : <><div className="gallery-progress-copy"><span>pontos para {row.evaluation.next_grade || "S"}</span><strong>{row.evaluation.score.toLocaleString("pt-BR")}</strong></div><div className="gallery-progress score" role="progressbar" aria-label="Progresso de pontos" aria-valuemin={0} aria-valuemax={100} aria-valuenow={points}><span style={{ width: `${points}%` }} /></div><div className="gallery-score">{row.evaluation.next_grade ? `faltam ${Math.max(0, (row.evaluation.next_threshold || 0) - row.evaluation.score).toLocaleString("pt-BR")} pontos` : "nota S prevista"} <span>{label(row)}</span></div></>}
            {reward && <div className="gallery-reward"><span>próxima recompensa · {reward.grade}</span><strong>{rewardLabel(reward.reward)}</strong></div>}
            {row.evaluation.next_grade && <div className="gallery-next">próxima {row.evaluation.next_grade} · {row.evaluation.next_threshold?.toLocaleString("pt-BR")}</div>}
            {row.evaluation.warnings?.[0] && <div className="gallery-warning">{row.evaluation.warnings[0]}</div>}
          </Link>;
          })}
        </div>
      </section>)}
    </div>
  </div>;
}
