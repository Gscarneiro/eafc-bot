import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { fetchEvolutionCatalog } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import Pagination from "../components/Pagination";
import { formatCoins } from "../format";
import { useData } from "../useData";
import type { EvolutionCatalogCollection, EvolutionCatalogItem } from "../types";
import "../shared.css";
import "./Evolucoes.css";

function buildQuery(params: URLSearchParams): string {
  const query = new URLSearchParams();
  const search = params.get("q")?.trim();
  const category = params.get("category")?.trim();
  if (search) query.set("$search", search);
  if (category && category !== "todas") query.set("$filter", `category eq '${category.replaceAll("'", "''")}'`);
  query.set("$top", "24");
  const page = Math.max(1, Number(params.get("page") || "1"));
  query.set("$skip", String((page - 1) * 24));
  return query.toString();
}

function costLabel(item: EvolutionCatalogItem): string {
  if (item.cost.free) return "grátis";
  const values: string[] = [];
  if (item.cost.coins > 0) values.push(`${formatCoins(item.cost.coins)} moedas`);
  if (item.cost.points > 0) values.push(`${formatCoins(item.cost.points)} pontos`);
  if (item.cost.tokens > 0) values.push(`${formatCoins(item.cost.tokens)} tokens`);
  return values.join(" · ") || "custo não informado";
}

function CatalogCard({ item }: { item: EvolutionCatalogItem }) {
  const slug = item.evolution.slug || item.evolution.id;
  return (
    <article className={`evo-catalog-card${item.expired ? " is-expired" : ""}`}>
      <div className="evo-card-topline">
        <Chip tone={item.expired ? "alert" : "turf"}>{item.category_label || item.category}</Chip>
        <span className="evo-card-origin">{item.origin_label || item.origin}</span>
      </div>
      <div className="evo-card-body">
        <div className="evo-card-heading">
          <div>
            <h2>{item.evolution.name}</h2>
            <p>{item.evolution.description || "Evolução publicada no catálogo do fut.gg."}</p>
          </div>
          <span className="evo-card-index" aria-hidden="true">{String(item.eligible_count).padStart(2, "0")}</span>
        </div>
        <div className="evo-card-meta">
          <span><strong>{item.eligible_count}</strong> {item.eligible_count === 1 ? "carta válida" : "cartas válidas"}</span>
          <span className="evo-card-cost">{costLabel(item)}</span>
          {item.repeatable && <span>repetível</span>}
        </div>
        <div className="evo-card-actions">
          <Link className="btn btn-primary" to={`/evolucoes/catalogo/${encodeURIComponent(slug)}`}>abrir laboratório</Link>
          {item.sources?.[0]?.url && <a className="evo-source-link" href={item.sources[0].url} target="_blank" rel="noreferrer">fonte fut.gg ↗</a>}
        </div>
      </div>
    </article>
  );
}

export default function Evolucoes() {
  const [params, setParams] = useSearchParams();
  const query = useMemo(() => buildQuery(params), [params]);
  const state = useData(() => fetchEvolutionCatalog(query), [query]);
  const gate = asyncGate(state.loading || false, state.error, state.data !== null, state.refetch);
  if (gate) return gate;
  const data = state.data as EvolutionCatalogCollection | null;
  if (!data) return null;
  const summary = data["@eafc.summary"];
  const categories = summary.categories ?? [];
  const selectedCategory = params.get("category") || "todas";
  const search = params.get("q") || "";
  const page = Math.max(1, Number(params.get("page") || "1"));
  const pages = data["@eafc.top"] > 0 ? Math.ceil(data["@odata.count"] / data["@eafc.top"]) : 0;
  const setParam = (key: string, value: string) => {
    const next = new URLSearchParams(params);
    if (!value || value === "todas") next.delete(key); else next.set(key, value);
    if (key !== "page") next.delete("page");
    setParams(next);
  };
  const setPage = (nextPage: number) => {
    const next = new URLSearchParams(params);
    if (nextPage <= 1) next.delete("page"); else next.set("page", String(nextPage));
    setParams(next);
  };
  const visibleCategories = [{ key: "todas", label: "Todas", count: summary.total }, ...categories];
  return (
    <div className="wrap evo-catalog-page">
      <PageHeader eyebrow="planejamento" title="Evoluções" meta="O catálogo do jogo, organizado pela taxonomia publicada na fonte." />
      <section className="evo-catalog-hero">
        <div>
          <span className="evo-hero-kicker">evolution lab</span>
          <h2>Escolha uma carta. Entenda o salto.</h2>
          <p>Explore cada seção oficial, veja o que muda em cada atributo e abra uma análise especialista quando quiser uma segunda opinião.</p>
        </div>
        <div className="evo-hero-stamp"><span>fonte</span><strong>fut.gg</strong><small>catálogo vivo</small></div>
      </section>
      <div className="stat-grid evo-catalog-stats">
        <div className="stat-tile"><div className="label">evoluções ativas</div><div className="value up">{summary.total}</div><div className="sub">todas as seções</div></div>
        <div className="stat-tile"><div className="label">com cartas válidas</div><div className="value up">{summary.eligible}</div><div className="sub">cruzadas com seu elenco</div></div>
        <div className="stat-tile"><div className="label">categorias oficiais</div><div className="value">{categories.length}</div><div className="sub">rótulo da fonte, sem chute</div></div>
        <div className="stat-tile"><div className="label">expiradas</div><div className="value down">{summary.expired}</div><div className="sub">mantidas para auditoria</div></div>
      </div>
      <div className="evo-catalog-toolbar">
        <label className="evo-catalog-search" htmlFor="evo-catalog-search"><span>buscar no catálogo</span><input id="evo-catalog-search" value={search} placeholder="nome da evolução…" onChange={(event) => setParam("q", event.target.value)} /></label>
        <span className="evo-toolbar-count">{data["@odata.count"]} resultado{data["@odata.count"] === 1 ? "" : "s"}</span>
      </div>
      <nav className="evo-category-tabs" aria-label="Categorias de evolução">
        {visibleCategories.map((category) => <button type="button" key={category.key} className={selectedCategory === category.key ? "active" : ""} onClick={() => setParam("category", category.key)}><span>{category.label}</span><strong>{category.count}</strong></button>)}
      </nav>
      {data.value.length === 0 ? <EmptyState message="Nenhuma evolução encontrada nesta seção." hint={selectedCategory !== "todas" || search ? "Tente outra categoria ou limpe a busca." : "A coleta ainda não trouxe evoluções para este ciclo."} /> : <div className="evo-catalog-grid">{data.value.map((item) => <CatalogCard key={item.evolution.id || item.evolution.slug} item={item} />)}</div>}
      {pages > 1 && <Pagination page={page} pages={pages} onPage={setPage} />}
      <p className="evo-catalog-footnote">Taxonomia e custos vêm da resposta do fut.gg. A categoria é exclusiva; origem e preço aparecem apenas como contexto.</p>
    </div>
  );
}
