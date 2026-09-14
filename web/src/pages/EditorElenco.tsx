import { useEffect, useMemo, useRef, useState } from "react";
import {
  applySquadPlanReference,
  deleteSquadPlan,
  evaluateSquadEditor,
  fetchSavedSquadPlans,
  fetchSquadEditor,
  saveSquadPlan,
} from "../api";
import { asyncGate } from "../components/asyncGate";
import PageHeader from "../components/PageHeader";
import { evaluationSourceLabel, formatCoins } from "../format";
import { useData } from "../useData";
import type {
  ClubPlayer,
  Position,
  SavedSquadPlan,
  SavedSquadPlanInput,
  SavedSquadPlanView,
  SquadCardReference,
  SquadEditorEvaluation,
  SquadPlanSlot,
  StarterCard,
} from "../types";
import "../shared.css";
import "./EditorElenco.css";

type Draft = {
  formation: string;
  formationSource: "confirmada" | "manual";
  gameStyle: string;
  slots: SquadPlanSlot[];
  bench: SquadCardReference[];
  unrelated: SquadCardReference[];
};
type Pool = "disponiveis" | "banco" | "nao_relacionados" | "alvos";
type EditorCard = {
	player: ClubPlayer;
	reference: SquadCardReference;
	origin: "clube" | "mercado" | "evolucao";
	cost?: number;
	description?: string;
};

const FORMATIONS: Record<string, Position[]> = {
  "4-3-3": ["GK", "RB", "CB", "CB", "LB", "CM", "CM", "CM", "RW", "ST", "LW"],
  "4-2-3-1": ["GK", "RB", "CB", "CB", "LB", "CDM", "CDM", "RM", "CAM", "LM", "ST"],
  "4-4-2": ["GK", "RB", "CB", "CB", "LB", "RM", "CM", "CM", "LM", "ST", "ST"],
  "3-4-2-1": ["GK", "CB", "CB", "CB", "RM", "CM", "CM", "LM", "CF", "CF", "ST"],
};

function cardRef(player: ClubPlayer): SquadCardReference {
  return { club_item_id: player.club_item_id || undefined, player_id: player.id };
}

function refKey(ref: SquadCardReference): string {
	const origin = ref.origem || "clube";
	const evolution = ref.evolucao_id ? `${ref.evolucao_id}:` : "";
  return ref.club_item_id ? `${origin}:${evolution}item:${ref.club_item_id}` : `${origin}:${evolution}player:${ref.player_id ?? 0}`;
}

function playerName(player: ClubPlayer): string { return player.common_name || player.name; }
function playsAt(player: ClubPlayer, position: Position): boolean { return player.position === position || (player.alt_positions ?? []).includes(position); }

function slotsFromStarters(starters: StarterCard[], formation: string): SquadPlanSlot[] {
  const current = starters.slice().sort((a, b) => a.index - b.index);
  if (current.length === 11) return current.map((starter) => ({ index: starter.index, posicao: starter.position, carta: cardRef(starter.player) }));
  return (FORMATIONS[formation] ?? FORMATIONS["4-3-3"]).map((posicao, index) => ({ index, posicao, carta: {} }));
}

function fromPlan(plan: SavedSquadPlan): Draft {
  return {
    formation: plan.formacao, formationSource: plan.origem_formacao, gameStyle: plan.estilo_jogo ?? "",
    slots: plan.vagas.map((slot) => ({ ...slot, carta: { ...slot.carta } })),
    bench: (plan.banco ?? []).map((card) => ({ ...card })), unrelated: (plan.nao_relacionados ?? []).map((card) => ({ ...card })),
  };
}

function inputFromDraft(name: string, draft: Draft, expectedRevision?: number): SavedSquadPlanInput {
  return { nome: name.trim(), formacao: draft.formation, origem_formacao: draft.formationSource, estilo_jogo: draft.gameStyle.trim() || undefined, revisao_esperada: expectedRevision, vagas: draft.slots, banco: draft.bench, nao_relacionados: draft.unrelated };
}
function draftKey(club: string, cycle: string | undefined) { return `eafc-editor-elenco-v2:${club}:${cycle || "atual"}`; }
function readDraft(key: string): Draft | null {
  try { const raw = localStorage.getItem(key); const value = raw ? JSON.parse(raw) as Draft : null; return value?.slots?.length === 11 ? value : null; } catch { return null; }
}
function referencesIn(draft: Draft): Set<string> { return new Set(draft.slots.filter((slot) => slot.carta.player_id || slot.carta.club_item_id).map((slot) => refKey(slot.carta))); }

function formationLines(formation: string, slots: SquadPlanSlot[]): SquadPlanSlot[][] | null {
  const rows = formation.replace(/\(.*\)/, "").trim().split("-").map((part) => Number.parseInt(part, 10));
  if (rows.some((row) => !Number.isFinite(row) || row <= 0) || rows.reduce((total, row) => total + row, 0) !== 10 || slots.length !== 11) return null;
  const sorted = slots.slice().sort((a, b) => a.index - b.index);
  const goalkeeper = sorted[0]!;
  let cursor = 1;
  const fieldRows = rows.map((size) => {
    const line = sorted.slice(cursor, cursor + size);
    cursor += size;
    return line.reverse();
  });
  return [...fieldRows.reverse(), [goalkeeper]];
}

export default function EditorElenco() {
  const editorData = useData(fetchSquadEditor, []);
  const [plans, setPlans] = useState<SavedSquadPlanView[]>([]);
  const [plansError, setPlansError] = useState("");
  const [draft, setDraft] = useState<Draft | null>(null);
  const [history, setHistory] = useState<Draft[]>([]);
  const [future, setFuture] = useState<Draft[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [focusedSlot, setFocusedSlot] = useState<number | null>(null);
  const [pool, setPool] = useState<Pool>("disponiveis");
  const [search, setSearch] = useState("");
  const [visibleCards, setVisibleCards] = useState(60);
  const [planName, setPlanName] = useState("Plano principal");
  const [openedPlan, setOpenedPlan] = useState<SavedSquadPlan | null>(null);
  const [saving, setSaving] = useState(false);
  const [actionError, setActionError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [planPendingDelete, setPlanPendingDelete] = useState<SavedSquadPlan | null>(null);
  const [evaluation, setEvaluation] = useState<SquadEditorEvaluation | null>(null);
  const [evaluationError, setEvaluationError] = useState("");
  const [evaluating, setEvaluating] = useState(false);
  const initializedFor = useRef("");
  const evaluationSequence = useRef(0);
  const data = editorData.data;
  const key = data ? draftKey(data.clube, data.avaliacao?.ciclo) : "";

  const refreshPlans = async () => {
    try { const response = await fetchSavedSquadPlans(); setPlans(response.value ?? []); setPlansError(""); }
    catch (error) { setPlansError(error instanceof Error ? error.message : "Não foi possível carregar os planos salvos."); }
  };

  useEffect(() => {
    if (!data || initializedFor.current === key) return;
    initializedFor.current = key;
    const initial: Draft = { formation: data.formacao || "4-3-3", formationSource: (data.titulares ?? []).length === 11 ? "confirmada" : "manual", gameStyle: "", slots: slotsFromStarters(data.titulares ?? [], data.formacao), bench: [], unrelated: [] };
    setDraft(readDraft(key) ?? initial); setHistory([]); setFuture([]); setSelected(null); setFocusedSlot(null); void refreshPlans();
  }, [data, key]);

  useEffect(() => { if (draft && key) localStorage.setItem(key, JSON.stringify(draft)); }, [draft, key]);
  useEffect(() => { setVisibleCards(60); }, [pool, search]);
  const evaluationKey = useMemo(() => draft ? JSON.stringify({ formation: draft.formation, gameStyle: draft.gameStyle, slots: draft.slots }) : "", [draft]);
  useEffect(() => {
    if (!draft || !evaluationKey) return;
    const sequence = ++evaluationSequence.current;
    const timer = window.setTimeout(() => {
      setEvaluating(true); setEvaluationError("");
      evaluateSquadEditor(draft.formation, draft.slots, draft.gameStyle)
        .then((result) => { if (sequence === evaluationSequence.current) setEvaluation(result); })
        .catch((error: unknown) => { if (sequence === evaluationSequence.current) setEvaluationError(error instanceof Error ? error.message : "Não foi possível avaliar o rascunho."); })
        .finally(() => { if (sequence === evaluationSequence.current) setEvaluating(false); });
    }, 240);
    return () => window.clearTimeout(timer);
  }, [draft?.formation, evaluationKey]);

  const gate = asyncGate(editorData.loading, editorData.error, !!data, editorData.refetch);
  if (gate) return gate;
  if (!data || !draft) return null;

  const cards = data.cartas ?? [];
	const ownedCards: EditorCard[] = cards.map(({ player }) => ({ player, reference: cardRef(player), origin: "clube" }));
	const targetCards: EditorCard[] = (data.alvos ?? []).map((target) => ({ player: target.player, reference: target.referencia, origin: target.tipo, cost: target.custo, description: target.descricao }));
	const editorCards = [...ownedCards, ...targetCards];
  const cardsByKey = new Map(editorCards.map((card) => [refKey(card.reference), card]));
	const selectedCard = selected ? cardsByKey.get(selected) : undefined;
  const selectedPlayer = selectedCard?.player;
  const used = referencesIn(draft);
  const bench = new Set(draft.bench.map(refKey));
  const unrelated = new Set(draft.unrelated.map(refKey));
	const availableCount = ownedCards.filter(({ reference }) => { const ref = refKey(reference); return !used.has(ref) && !bench.has(ref) && !unrelated.has(ref); }).length;
	const sourceCards = (pool === "alvos" ? targetCards : ownedCards).filter(({ reference }) => {
		const ref = refKey(reference);
    if (used.has(ref)) return false;
		if (pool === "alvos") return true;
    if (pool === "banco") return bench.has(ref);
    if (pool === "nao_relacionados") return unrelated.has(ref);
    return !bench.has(ref) && !unrelated.has(ref);
	}).filter(({ player, description }) => `${playerName(player)} ${player.position} ${player.version ?? ""} ${description ?? ""}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()));
  const evaluationByIndex = new Map((evaluation?.vagas ?? []).map((slot) => [slot.index, slot]));
  const slotInFocus = focusedSlot === null ? null : draft.slots.find((slot) => slot.index === focusedSlot) ?? null;
	const evaluationInFocus = slotInFocus ? evaluationByIndex.get(slotInFocus.index) : undefined;
	const rolesInFocus = slotInFocus ? (data.funcoes ?? []).filter((role) => role.posicao === slotInFocus.posicao) : [];

  const commit = (next: Draft) => { setHistory((entries) => [...entries.slice(-29), draft]); setFuture([]); setDraft(next); setActionError(""); setActionMessage(""); };
	const assign = (index: number, card: EditorCard) => {
		const reference = card.reference; const id = refKey(reference); const target = draft.slots.findIndex((slot) => slot.index === index); if (target < 0) return;
    const existing = draft.slots.findIndex((slot) => refKey(slot.carta) === id); const slots = draft.slots.map((slot) => ({ ...slot, carta: { ...slot.carta } }));
    if (existing >= 0) { const replaced = slots[target]!.carta; slots[target]!.carta = reference; slots[existing]!.carta = replaced; } else slots[target]!.carta = reference;
    commit({ ...draft, slots, bench: draft.bench.filter((ref) => refKey(ref) !== id), unrelated: draft.unrelated.filter((ref) => refKey(ref) !== id) }); setSelected(null);
  };
  const clearSlot = (index: number) => { const existing = draft.slots.find((slot) => slot.index === index)?.carta; if (!existing || (!existing.player_id && !existing.club_item_id)) return; commit({ ...draft, slots: draft.slots.map((slot) => slot.index === index ? { ...slot, carta: {} } : slot) }); };
  const changeFormation = (formation: string) => {
    const observed = formation === data.formacao && (data.titulares ?? []).length === 11;
    const positions = observed
      ? (data.titulares ?? []).slice().sort((a, b) => a.index - b.index).map((starter) => starter.position)
      : FORMATIONS[formation];
    if (!positions) return;
    commit({ ...draft, formation, formationSource: observed ? "confirmada" : "manual", slots: positions.map((posicao, index) => ({ ...draft.slots.find((slot) => slot.index === index), index, posicao, carta: { ...(draft.slots.find((slot) => slot.index === index)?.carta ?? {}) } })) });
  };
  const editSlot = (index: number, changes: Partial<Pick<SquadPlanSlot, "posicao" | "funcao" | "estilo_entrosamento">>) => {
    commit({ ...draft, formationSource: changes.posicao ? "manual" : draft.formationSource, slots: draft.slots.map((slot) => slot.index === index ? { ...slot, ...changes } : slot) });
  };
  const moveTo = (player: ClubPlayer, destination: "banco" | "nao_relacionados" | "disponiveis") => {
    const ref = cardRef(player); const id = refKey(ref); const next = { ...draft, bench: draft.bench.filter((entry) => refKey(entry) !== id), unrelated: draft.unrelated.filter((entry) => refKey(entry) !== id) };
    if (destination === "banco") next.bench = [...next.bench, ref]; if (destination === "nao_relacionados") next.unrelated = [...next.unrelated, ref]; commit(next);
  };
  const undo = () => { const previous = history.at(-1); if (!previous) return; setFuture((entries) => [draft, ...entries]); setHistory((entries) => entries.slice(0, -1)); setDraft(previous); };
  const redo = () => { const next = future[0]; if (!next) return; setHistory((entries) => [...entries, draft]); setFuture((entries) => entries.slice(1)); setDraft(next); };
  const openPlan = (view: SavedSquadPlanView) => { setDraft(fromPlan(view.plano)); setOpenedPlan(view.plano); setPlanName(view.plano.nome); setHistory([]); setFuture([]); setSelected(null); setFocusedSlot(null); setActionError(""); };
  const save = async (update = !!openedPlan) => {
    if (!planName.trim()) { setActionError("Dê um nome ao plano antes de salvar."); return; }
    setSaving(true); setActionError("");
    try { const saved = await saveSquadPlan(inputFromDraft(planName, draft, update ? openedPlan?.revisao : undefined), update ? openedPlan?.id : undefined); setOpenedPlan(saved.plano); setPlanName(saved.plano.nome); setActionMessage(update ? `Revisão ${saved.plano.revisao} salva.` : "Plano salvo."); await refreshPlans(); }
    catch (error) { setActionError(error instanceof Error ? error.message : "Não foi possível salvar o plano."); } finally { setSaving(false); }
  };
  const applyReference = async (id: string) => { setSaving(true); setActionError(""); try { const applied = await applySquadPlanReference(id); setOpenedPlan(applied.plano); setActionMessage(`“${applied.plano.nome}” agora é a referência ativa.`); await refreshPlans(); } catch (error) { setActionError(error instanceof Error ? error.message : "Não foi possível aplicar a referência."); } finally { setSaving(false); } };
  const removePlan = async (id: string) => { setSaving(true); setActionError(""); try { await deleteSquadPlan(id); if (openedPlan?.id === id) setOpenedPlan(null); setPlanPendingDelete(null); setActionMessage("Plano apagado."); await refreshPlans(); } catch (error) { setActionError(error instanceof Error ? error.message : "Não foi possível apagar o plano."); } finally { setSaving(false); } };
	const pitchLines = formationLines(draft.formation, draft.slots);
	const renderSlot = (slot: SquadPlanSlot) => {
		const card = (slot.carta.player_id || slot.carta.club_item_id) ? cardsByKey.get(refKey(slot.carta)) : undefined;
		const player = card?.player;
		const score = evaluationByIndex.get(slot.index);
		const out = !!player && !playsAt(player, slot.posicao);
		return <article data-slot-index={slot.index} className={`editor-slot${selectedPlayer ? " target" : ""}${focusedSlot === slot.index ? " focused" : ""}${out ? " out" : ""}${card?.origin !== "clube" ? " planned" : ""}${player ? " draggable" : ""}`} key={slot.index} draggable={!!player} onDragStart={(event) => { if (!player) { event.preventDefault(); return; } event.dataTransfer.setData("card-key", refKey(slot.carta)); event.dataTransfer.effectAllowed = "move"; }} onDragOver={(event) => event.preventDefault()} onDrop={(event) => { event.preventDefault(); const dropped = cardsByKey.get(event.dataTransfer.getData("card-key")); if (dropped) assign(slot.index, dropped); }}><button type="button" className="editor-slot-main" aria-label={`${slot.posicao}: ${player ? playerName(player) : "vaga livre"}`} onClick={() => selectedCard ? assign(slot.index, selectedCard) : setFocusedSlot(slot.index)} onKeyDown={(event) => { if (event.key === "Escape") setSelected(null); }}><span>{slot.posicao} · vaga {slot.index + 1}{card?.origin !== "clube" ? ` · alvo ${card?.origin}` : ""}</span>{player ? <><strong>{playerName(player)}</strong><small>{score?.nota_disponivel ? `${score.nota!.toFixed(1)} ${evaluationSourceLabel(data.avaliacao.fonte, true)}` : "nota indisponível"}{out ? " · fora da posição" : ""}</small></> : <em>vaga livre</em>}</button>{player ? <button type="button" className="editor-slot-clear" onClick={() => clearSlot(slot.index)} aria-label={`Retirar ${playerName(player)} da vaga ${slot.index + 1}`}>×</button> : null}</article>;
	};

  return <div className="wrap editor-page">
    <PageHeader
      eyebrow="elenco · planos locais"
      title="Editor de elenco"
      meta="Monte, simule e salve planos sem alterar sua conta EA. A referência marca a revisão ativa do editor."
      actions={<div className="editor-control-actions"><button type="button" className="btn ghost" onClick={undo} disabled={!history.length}>desfazer</button><button type="button" className="btn ghost" onClick={redo} disabled={!future.length}>refazer</button></div>}
    />
    <div className="editor-controlbar" aria-label="Controles do editor">
      <label className="editor-control-cell">Formação · {draft.formationSource}<select value={draft.formation} onChange={(event) => changeFormation(event.target.value)}>{data.formacao && !FORMATIONS[data.formacao] ? <option value={data.formacao}>{data.formacao} · observada</option> : null}{!FORMATIONS[draft.formation] && draft.formation !== data.formacao ? <option value={draft.formation}>{draft.formation} · manual</option> : null}{Object.keys(FORMATIONS).map((formation) => <option key={formation} value={formation}>{formation}{formation === data.formacao ? " · observada" : " · modelo manual"}</option>)}</select></label>
      <label className="editor-control-cell">Estilo do plano<select value={draft.gameStyle} onChange={(event) => commit({ ...draft, gameStyle: event.target.value })}><option value="">sem preferência</option><option value="meta competitivo">meta competitivo</option><option value="posse">posse</option><option value="contra-ataque">contra-ataque</option><option value="jogo pelas pontas">jogo pelas pontas</option></select></label>
      <div className="editor-control-cell editor-control-status">
        <span className={`editor-status-dot${evaluation?.status === "ok" ? "" : " pending"}`} aria-hidden="true" />
        <p className="editor-status" aria-live="polite">{selectedPlayer ? `${playerName(selectedPlayer)} selecionado. Escolha uma vaga com Enter ou toque.` : "Arraste uma carta ou selecione-a e escolha uma vaga. Fora de posição continua permitido e fica sinalizado."}</p>
        <span className="editor-recalc">{evaluating ? "recalculando" : evaluation?.status ?? "aguardando"}</span>
      </div>
    </div>
    {actionError ? <p className="editor-message error" role="alert">{actionError}</p> : null}{actionMessage ? <p className="editor-message success" role="status">{actionMessage}</p> : null}
    <div className="editor-workspace">
      <section className="panel editor-board" aria-labelledby="editor-xi-title"><div className="panel-head"><span id="editor-xi-title">Titulares <span className="panel-head-sub">XI simulado · {draft.slots.filter((slot) => slot.carta.player_id || slot.carta.club_item_id).length}/11</span></span><span className="panel-head-meta">rascunho salvo neste navegador</span></div><div className={`editor-pitch${pitchLines ? " formation" : ""}`} aria-label="Vagas titulares do plano">{pitchLines ? pitchLines.map((line, row) => <div className="editor-pitch-row" key={row}>{line.map(renderSlot)}</div>) : draft.slots.map(renderSlot)}</div></section>
      <aside className="panel editor-analysis" aria-label="Análise do plano"><span className="eyebrow">Análise desta revisão</span><h2>{evaluation?.media ? evaluation.media.toFixed(1) : "—"} <small>{evaluationSourceLabel(data.avaliacao.fonte)}</small></h2><p className="editor-revision">{evaluation?.revisao_plano ? `${evaluation.revisao_plano} · ${evaluation.avaliacao.estilo_jogo || "sem estilo do plano"}` : "aguardando revisão"}</p><dl><div><dt>Cobertura</dt><dd>{evaluation?.cobertura ?? 0}/11</dd></div><div><dt>Química observada</dt><dd>{data.quimica_referencia?.verificacao.status !== "sem_oraculo" ? `${data.quimica_referencia?.verificacao.observado ?? 0}/${data.quimica_referencia?.maximo ?? 33}` : "não sincronizada"}</dd></div><div><dt>Química simulada</dt><dd>{evaluation?.quimica ? `${evaluation.quimica.total}/${evaluation.quimica.maximo}` : "—"}</dd></div></dl>{slotInFocus ? <div className="editor-slot-settings"><span>Vaga {slotInFocus.index + 1}</span><label>Posição<select value={slotInFocus.posicao} onChange={(event) => editSlot(slotInFocus.index, { posicao: event.target.value as Position })}>{(["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"] as Position[]).map((position) => <option key={position}>{position}</option>)}</select></label><label>Função<input list="funcoes-da-vaga" value={slotInFocus.funcao ?? ""} onChange={(event) => editSlot(slotInFocus.index, { funcao: event.target.value })} placeholder="ex.: Holding" /></label>{rolesInFocus.length ? <datalist id="funcoes-da-vaga">{rolesInFocus.map((role) => <option key={`${role.posicao}-${role.nome}`} value={role.nome} />)}</datalist> : null}<label>Estilo de entrosamento<select value={slotInFocus.estilo_entrosamento ?? ""} onChange={(event) => editSlot(slotInFocus.index, { estilo_entrosamento: event.target.value })}><option value="">não definido</option><option>Básico</option><option>Caçador</option><option>Sombra</option><option>Motor</option><option>Catalisador</option><option>Âncora</option><option>Falcão</option></select></label><p>Função entra na nota quando o catálogo confirma a familiaridade. O estilo é aplicado assim que a tabela do ciclo for confirmada; enquanto isso a limitação fica visível na nota.</p>{evaluationInFocus?.avaliacao ? <div className="editor-score-details"><strong>Composição da nota</strong><ul>{(evaluationInFocus.avaliacao.componentes ?? []).map((component) => <li key={component.chave}><span>{component.rotulo}</span><b>{component.valor >= 0 ? "+" : ""}{component.valor.toFixed(1)}</b></li>)}</ul>{evaluationInFocus.avaliacao.pontos_fortes?.length ? <p><em>Pontos fortes:</em> {evaluationInFocus.avaliacao.pontos_fortes.join(", ")}</p> : null}{evaluationInFocus.avaliacao.limitacoes?.length ? <p><em>Limitações:</em> {evaluationInFocus.avaliacao.limitacoes.join(", ")}</p> : null}{evaluationInFocus.avaliacao.dados_ausentes?.length ? <p><em>Dados ausentes:</em> {evaluationInFocus.avaliacao.dados_ausentes.join(", ")}</p> : null}</div> : null}</div> : <p className="editor-slot-help">Selecione uma vaga para ajustar posição, função e estilo de entrosamento.</p>}{evaluation?.elo_mais_fraco?.carta ? <div className="editor-weak"><span>Menor nota na vaga</span><strong>{playerName(evaluation.elo_mais_fraco.carta)}</strong><p>{evaluation.elo_mais_fraco.posicao} · {evaluation.elo_mais_fraco.nota?.toFixed(1)}</p></div> : null}{evaluationError ? <p className="editor-message error">{evaluationError}</p> : null}{(evaluation?.avisos ?? []).map((warning) => <p className="editor-warning" key={warning}>{warning}</p>)}</aside>
    </div>
    <section className="panel editor-collection" aria-labelledby="editor-club-title"><div className="panel-head editor-collection-head"><span id="editor-club-title">Clube completo e alvos <span className="panel-head-sub">banco, não relacionados e simulações</span></span><label className="editor-search"><span>Pesquisar cartas</span><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Nome, posição ou evolução" /></label></div><div className="tab-strip flush" role="tablist" aria-label="Categoria de cartas"><button type="button" role="tab" aria-selected={pool === "disponiveis"} className={`tab-strip-cell${pool === "disponiveis" ? " active" : ""}`} onClick={() => setPool("disponiveis")}><span className="tab-strip-label">disponíveis · {availableCount}</span></button><button type="button" role="tab" aria-selected={pool === "banco"} className={`tab-strip-cell${pool === "banco" ? " active" : ""}`} onClick={() => setPool("banco")}><span className="tab-strip-label">banco · {draft.bench.length}</span></button><button type="button" role="tab" aria-selected={pool === "nao_relacionados"} className={`tab-strip-cell${pool === "nao_relacionados" ? " active" : ""}`} onClick={() => setPool("nao_relacionados")}><span className="tab-strip-label">não relacionados · {draft.unrelated.length}</span></button><button type="button" role="tab" aria-selected={pool === "alvos"} className={`tab-strip-cell${pool === "alvos" ? " active" : ""}`} onClick={() => setPool("alvos")}><span className="tab-strip-label">alvos · {targetCards.length}</span></button></div><div className="editor-player-list" role="tabpanel">{sourceCards.length === 0 ? <p className="editor-empty">Nenhuma carta nesta categoria com essa busca.</p> : sourceCards.slice(0, visibleCards).map(({ player, reference, origin, cost, description }) => {
		const id = refKey(reference); return <article key={id} className={`editor-player${selected === id ? " selected" : ""}${origin !== "clube" ? " planned" : ""}`} draggable onDragStart={(event) => event.dataTransfer.setData("card-key", id)}><button type="button" className="editor-player-select" onClick={() => setSelected(selected === id ? null : id)}><span>{player.position}{origin !== "clube" ? ` · alvo ${origin}` : ""}</span><strong>{playerName(player)}</strong><small>{player.rating} OVR · {player.gg_rating ? `${player.gg_rating.toFixed(1)} GG` : "nota indisponível"}{cost ? ` · ${formatCoins(cost)}` : ""}</small>{description ? <em>{description}</em> : null}</button>{origin === "clube" ? <div className="editor-player-actions"><button type="button" onClick={() => moveTo(player, "banco")}>banco</button><button type="button" onClick={() => moveTo(player, "nao_relacionados")}>não relacionar</button>{pool !== "disponiveis" ? <button type="button" onClick={() => moveTo(player, "disponiveis")}>liberar</button> : null}</div> : <div className="editor-target-note">simulação · não pertence ao clube</div>}</article>;
    })}</div>{sourceCards.length > visibleCards ? <div className="editor-more"><span>Mostrando {visibleCards} de {sourceCards.length} cartas.</span><button type="button" className="btn ghost" onClick={() => setVisibleCards((count) => count + 60)}>mostrar mais</button></div> : null}</section>
    <section className="panel editor-plans" aria-labelledby="editor-plans-title"><div className="panel-head editor-plans-head"><span id="editor-plans-title">Planos persistidos <span className="panel-head-sub">salvar cria uma revisão; aplicar referência marca a revisão ativa e não muda seu elenco no jogo</span></span><div className="editor-save"><input value={planName} onChange={(event) => setPlanName(event.target.value)} maxLength={80} aria-label="Nome do plano" placeholder="Nome do plano" /><button type="button" className="btn primary" disabled={saving} onClick={() => save(!!openedPlan)}>{openedPlan ? "atualizar revisão" : "salvar plano"}</button>{openedPlan ? <button type="button" className="btn ghost" disabled={saving} onClick={() => { setOpenedPlan(null); setPlanName(`Plano ${plans.length + 1}`); }}>salvar como novo</button> : null}</div></div><div className="panel-body">{plansError ? <p className="editor-message error">{plansError}</p> : null}{planPendingDelete ? <div className="editor-delete-confirm" role="alert"><span>Apagar “{planPendingDelete.nome}”? Esta ação não pode ser desfeita.</span><div><button type="button" className="btn ghost" onClick={() => setPlanPendingDelete(null)} disabled={saving}>cancelar</button><button type="button" className="btn ghost danger" onClick={() => removePlan(planPendingDelete.id)} disabled={saving}>confirmar apagar</button></div></div> : null}<div className="editor-saved-list">{plans.length === 0 ? <p className="editor-empty">Nenhum plano salvo ainda.</p> : plans.map((view) => <article className={`editor-saved${view.plano.referencia ? " reference" : ""}`} key={view.plano.id}><div><strong>{view.plano.nome}</strong><span>{view.plano.formacao} · revisão {view.plano.revisao}{view.plano.referencia ? " · referência ativa" : ""}</span>{view.pendencias?.length ? <details className="editor-reconciliation"><summary>{view.pendencias.length} pendência{view.pendencias.length === 1 ? "" : "s"} de reconciliação</summary><p>Abra o plano, substitua ou retire cada carta indicada e salve uma nova revisão.</p><ul>{view.pendencias.map((pending) => <li key={pending}>{pending}</li>)}</ul></details> : null}</div><div><button type="button" className="btn ghost" onClick={() => openPlan(view)}>abrir</button><button type="button" className="btn ghost" disabled={saving || view.plano.referencia} onClick={() => applyReference(view.plano.id)}>aplicar referência</button><button type="button" className="btn ghost danger" disabled={saving} onClick={() => setPlanPendingDelete(view.plano)}>apagar</button></div></article>)}</div></div></section>
  </div>;
}
