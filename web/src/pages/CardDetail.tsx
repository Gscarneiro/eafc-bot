import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { RadarChart, PolarGrid, PolarAngleAxis, Radar, ResponsiveContainer, Tooltip } from "recharts";
import { ApiError, fetchCard, fetchEvolutionPlan, saveEvolutionProgress } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import GGRating from "../components/GGRating";
import TrendChart from "../components/TrendChart";
import { formatCoins, formatDate, formatSigned, styleNames } from "../format";
import { useData } from "../useData";
import type { CardDetailResponse, DetailedAttributes, EvoPotential, EvolutionGraph, EvolutionGraphTransition, PlayStyleDefinition, PlayStyleRecommendation, Position } from "../types";
import "../shared.css";
import "./CardDetail.css";

const labels: Record<string, string> = {
  acceleration: "Aceleração", sprint_speed: "Velocidade", agility: "Agilidade", balance: "Equilíbrio", jumping: "Impulsão", stamina: "Resistência", strength: "Força", reactions: "Reações", aggression: "Agressividade", composure: "Compostura", interceptions: "Interceptações", positioning: "Posicionamento", vision: "Visão", ball_control: "Controle de bola", crossing: "Cruzamento", dribbling: "Drible", finishing: "Finalização", fk_accuracy: "Precisão em faltas", heading_accuracy: "Precisão de cabeceio", long_passing: "Passe longo", short_passing: "Passe curto", defensive_awareness: "Consciência defensiva", shot_power: "Potência do chute", long_shots: "Chutes longos", standing_tackle: "Desarme em pé", sliding_tackle: "Carrinho", volleys: "Voleios", curve: "Curva", penalties: "Pênaltis", gk_diving: "Defesas", gk_handling: "Manuseio", gk_kicking: "Tiro", gk_reflexes: "Reflexos", gk_speed: "Velocidade", gk_positioning: "Posicionamento",
};

const groups: Record<string, string[]> = {
  "Ritmo": ["acceleration", "sprint_speed"],
  "Finalização": ["positioning", "finishing", "shot_power", "long_shots", "volleys", "penalties"],
  "Passe": ["vision", "crossing", "fk_accuracy", "short_passing", "long_passing", "curve"],
  "Drible": ["agility", "balance", "reactions", "ball_control", "dribbling", "composure"],
  "Defesa": ["interceptions", "heading_accuracy", "defensive_awareness", "standing_tackle", "sliding_tackle"],
  "Físico": ["jumping", "stamina", "strength", "aggression"],
};

const keeperGroups: Record<string, string[]> = { "Defesas": ["gk_diving", "gk_handling", "gk_kicking", "gk_reflexes", "gk_speed", "gk_positioning"] };

function Stars({ value, label }: { value: number; label: string }) {
  const safe = Math.max(0, Math.min(5, value || 0));
  return <div className="detail-rating" aria-label={`${label}: ${safe} de 5 estrelas`}><span className="detail-rating-label">{label}</span><span className="detail-stars" aria-hidden="true">{Array.from({ length: 5 }, (_, i) => <span key={i} className={i < safe ? "is-filled" : ""}>★</span>)}</span><strong>{safe}/5</strong></div>;
}

function RadarTooltip({ active, payload, label }: { active?: boolean; payload?: Array<{ value?: number }>; label?: string }) {
  if (!active || !payload?.length) return null;
  return <div className="detail-tooltip"><strong>{label}</strong><span>Nota: {payload[0].value ?? "—"}</span></div>;
}

function RadarFace({ p }: { p: CardDetailResponse["player"] }) {
  const a = p.attributes;
  const data = [{ name: p.position === "GK" ? "DIV" : "PAC", value: a.pace }, { name: p.position === "GK" ? "HAN" : "SHO", value: a.shooting }, { name: p.position === "GK" ? "KIC" : "PAS", value: a.passing }, { name: p.position === "GK" ? "REF" : "DRI", value: a.dribbling }, { name: p.position === "GK" ? "SPD" : "DEF", value: a.defending }, { name: p.position === "GK" ? "POS" : "PHY", value: a.physical }];
  return <div className="detail-radar" aria-label="Radar dos seis atributos principais"><ResponsiveContainer width="100%" height={280}><RadarChart data={data}><PolarGrid stroke="var(--detail-chart-grid, var(--rule))"/><PolarAngleAxis dataKey="name" stroke="var(--ink-2)"/><Radar dataKey="value" stroke="var(--turf)" fill="var(--turf)" fillOpacity={0.32}/><Tooltip content={<RadarTooltip/>}/></RadarChart></ResponsiveContainer></div>;
}

function AttributeGroups({ attrs, keeper }: { attrs?: DetailedAttributes; keeper: boolean }) {
  const source = keeper ? keeperGroups : groups;
  const values = attrs ?? {};
  const sections = Object.entries(source).map(([title, keys]) => ({ title, rows: keys.map(key => [key, values[key]] as const).filter(([, value]) => typeof value === "number") })).filter(section => section.rows.length > 0);
  if (!sections.length) return <div className="empty">Subatributos não disponíveis nesta coleta.</div>;
  return <div className="detail-attribute-groups">{sections.map(section => <section className="detail-attribute-group" key={section.title}><h3>{section.title}</h3>{section.rows.map(([key, value]) => <div className="detail-bar" key={key}><div><span>{labels[key] ?? key}</span><strong>{value}</strong></div><span className="detail-track"><i style={{ width: `${Math.max(0, Math.min(100, value as number))}%` }}/></span></div>)}</section>)}</div>;
}

function PositionPicker({ positions, selected, ratings, onSelect }: { positions: Position[]; selected: Position; ratings: Partial<Record<Position, number>>; onSelect: (position: Position) => void }) {
  return <div className="detail-position-picker" role="tablist" aria-label="Posições avaliadas pelo fut.gg">{positions.map(position => <button key={position} type="button" role="tab" aria-selected={position === selected} className={position === selected ? "is-selected" : ""} onClick={() => onSelect(position)}><strong>{ratings[position] ? ratings[position]!.toFixed(1) : "—"}</strong><span>{position}</span></button>)}</div>;
}

function PlayStyleGallery({ p, catalog, recommendation }: { p: CardDetailResponse["player"]; catalog?: PlayStyleDefinition[]; recommendation?: PlayStyleRecommendation }) {
  const [selected, setSelected] = useState<PlayStyleDefinition | null>(null);
  const current = p.play_styles ?? [];
  const currentByName = new Map(current.map(style => [style.name.toLowerCase(), style]));
  const recommended = new Set((recommendation?.styles ?? []).map(style => style.name.toLowerCase()));
  const categories = [...new Set((catalog ?? []).map(item => item.category || "Outros"))];
  const items: PlayStyleDefinition[] = catalog?.length ? catalog : current.map(style => ({ ea_id: style.ea_id ?? 0, name: style.name, category: "Presentes" }));
  return <div className="detail-playstyles"><div className="detail-source-note">{recommendation?.source === "bot" ? "Sugestão do bot · baseada na posição e nas funções da carta" : "Recomendação oficial do fut.gg"}</div>{(categories.length ? categories : ["Presentes"]).map(category => <div className="detail-playstyle-category" key={category}><h3>{category}</h3><div className="detail-ps-grid">{items.filter(item => (item.category || "Outros") === category).map(item => { const actual = currentByName.get(item.name.toLowerCase()); const isRecommended = recommended.has(item.name.toLowerCase()); return <button type="button" className={`detail-ps ${actual ? "is-present" : "is-muted"} ${isRecommended ? "is-recommended" : ""}`} key={`${item.ea_id}-${item.name}`} aria-pressed={selected?.name === item.name} onClick={() => setSelected(item)}><span className="detail-ps-icon">{item.image_url ? <img src={item.image_url} alt="" loading="lazy"/> : "✦"}</span><span>{item.name}{actual?.plus ? " +" : ""}</span>{isRecommended && <small>indicado</small>}</button>; })}</div></div>)}{selected && <aside className="detail-ps-detail" aria-live="polite"><div>{selected.image_url ? <img src={selected.image_url} alt=""/> : <span className="detail-ps-detail-fallback">✦</span>}</div><div><h3>{selected.name}{currentByName.get(selected.name.toLowerCase())?.plus ? " +" : ""}</h3><p>{currentByName.get(selected.name.toLowerCase())?.plus ? selected.plus_description || selected.description || "PlayStyle+ presente nesta carta." : selected.description || "Descrição indisponível no catálogo atual."}</p></div></aside>}</div>;
}

function Path({ potential }: { potential: EvoPotential }) { return <div className="detail-potential"><div className="detail-potential-head"><strong>{potential.final_overall} · GG final {potential.final_gg_rating.toFixed(1)}</strong><Chip tone="gain">{formatSigned(potential.gg_rating_gain)} GG ganho</Chip></div><div>{potential.coin_cost ? formatCoins(potential.coin_cost) : "grátis"}{potential.point_cost ? ` · ${potential.point_cost} pontos` : ""}</div><p>{potential.gained_play_styles?.length ? `Novos PlayStyles: ${styleNames(potential.gained_play_styles)}` : ""}</p></div>; }

// TransitionCard mostra UMA aresta do grafo confirmado — não só o "melhor"
// caminho (isso já é o Path acima), a estrutura inteira: branch/rejoin são
// calculados aqui, no cliente, contando quantas transições do MESMO grafo
// compartilham from/to (o JSON só traz nós+arestas crus, sem esses flags).
function TransitionCard({ transition, graph, transitions, completed, onToggle }: { transition: EvolutionGraphTransition; graph: EvolutionGraph; transitions: EvolutionGraphTransition[]; completed: string[]; onToggle: (name: string, checked: boolean) => void }) {
  const from = graph.nodes?.[transition.from];
  const to = graph.nodes?.[transition.to];
  const isBranch = transitions.filter((t) => t.from === transition.from).length > 1;
  const isRejoin = transitions.filter((t) => t.to === transition.to).length > 1;
  const done = completed.includes(transition.evolution);
  return <div className="detail-potential evo-transition"><div className="detail-potential-head"><strong>{from?.card.rating ?? "?"} → {to?.card.rating ?? "?"}{to?.card.gg_rating ? ` · GG final ${to.card.gg_rating.toFixed(1)}` : ""}</strong><Chip tone={transition.is_expired ? "alert" : "coin"}>{transition.coin_cost ? formatCoins(transition.coin_cost) : "grátis"}</Chip></div><p className="evo-transition-name">{transition.evolution}</p><div className="evo-transition-badges">{isBranch && <Chip tone="flat">ramificação</Chip>}{isRejoin && <Chip tone="flat">reencontro</Chip>}{transition.lab && <Chip tone="flat">Lab</Chip>}{transition.repeatable && <Chip tone="flat">repetível</Chip>}{transition.is_expired && <Chip tone="alert">expirada</Chip>}</div><div>{transition.training_time || "tempo não informado"}{transition.point_cost ? ` · ${transition.point_cost} pontos` : ""}</div><label className="evo-transition-done"><input type="checkbox" checked={done} onChange={(event) => onToggle(transition.evolution, event.target.checked)} /> já concluí</label></div>;
}

// EvolutionWorkbench é a visão estrutural completa do grafo, ao lado da
// visão "recomendado" (Path/Best/Alternates acima, já filtrada por ganho).
// Seção secundária: qualquer falha de rede fica silenciosa aqui — Best/
// Alternates já cobrem o essencial da tela.
function EvolutionWorkbench({ slug }: { slug: string }) {
  const { data: plan, loading, error } = useData(() => fetchEvolutionPlan(slug), [slug]);
  const [completed, setCompleted] = useState<string[]>([]);
  useEffect(() => { if (plan?.completed) setCompleted(plan.completed); }, [plan]);

  if (loading || error || !plan) return null;
  if (plan.status === "not_eligible" || plan.status === "not_checked") return null;

  const toggle = async (name: string, checked: boolean) => {
    const next = checked ? [...completed, name] : completed.filter((item) => item !== name);
    setCompleted(next);
    try { await saveEvolutionProgress(slug, next); } catch { setCompleted(completed); }
  };

  const graph = plan.graph;
  // graph.transitions espelha um []domain.EvolutionTransition do Go: sem
  // caminho confirmado (carta já no teto, por exemplo), o campo vem null,
  // não `[]` — mesma convenção de EvolutionPath.chain neste mesmo arquivo.
  const transitions = graph?.transitions ?? [];
  const estimated = plan.estimated_only ?? [];
  if (transitions.length === 0 && estimated.length === 0 && plan.status !== "fetch_error") return null;

  return <div className="evo-workbench">
    <h3>Workbench — todos os ramos</h3>
    {plan.status === "fetch_error" && <div className="empty">Falha ao confirmar caminhos nesta coleta{plan.error ? `: ${plan.error}` : "."}</div>}
    {graph && transitions.length > 0 && <div className="evo-transition-grid">{transitions.map((transition, index) => <TransitionCard key={`${transition.from}-${transition.to}-${index}`} transition={transition} graph={graph} transitions={transitions} completed={completed} onToggle={toggle} />)}</div>}
    {estimated.length > 0 && <div className="evo-estimated"><h4>Elegível pelas regras, sem caminho confirmado pelo fut.gg</h4><ul>{estimated.map((item) => <li key={item.evolution.id}><strong>{item.evolution.name}</strong> · {item.acquisition}{item.status === "fetch_error" ? " (não foi possível confirmar nesta coleta)" : ""}</li>)}</ul></div>}
  </div>;
}

export default function CardDetail() {
  const { slug = "" } = useParams<{ slug: string }>();
  const { data: report, error, loading, refetch } = useData(() => fetchCard(slug), [slug]);
  const [selectedPosition, setSelectedPosition] = useState<Position | null>(null);
  if (error instanceof ApiError && error.status === 404) return <div className="wrap narrow"><Link className="back-link" to="/time">← voltar para o time</Link><div className="empty">Carta não encontrada na última coleta.</div></div>;
  const gate = asyncGate(loading, error, !!report, refetch); if (gate) return gate; if (!report) return null;
  const p = report.player;
  const allPositions = [p.position, ...(p.alt_positions ?? []), ...(p.gg_rating_pos ? [p.gg_rating_pos] : []), ...Object.keys(p.gg_ratings ?? {}) as Position[]];
  const positions = [...new Set(allPositions)].filter(Boolean);
  const ratings: Partial<Record<Position, number>> = { ...(p.gg_ratings ?? {}) };
  if (p.gg_rating && p.gg_rating_pos && !ratings[p.gg_rating_pos]) ratings[p.gg_rating_pos] = p.gg_rating;
  const bestPosition = positions.reduce<Position | null>((best, position) => !best || (ratings[position] ?? 0) > (ratings[best] ?? 0) ? position : best, null) ?? p.position;
  const activePosition = selectedPosition && positions.includes(selectedPosition) ? selectedPosition : bestPosition;
  const activeRating = ratings[activePosition] ?? (p.gg_rating_pos === activePosition ? p.gg_rating : undefined);
  const recommendation = report.play_style_recommendations?.find(item => item.position === activePosition);
  const pricePoints = (report.price_series ?? []).filter(point => point.coins > 0).sort((a, b) => a.observed_at.localeCompare(b.observed_at)).map(point => ({ label: formatDate(point.observed_at), value: point.coins }));
  const stats = p.club_stats;
  return <div className="wrap detail-page"><Link className="back-link" to="/time">← voltar para o time</Link><div className="detail-layout"><aside className="detail-sidebar"><div className="detail-hero">{p.image_url && <img src={p.image_url} alt={`Arte de ${p.common_name || p.name}`}/>}<div><h1>{p.common_name || p.name}</h1><p>{p.version} · {p.club}</p><div className="detail-hero-ratings"><Chip tone="flat">EA {p.rating}</Chip><GGRating current={p.gg_rating} currentPosition={p.gg_rating_pos} positional={activeRating} positionalPosition={activePosition} variant="detail" /></div></div></div><section><h2>Nota por posição</h2><PositionPicker positions={positions} selected={activePosition} ratings={ratings} onSelect={setSelectedPosition}/><p className="detail-helper">A nota EA da carta é {p.rating}. A GG atual identifica esta cópia do clube; a nota posicional vem da referência do fut.gg e pode ser compartilhada entre cópias da mesma carta.</p></section><section><h2>Ficha</h2><dl className="detail-facts"><div><dt>Liga</dt><dd>{p.league || "—"}</dd></div><div><dt>Nação</dt><dd>{p.nation || "—"}</dd></div><div><dt>Altura</dt><dd>{p.height_cm ? `${p.height_cm} cm` : "—"}</dd></div><div><dt>Pé</dt><dd>{p.foot || "—"}</dd></div><div><dt>AcceleRATE</dt><dd>{p.accelerate_type || "não informado"}</dd></div><div><dt>Coleta</dt><dd>{formatDate(report.generated_at)}</dd></div></dl><div className="detail-star-grid"><Stars value={p.skill_moves} label="Habilidades"/><Stars value={p.weak_foot} label="Pé fraco"/></div></section><section><h2>No clube</h2><p>{p.untradeable ? "Intransferível" : "Transferível"}{p.in_squad ? " · no time titular" : ""}</p>{stats && <div className="detail-stat-grid">{[["Jogos", stats.games], ["Gols", stats.goals], ["Assistências", stats.assists]].map(([label, value]) => <div key={label as string}><strong>{value ?? "—"}</strong><span>{label}</span></div>)}</div>}</section></aside><main className="detail-main"><section><div className="detail-section-heading"><div><span className="detail-kicker">ANÁLISE VISUAL</span><h2>Atributos principais</h2></div><span className="detail-selection">Posição: {activePosition}</span></div><RadarFace p={{ ...p, position: activePosition }}/><AttributeGroups attrs={p.detailed_attributes} keeper={activePosition === "GK"}/></section><section><div className="detail-section-heading"><div><span className="detail-kicker">FUNÇÕES</span><h2>PlayStyles para {activePosition}</h2></div>{recommendation?.role && <span className="detail-selection">Role: {recommendation.role}</span>}</div><PlayStyleGallery p={p} catalog={report.play_style_catalog} recommendation={recommendation}/></section><section><div className="detail-section-heading"><div><span className="detail-kicker">MERCADO</span><h2>Referência de mercado</h2></div><span className="detail-selection">{report.price_history_status}</span></div>{pricePoints.length > 1 ? <TrendChart data={pricePoints} valueFormatter={formatCoins} height={220}/> : <div className="empty">Histórico insuficiente para desenhar tendência; não representa preço ao vivo.</div>}</section><section><h2>Evolução</h2>{report.best ? <Path potential={report.best}/> : <div className="empty">{report.evolution_status === "not_checked" ? "Ainda não verificada para esta carta." : "Nenhum caminho confirmado nesta coleta."}</div>}{(report.alternates ?? []).map((potential, index) => <Path key={index} potential={potential}/>)}<EvolutionWorkbench slug={slug} /></section>{(report.related_cards ?? []).length > 0 && <section><h2>Outras cartas deste atleta no clube</h2><div className="detail-related">{report.related_cards?.map(card => <Link key={card.card_slug} to={`/time/${card.card_slug}`}>{card.player.rating} {card.player.position} · {card.player.version}</Link>)}</div></section>}</main></div></div>;
}
