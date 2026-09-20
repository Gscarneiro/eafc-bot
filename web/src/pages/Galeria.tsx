import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { fetchGaleria } from "../api";
import { useData } from "../useData";
import type { GalleryRecord } from "../types";
import PageHeader from "../components/PageHeader";
import "./Galeria.css";

const rank: Record<string, number> = { "": 0, D: 1, C: 2, B: 3, A: 4, S: 5 };
function label(r: GalleryRecord) { if (r.evaluation.status === "missing") return "faltam cartas"; if (r.completion?.grade === "S") return "finalizada"; if (r.evaluation.status === "incomplete") return "cobertura parcial"; if (r.completion && rank[r.evaluation.grade] <= rank[r.completion.grade]) return "sem avanço"; return r.evaluation.grade || "pendente"; }

export default function Galeria() {
  const [search, setSearch] = useState(""); const [filter, setFilter] = useState("todos");
  const { data, loading, error, refetch } = useData(() => fetchGaleria(search ? `$search=${encodeURIComponent(search)}` : ""), [search]);
  const rows = useMemo(() => (data?.value ?? []).filter((r) => filter === "todos" || (filter === "disponiveis" ? r.evaluation.status === "available" && r.evaluation.grade !== "" : filter === "concluidas" ? !!r.completion : filter === "pendentes" ? r.evaluation.status === "incomplete" || r.evaluation.status === "missing" : r.evaluation.status === "missing")), [data, filter]);
  return <div className="wrap galeria-page"><PageHeader eyebrow="elenco · fut gallery" title="FUT Gallery" meta={data ? `${data["@odata.count"]} conjuntos` : "catálogo e histórico"} actions={<Link className="btn ghost" to="/galeria/colecao">minha coleção</Link>} />
    <div className="gallery-toolbar"><input aria-label="Buscar conjuntos" placeholder="Buscar conjunto..." value={search} onChange={(e)=>setSearch(e.target.value)} /><select aria-label="Filtrar conjuntos" value={filter} onChange={(e)=>setFilter(e.target.value)}><option value="todos">todos</option><option value="disponiveis">disponíveis</option><option value="pendentes">dados pendentes</option><option value="concluidas">concluídas</option><option value="faltam">faltam cartas</option></select></div>
    {loading && <p className="hint">Carregando catálogo Gallery...</p>}{error && <div className="banner alert">{error.message}<button className="btn ghost" onClick={refetch}>tentar novamente</button></div>}
    {!loading && !error && rows.length === 0 && <div className="panel"><p className="hint">Nenhum conjunto avaliado ainda. Rode uma sincronização para carregar o catálogo.</p></div>}
    <div className="gallery-grid">{rows.map((r)=><Link className="gallery-card" to={`/galeria/${encodeURIComponent(r.set.id)}`} key={r.set.id}><div className="gallery-card-head"><strong>{r.set.name}</strong><span className="gallery-grade">{r.completion?.grade || r.evaluation.grade || "—"}</span></div><div className="gallery-meta">{r.set.category || "Gallery"} · {r.evaluation.filled}/{r.evaluation.required} cartas</div><div className="gallery-score">{r.evaluation.score.toLocaleString("pt-BR")} pontos <span>{label(r)}</span></div>{r.evaluation.next_grade && <div className="gallery-next">próxima {r.evaluation.next_grade} · {r.evaluation.next_threshold?.toLocaleString("pt-BR")}</div>}{r.evaluation.warnings?.[0] && <div className="gallery-warning">{r.evaluation.warnings[0]}</div>}</Link>)}</div>
  </div>;
}
