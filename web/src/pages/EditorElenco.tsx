import { useEffect, useMemo, useRef, useState } from "react";
import {
  applySquadPlanReference,
  deleteSquadPlan,
  evaluateSquadEditor,
  fetchFormations,
  fetchSavedSquadPlans,
  fetchSquadEditor,
  saveSquadPlan,
} from "../api";
import { asyncGate } from "../components/asyncGate";
import { CardArt, FutCardFace, FutLegend, FutPitch, formationLayout, ratingScale, spotStyle, type FutTone } from "../components/FutPitch";
import PageHeader from "../components/PageHeader";
import { evaluationSourceLabel, formatCoins } from "../format";
import { useData } from "../useData";
import type {
  CardEvaluationComponent,
  ClubPlayer,
  Formation,
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

const POSITIONS: Position[] = ["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"];
const CHEMISTRY_STYLES = ["Básico", "Caçador", "Sombra", "Motor", "Catalisador", "Âncora", "Falcão"];
const GAME_STYLES = ["meta competitivo", "posse", "contra-ataque", "jogo pelas pontas"];
const STATUS_LABELS: Record<SquadEditorEvaluation["status"], string> = { ok: "recalculado", incompleto: "incompleto", indisponivel: "indisponível" };
const ORIGIN_LABELS: Record<EditorCard["origin"], string> = { clube: "clube", mercado: "mercado", evolucao: "evolução" };

function cardRef(player: ClubPlayer): SquadCardReference {
  return { club_item_id: player.club_item_id || undefined, player_id: player.id };
}

function refKey(ref: SquadCardReference): string {
  const origin = ref.origem || "clube";
  const evolution = ref.evolucao_id ? `${ref.evolucao_id}:` : "";
  return ref.club_item_id ? `${origin}:${evolution}item:${ref.club_item_id}` : `${origin}:${evolution}player:${ref.player_id ?? 0}`;
}

function hasCard(ref: SquadCardReference): boolean { return !!(ref.player_id || ref.club_item_id); }
function playerName(player: ClubPlayer): string { return player.common_name || player.name; }
function playsAt(player: ClubPlayer, position: Position): boolean { return player.position === position || (player.alt_positions ?? []).includes(position); }

function slotsFromStarters(starters: StarterCard[], formation: string, catalog: Formation[]): SquadPlanSlot[] {
  const current = starters.slice().sort((a, b) => a.index - b.index);
  if (current.length === 11) return current.map((starter) => ({ index: starter.index, posicao: starter.position, carta: cardRef(starter.player) }));
  const template = catalog.find((entry) => entry.nome === formation) ?? catalog.find((entry) => entry.nome === "4-3-3");
  return (template?.vagas ?? []).map((vaga) => ({ index: vaga.index, posicao: vaga.posicao, carta: {} }));
}

// Encaixe global das cartas: uma escolha gananciosa para a primeira vaga
// pode roubar a única vaga natural de outra carta. O DP maximiza primeiro
// posições principais, depois alternativas, e só então a vaga anterior.
function reencaixarVagas(slots: SquadPlanSlot[], target: Formation, cards: Map<string, EditorCard>, roles: { nome: string; posicao: Position }[]): SquadPlanSlot[] {
  const occupied = slots.filter((slot) => hasCard(slot.carta)).sort((a, b) => a.index - b.index);
  const positions = target.vagas.slice().sort((a, b) => a.index - b.index);
  const memo = new Map<string, { score: number; indexes: number[] }>();
  const solve = (cardIndex: number, used: number): { score: number; indexes: number[] } => {
    if (cardIndex === occupied.length) return { score: 0, indexes: [] };
    const key = `${cardIndex}:${used}`;
    const cached = memo.get(key);
    if (cached) return cached;
    const old = occupied[cardIndex]!;
    const player = cards.get(refKey(old.carta))?.player;
    let best = { score: Number.NEGATIVE_INFINITY, indexes: [] as number[] };
    for (const next of positions) {
      const bit = 1 << next.index;
      if (used & bit) continue;
      const exact = player?.position === next.posicao ? 1 : 0;
      const alternative = !exact && player?.alt_positions?.includes(next.posicao) ? 1 : 0;
      const score = exact * 1_000_000 + alternative * 10_000 + (old.index === next.index ? 100 : 0) - Math.abs(old.index - next.index);
      const tail = solve(cardIndex + 1, used | bit);
      if (score + tail.score > best.score) best = { score: score + tail.score, indexes: [next.index, ...tail.indexes] };
    }
    memo.set(key, best);
    return best;
  };
  const destinations = solve(0, 0).indexes;
  const assigned = new Map(destinations.map((index, entry) => [index, occupied[entry]!]));
  return positions.map(({ index, posicao }) => {
    const old = assigned.get(index);
    if (!old) return { index, posicao, carta: {} };
    return {
      index, posicao, carta: { ...old.carta },
      estilo_entrosamento: old.estilo_entrosamento,
      funcao: old.funcao && roles.some((role) => role.nome === old.funcao && role.posicao === posicao) ? old.funcao : undefined,
    };
  });
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
function referencesIn(draft: Draft): Set<string> { return new Set(draft.slots.filter((slot) => hasCard(slot.carta)).map((slot) => refKey(slot.carta))); }

function formatComponent(component: CardEvaluationComponent, first: boolean): string {
  // O primeiro componente é a base da nota (subatributos, GG da carta); os
  // demais são ajustes em cima dela, por isso só eles levam sinal.
  if (first) return component.valor.toFixed(1);
  return `${component.valor > 0 ? "+" : component.valor < 0 ? "−" : ""}${Math.abs(component.valor).toFixed(1)}`;
}

export default function EditorElenco() {
  const editorData = useData(fetchSquadEditor, []);
  const formationsData = useData(fetchFormations, []);
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
  const formations = formationsData.data?.value ?? [];
  const key = data ? draftKey(data.clube, data.avaliacao?.ciclo) : "";

  const refreshPlans = async () => {
    try { const response = await fetchSavedSquadPlans(); setPlans(response.value ?? []); setPlansError(""); }
    catch (error) { setPlansError(error instanceof Error ? error.message : "Não foi possível carregar os planos salvos."); }
  };

  useEffect(() => {
    if (!data || !formationsData.data || initializedFor.current === key) return;
    initializedFor.current = key;
    const hasObservedXI = (data.titulares ?? []).length === 11;
    const formation = data.formacao || (hasObservedXI ? "" : "4-3-3");
    const initial: Draft = { formation, formationSource: hasObservedXI && !!data.formacao ? "confirmada" : "manual", gameStyle: "", slots: slotsFromStarters(data.titulares ?? [], formation, formationsData.data.value), bench: [], unrelated: [] };
    setDraft(readDraft(key) ?? initial); setHistory([]); setFuture([]); setSelected(null); setFocusedSlot(null); void refreshPlans();
  }, [data, key, formationsData.data]);

  useEffect(() => { if (draft && key) localStorage.setItem(key, JSON.stringify(draft)); }, [draft, key]);
  useEffect(() => { setVisibleCards(60); }, [pool, search]);
  const evaluationKey = useMemo(() => draft ? JSON.stringify({ formation: draft.formation, gameStyle: draft.gameStyle, slots: draft.slots }) : "", [draft]);
  useEffect(() => {
    if (!draft || !evaluationKey || !draft.formation) { setEvaluation(null); return; }
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

  const gate = asyncGate(editorData.loading || formationsData.loading, editorData.error ?? formationsData.error, !!data && !!formationsData.data, () => { editorData.refetch(); formationsData.refetch(); });
  if (gate) return gate;
  if (!data || !draft || !formationsData.data) return null;

  const cards = data.cartas ?? [];
  const ownedCards: EditorCard[] = cards.map(({ player }) => ({ player, reference: cardRef(player), origin: "clube" }));
  const targetCards: EditorCard[] = (data.alvos ?? []).map((target) => ({ player: target.player, reference: target.referencia, origin: target.tipo, cost: target.custo, description: target.descricao }));
  const cardsByKey = new Map([...ownedCards, ...targetCards].map((card) => [refKey(card.reference), card]));
  const selectedCard = selected ? cardsByKey.get(selected) : undefined;
  const selectedPlayer = selectedCard?.player;
  const used = referencesIn(draft);
  const bench = new Set(draft.bench.map(refKey));
  const unrelated = new Set(draft.unrelated.map(refKey));
  const availableCount = ownedCards.filter(({ reference }) => { const ref = refKey(reference); return !used.has(ref) && !bench.has(ref) && !unrelated.has(ref); }).length;
  const targetCount = targetCards.filter(({ reference }) => !used.has(refKey(reference))).length;
  const sourceCards = (pool === "alvos" ? targetCards : ownedCards).filter(({ reference }) => {
    const ref = refKey(reference);
    if (used.has(ref)) return false;
    if (pool === "alvos") return true;
    if (pool === "banco") return bench.has(ref);
    if (pool === "nao_relacionados") return unrelated.has(ref);
    return !bench.has(ref) && !unrelated.has(ref);
  }).filter(({ player, description }) => `${playerName(player)} ${player.position} ${player.version ?? ""} ${description ?? ""}`.toLocaleLowerCase().includes(search.toLocaleLowerCase()));
  const evaluationByIndex = new Map((evaluation?.vagas ?? []).map((slot) => [slot.index, slot]));
  const chemistryByIndex = new Map((evaluation?.quimica?.jogadores ?? []).map((player) => [player.index, player]));
  const weakestIndex = evaluation?.elo_mais_fraco?.carta ? evaluation.elo_mais_fraco.index : null;
  const slotInFocus = focusedSlot === null ? null : draft.slots.find((slot) => slot.index === focusedSlot) ?? null;
  const cardInFocus = slotInFocus && hasCard(slotInFocus.carta) ? cardsByKey.get(refKey(slotInFocus.carta)) : undefined;
  const evaluationInFocus = slotInFocus ? evaluationByIndex.get(slotInFocus.index) : undefined;
  const rolesInFocus = slotInFocus ? (data.funcoes ?? []).filter((role) => role.posicao === slotInFocus.posicao) : [];
  const sourceLabel = evaluationSourceLabel(data.avaliacao.fonte, true);

  const commit = (next: Draft) => { setHistory((entries) => [...entries.slice(-29), draft]); setFuture([]); setDraft(next); setActionError(""); setActionMessage(""); };
  const assign = (index: number, card: EditorCard) => {
    const reference = card.reference; const id = refKey(reference); const target = draft.slots.findIndex((slot) => slot.index === index); if (target < 0) return;
    const existing = draft.slots.findIndex((slot) => refKey(slot.carta) === id); const slots = draft.slots.map((slot) => ({ ...slot, carta: { ...slot.carta } }));
    if (existing >= 0) { const replaced = slots[target]!.carta; slots[target]!.carta = reference; slots[existing]!.carta = replaced; } else slots[target]!.carta = reference;
    commit({ ...draft, slots, bench: draft.bench.filter((ref) => refKey(ref) !== id), unrelated: draft.unrelated.filter((ref) => refKey(ref) !== id) }); setSelected(null); setFocusedSlot(index);
  };
  const clearSlot = (index: number) => { const existing = draft.slots.find((slot) => slot.index === index)?.carta; if (!existing || !hasCard(existing)) return; commit({ ...draft, slots: draft.slots.map((slot) => slot.index === index ? { ...slot, carta: {} } : slot) }); };
  const changeFormation = (formation: string) => {
    const template = formations.find((entry) => entry.nome === formation);
    if (!template) return;
    const observed = formation === data.formacao && (data.titulares ?? []).length === 11;
    const observedPositions = new Map((data.titulares ?? []).map((starter) => [starter.index, starter.position]));
    const target = observed ? { ...template, vagas: template.vagas.map((vaga) => ({ ...vaga, posicao: observedPositions.get(vaga.index) ?? vaga.posicao })) } : template;
    const slots = reencaixarVagas(draft.slots, target, cardsByKey, data.funcoes ?? []);
    commit({ ...draft, formation, formationSource: observed ? "confirmada" : "manual", slots });
    const outOfPosition = slots.filter((slot) => {
      const card = cardsByKey.get(refKey(slot.carta));
      return card && !playsAt(card.player, slot.posicao);
    }).length;
    if (outOfPosition) setActionMessage(`${outOfPosition} carta${outOfPosition > 1 ? "s" : ""} mantida${outOfPosition > 1 ? "s" : ""} no XI fora de posição.`);
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
    if (!draft.formation) { setActionError("Escolha uma formação antes de salvar o plano."); return; }
    setSaving(true); setActionError("");
    try { const saved = await saveSquadPlan(inputFromDraft(planName, draft, update ? openedPlan?.revisao : undefined), update ? openedPlan?.id : undefined); setOpenedPlan(saved.plano); setPlanName(saved.plano.nome); setActionMessage(update ? `Revisão ${saved.plano.revisao} salva.` : "Plano salvo."); await refreshPlans(); }
    catch (error) { setActionError(error instanceof Error ? error.message : "Não foi possível salvar o plano."); } finally { setSaving(false); }
  };
  const applyReference = async (id: string) => { setSaving(true); setActionError(""); try { const applied = await applySquadPlanReference(id); setOpenedPlan(applied.plano); setActionMessage(`“${applied.plano.nome}” agora é a referência ativa.`); await refreshPlans(); } catch (error) { setActionError(error instanceof Error ? error.message : "Não foi possível aplicar a referência."); } finally { setSaving(false); } };
  const removePlan = async (id: string) => { setSaving(true); setActionError(""); try { await deleteSquadPlan(id); if (openedPlan?.id === id) setOpenedPlan(null); setPlanPendingDelete(null); setActionMessage("Plano apagado."); await refreshPlans(); } catch (error) { setActionError(error instanceof Error ? error.message : "Não foi possível apagar o plano."); } finally { setSaving(false); } };

  const layout = formationLayout(draft.formation, draft.slots, formations);
  const spots = layout?.spots;
  const orderedSlots = layout?.ordered ?? draft.slots.slice().sort((a, b) => a.index - b.index);
  const filledCount = draft.slots.filter((slot) => hasCard(slot.carta)).length;

  // Tom da carta no campo: alert = fora de posição, coin = alvo que ainda não
  // é seu (dinheiro ou evolução a gastar), turf = carta do clube na posição.
  const slotTone = (slot: SquadPlanSlot): { card?: EditorCard; tone: FutTone; out: boolean } => {
    const card = hasCard(slot.carta) ? cardsByKey.get(refKey(slot.carta)) : undefined;
    const out = !!card && !playsAt(card.player, slot.posicao);
    return { card, out, tone: out ? "alert" : card && card.origin !== "clube" ? "coin" : "turf" };
  };
  const slotScore = (index: number): number | undefined => { const score = evaluationByIndex.get(index); return score?.nota_disponivel ? score.nota : undefined; };

  const renderSlot = (slot: SquadPlanSlot) => {
    const { card, tone, out } = slotTone(slot);
    const player = card?.player;
    const score = slotScore(slot.index);
    const chem = chemistryByIndex.get(slot.index);
    const pips = chem ? (chem.fora_de_posicao ? 0 : chem.pontos) : undefined;
    const spot = spots?.get(slot.index);
    const weakest = !!player && !out && weakestIndex === slot.index;
    const classes = ["editor-slot", "fut-slot", player ? "filled draggable" : "empty", selectedCard ? "target" : "", focusedSlot === slot.index ? "focused" : ""].filter(Boolean).join(" ");
    const label = player ? `${slot.posicao}: ${playerName(player)} · ${score !== undefined ? `${sourceLabel} ${score.toFixed(1)}` : "nota indisponível"}${out ? " · fora de posição" : ""}${card?.origin !== "clube" ? ` · alvo ${ORIGIN_LABELS[card!.origin]}` : ""}` : `${slot.posicao}: vaga livre`;
    return <article
      key={slot.index}
      data-slot-index={slot.index}
      className={classes}
      style={spotStyle(spot)}
      draggable={!!player}
      onDragStart={(event) => { if (!player) { event.preventDefault(); return; } event.dataTransfer.setData("card-key", refKey(slot.carta)); event.dataTransfer.effectAllowed = "move"; }}
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => { event.preventDefault(); const dropped = cardsByKey.get(event.dataTransfer.getData("card-key")); if (dropped) assign(slot.index, dropped); }}
    >
      <button type="button" className={`editor-slot-main${player ? ` fut-card tone-${tone}` : ""}`} aria-label={label} onClick={() => selectedCard ? assign(slot.index, selectedCard) : setFocusedSlot(slot.index)} onKeyDown={(event) => { if (event.key === "Escape") setSelected(null); }}>
        {player ? <FutCardFace
          player={player}
          position={slot.posicao}
          score={score}
          pips={pips}
          scoreTone={out ? "alert" : weakest ? "cost" : ""}
          badge={out ? "fora de posição" : card?.origin !== "clube" ? `alvo ${ORIGIN_LABELS[card!.origin]}` : null}
        /> : <>
          <span className="editor-slot-pos">{slot.posicao}</span>
          <span className="editor-slot-free">vaga livre</span>
        </>}
      </button>
      {player ? <button type="button" className="editor-slot-clear" title="Retirar da vaga" onClick={() => clearSlot(slot.index)} aria-label={`Retirar ${playerName(player)} da vaga ${slot.index + 1}`}>×</button> : null}
    </article>;
  };

  // Delta "vs jogo" só com os dois lados completos: a média da avaliação
  // exclui carta sem nota, e media_atual só vem com as 11 vagas avaliadas.
  const media = evaluation?.media;
  const complete = evaluation?.status === "ok" && evaluation.cobertura === 11;
  const delta = complete && media !== undefined && data.media_atual !== undefined ? media - data.media_atual : undefined;
  const referenceChemistry = data.quimica_referencia && data.quimica_referencia.verificacao.status !== "sem_oraculo" ? data.quimica_referencia.verificacao.observado : undefined;

  const scores = orderedSlots.map((slot) => slotScore(slot.index)).filter((score): score is number => score !== undefined);
  const scale = ratingScale(media !== undefined ? [...scores, media] : scores).pct;

  const selectingText = selectedPlayer ? `${playerName(selectedPlayer)} selecionado — clique numa vaga do campo (ou Enter). Fora de posição é permitido e fica sinalizado.` : "Arraste ou selecione uma carta abaixo e escolha uma vaga. Clique numa vaga sem seleção para ajustá-la.";
  const statusLabel = evaluating ? "recalculando" : evaluation ? STATUS_LABELS[evaluation.status] : "aguardando";
  const dotTone = selectedPlayer ? "selecting" : evaluation?.status === "ok" ? "ok" : "";
  const pools: { key: Pool; label: string; count: number }[] = [
    { key: "disponiveis", label: "disponíveis", count: availableCount },
    { key: "banco", label: "banco", count: draft.bench.length },
    { key: "nao_relacionados", label: "não relacionados", count: draft.unrelated.length },
    { key: "alvos", label: "alvos", count: targetCount },
  ];
  const focusTitle = slotInFocus ? `${slotInFocus.posicao} · vaga ${slotInFocus.index + 1} · ${cardInFocus ? playerName(cardInFocus.player) : "livre"}` : "nenhuma selecionada";
  const focusEvaluation = evaluationInFocus?.avaliacao;

  return <div className="wrap editor-page">
    <PageHeader
      eyebrow="Elenco / Planos locais"
      title="Editor de elenco"
      meta="Monte, simule e salve planos sem alterar sua conta EA."
      actions={<><button type="button" className="btn ghost" onClick={undo} disabled={!history.length}>desfazer</button><button type="button" className="btn ghost" onClick={redo} disabled={!future.length}>refazer</button></>}
    />

    <div className="editor-controlbar" aria-label="Controles do editor">
      <label className="editor-control-cell">Formação · {draft.formationSource}<select value={draft.formation} onChange={(event) => changeFormation(event.target.value)}>{!draft.formation ? <option value="">formação desconhecida · escolha uma</option> : null}{data.formacao && !formations.some((entry) => entry.nome === data.formacao) ? <option value={data.formacao}>{data.formacao} · observada (sem desenho)</option> : null}{draft.formation && !formations.some((entry) => entry.nome === draft.formation) && draft.formation !== data.formacao ? <option value={draft.formation}>{draft.formation} · plano salvo (sem desenho)</option> : null}{formations.map((entry) => <option key={entry.nome} value={entry.nome}>{entry.nome}{entry.nome === data.formacao ? " · observada" : ""}</option>)}</select></label>
      <label className="editor-control-cell">Estilo do plano<select value={draft.gameStyle} onChange={(event) => commit({ ...draft, gameStyle: event.target.value })}><option value="">sem preferência</option>{GAME_STYLES.map((style) => <option key={style} value={style}>{style}</option>)}</select></label>
      <div className="editor-control-cell editor-control-status">
        <span className={`editor-status-dot ${dotTone}`} aria-hidden="true" />
        <p className="editor-status" aria-live="polite">{selectingText}</p>
        <span className={`editor-recalc${evaluating ? " busy" : ""}`}>{statusLabel}</span>
      </div>
    </div>
    {actionError ? <p className="editor-message error" role="alert">{actionError}</p> : null}
    {actionMessage ? <p className="editor-message success" role="status">{actionMessage}</p> : null}

    <div className="editor-workspace">
      <section className="panel editor-board" aria-labelledby="editor-xi-title">
        <div className="panel-head">
          <span id="editor-xi-title">Titulares <span className="panel-head-sub">/ XI simulado · {filledCount}/11</span></span>
          <span className="panel-head-meta">rascunho salvo neste navegador</span>
        </div>
        {!layout && <p className="hint">Formação desconhecida ou sem 11 vagas; ajuste o plano na grade abaixo.</p>}
        {spots ? <FutPitch formation={draft.formation} lines={layout!.lines} dense={layout!.dense} className="editor-pitch" label="Vagas titulares do plano">
          {orderedSlots.map(renderSlot)}
        </FutPitch> : <div className="editor-pitch grid" aria-label="Vagas titulares do plano">{orderedSlots.map(renderSlot)}</div>}
        <FutLegend items={[["turf", "na posição"], ["alert", "fora de posição (sem entrosamento)"], ["coin", "alvo de mercado / evolução"]]} />
      </section>

      <aside className="editor-side" aria-label="Análise do plano">
        <div className="panel editor-summary">
          <div className="editor-stats">
            <div className="editor-stat wide">
              <span className="editor-stat-label">Média do plano · {sourceLabel}</span>
              <strong className="editor-avg">{media !== undefined ? media.toFixed(1) : "—"}</strong>{" "}
              {delta !== undefined ? <span className={`editor-delta${delta > 0.05 ? " up" : delta < -0.05 ? " down" : ""}`}>{delta >= 0 ? "+" : "−"}{Math.abs(delta).toFixed(1)} vs jogo</span> : null}
            </div>
            <div className="editor-stat narrow">
              <span className="editor-stat-label">Cobertura</span>
              <strong>{evaluation?.cobertura ?? 0}/11</strong>
            </div>
            <div className="editor-stat">
              <span className="editor-stat-label">Química</span>
              <strong>{evaluation?.quimica ? `${evaluation.quimica.total}/${evaluation.quimica.maximo}` : "—"}</strong>{" "}
              {referenceChemistry !== undefined ? <small>jogo {referenceChemistry}</small> : null}
            </div>
          </div>
          <div className="editor-map">
            <div className="editor-map-head"><span>nota por vaga</span><span>linha = média</span></div>
            {orderedSlots.map((slot) => {
              const { card, out } = slotTone(slot);
              const score = slotScore(slot.index);
              const barTone = out ? "alert" : weakestIndex === slot.index ? "cost" : "turf";
              return <button type="button" key={slot.index} className={`editor-map-row${focusedSlot === slot.index ? " focused" : ""}`} onClick={() => setFocusedSlot(slot.index)} aria-label={`Vaga ${slot.index + 1}, ${slot.posicao}: ${score !== undefined ? score.toFixed(1) : card ? "nota indisponível" : "livre"}`}>
                <span className="editor-map-pos">{slot.posicao}</span>
                <span className="editor-map-track">
                  {score !== undefined ? <span className={`editor-map-bar ${barTone}`} style={{ width: `${Math.max(3, scale(score))}%` }} /> : null}
                  {media !== undefined ? <span className="editor-map-mark" style={{ left: `${scale(media)}%` }} /> : null}
                </span>
                <span className="editor-map-val">{score !== undefined ? score.toFixed(1) : "—"}</span>
              </button>;
            })}
          </div>
          {evaluation?.elo_mais_fraco?.carta ? <div className="editor-weak">
            <span>menor nota na vaga</span>
            <strong>{playerName(evaluation.elo_mais_fraco.carta)} · {evaluation.elo_mais_fraco.posicao} · {evaluation.elo_mais_fraco.nota?.toFixed(1)}</strong>
          </div> : null}
          {evaluationError || (evaluation?.avisos ?? []).length ? <div className="editor-notes">
            {evaluationError ? <p className="error">{evaluationError}</p> : null}
            {(evaluation?.avisos ?? []).map((warning) => <p key={warning}>{warning}</p>)}
          </div> : null}
        </div>

        <div className="panel editor-focus">
          <div className="panel-head"><span>Vaga <span className="panel-head-sub">/ {focusTitle}</span></span></div>
          {slotInFocus ? <>
            <div className="editor-focus-fields">
              <label className="editor-field">Posição<select value={slotInFocus.posicao} onChange={(event) => editSlot(slotInFocus.index, { posicao: event.target.value as Position })}>{POSITIONS.map((position) => <option key={position}>{position}</option>)}</select></label>
              <label className="editor-field">Função<input list="funcoes-da-vaga" value={slotInFocus.funcao ?? ""} onChange={(event) => editSlot(slotInFocus.index, { funcao: event.target.value })} placeholder="ex.: Holding" /></label>
              {rolesInFocus.length ? <datalist id="funcoes-da-vaga">{rolesInFocus.map((role) => <option key={`${role.posicao}-${role.nome}`} value={role.nome} />)}</datalist> : null}
              <label className="editor-field">Entrosamento<select value={slotInFocus.estilo_entrosamento ?? ""} onChange={(event) => editSlot(slotInFocus.index, { estilo_entrosamento: event.target.value })}><option value="">não definido</option>{CHEMISTRY_STYLES.map((style) => <option key={style}>{style}</option>)}</select></label>
            </div>
            {cardInFocus ? <div className="editor-comp">
              <span className="editor-comp-title">composição da nota</span>
              {(focusEvaluation?.componentes ?? []).map((component, index) => <div className="editor-comp-row" key={component.chave}>
                <span>{component.rotulo}</span>
                <b className={index > 0 && component.valor < 0 ? "neg" : index > 0 && component.valor === 0 ? "zero" : ""}>{formatComponent(component, index === 0)}</b>
              </div>)}
              <div className="editor-comp-row total"><span>nota na vaga</span><b>{evaluationInFocus?.nota_disponivel && evaluationInFocus.nota !== undefined ? evaluationInFocus.nota.toFixed(1) : "—"}</b></div>
              {focusEvaluation && !focusEvaluation.disponivel && focusEvaluation.motivo ? <p>{focusEvaluation.motivo}</p> : null}
              {focusEvaluation?.pontos_fortes?.length ? <p><em>Pontos fortes:</em> {focusEvaluation.pontos_fortes.join(", ")}</p> : null}
              {focusEvaluation?.limitacoes?.length ? <p><em>Limitações:</em> {focusEvaluation.limitacoes.join(", ")}</p> : null}
              {focusEvaluation?.dados_ausentes?.length ? <p><em>Dados ausentes:</em> {focusEvaluation.dados_ausentes.join(", ")}</p> : null}
            </div> : null}
          </> : <p className="editor-focus-help">Clique numa vaga do campo para ajustar posição, função e estilo de entrosamento.</p>}
        </div>
      </aside>
    </div>

    <section className="panel editor-collection" aria-labelledby="editor-club-title">
      <div className="panel-head editor-collection-head">
        <span id="editor-club-title">Clube e alvos <span className="panel-head-sub">/ selecione uma carta e clique numa vaga</span></span>
        <input className="editor-search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Nome, posição ou evolução" aria-label="Pesquisar cartas" />
      </div>
      <div className="editor-pool-tabs" role="tablist" aria-label="Categoria de cartas">
        {pools.map((entry) => <button key={entry.key} type="button" role="tab" aria-selected={pool === entry.key} className={pool === entry.key ? "active" : ""} onClick={() => setPool(entry.key)}>{entry.label} <span>{entry.count}</span></button>)}
      </div>
      {sourceCards.length === 0 ? <p className="editor-empty">Nenhuma carta nesta categoria com essa busca.</p> : <div className="editor-player-list" role="tabpanel">
        {sourceCards.slice(0, visibleCards).map(({ player, reference, origin, cost, description }) => {
          const id = refKey(reference);
          const inBench = bench.has(id);
          const inUnrelated = unrelated.has(id);
          const targetLine = [ORIGIN_LABELS[origin], description, cost ? formatCoins(cost) : ""].filter(Boolean).join(" · ");
          return <article key={id} className={`editor-player${selected === id ? " selected" : ""}${origin !== "clube" ? " planned" : ""}`} draggable onDragStart={(event) => event.dataTransfer.setData("card-key", id)}>
            <button type="button" className="editor-player-select" aria-pressed={selected === id} onClick={() => setSelected(selected === id ? null : id)}>
              <CardArt player={player} className="editor-player-art" />
              <span className="editor-player-text">
                <strong>{playerName(player)}</strong>
                <span>{[player.position, ...(player.alt_positions ?? [])].join(" / ")} · {player.rating} OVR{player.nation ? ` · ${player.nation}` : ""}</span>
                {origin !== "clube" ? <em>{targetLine}</em> : null}
              </span>
              <b className="editor-player-gg" title="GG Rating da carta">{player.gg_rating ? player.gg_rating.toFixed(1) : "—"}</b>
            </button>
            {origin === "clube" ? <div className="editor-player-actions">
              <button type="button" onClick={() => moveTo(player, inBench ? "disponiveis" : "banco")}>{inBench ? "liberar" : "banco"}</button>
              <button type="button" onClick={() => moveTo(player, inUnrelated ? "disponiveis" : "nao_relacionados")}>{inUnrelated ? "liberar" : "não relacionar"}</button>
            </div> : null}
          </article>;
        })}
      </div>}
      {sourceCards.length > visibleCards ? <div className="editor-more"><span>Mostrando {visibleCards} de {sourceCards.length} cartas.</span><button type="button" className="btn ghost" onClick={() => setVisibleCards((count) => count + 60)}>mostrar mais</button></div> : null}
    </section>

    <section className="panel editor-plans" aria-labelledby="editor-plans-title">
      <div className="panel-head editor-plans-head">
        <span id="editor-plans-title">Planos salvos <span className="panel-head-sub">/ a referência não muda seu elenco no jogo</span></span>
        <div className="editor-save">
          <input value={planName} onChange={(event) => setPlanName(event.target.value)} maxLength={80} aria-label="Nome do plano" placeholder="Nome do plano" />
          <button type="button" className="btn primary" disabled={saving} onClick={() => save(!!openedPlan)}>{openedPlan ? "atualizar revisão" : "salvar plano"}</button>
          {openedPlan ? <button type="button" className="btn ghost" disabled={saving} onClick={() => { setOpenedPlan(null); setPlanName(`Plano ${plans.length + 1}`); }}>salvar como novo</button> : null}
        </div>
      </div>
      {plansError ? <p className="editor-message error editor-plans-note">{plansError}</p> : null}
      {planPendingDelete ? <div className="editor-delete-confirm" role="alert"><span>Apagar “{planPendingDelete.nome}”? Esta ação não pode ser desfeita.</span><div><button type="button" className="btn ghost" onClick={() => setPlanPendingDelete(null)} disabled={saving}>cancelar</button><button type="button" className="btn ghost danger" onClick={() => removePlan(planPendingDelete.id)} disabled={saving}>confirmar apagar</button></div></div> : null}
      {plans.length === 0 ? <p className="editor-empty">Nenhum plano salvo ainda.</p> : plans.map((view) => {
        const plan = view.plano;
        const pending = view.pendencias ?? [];
        return <article className={`editor-saved${plan.referencia ? " reference" : ""}`} key={plan.id}>
          <div className="editor-saved-text">
            <strong>{plan.nome}</strong>
            <span>
              {plan.formacao} · revisão {plan.revisao}{view.media !== undefined ? ` · média ${view.media.toFixed(1)}` : ""}
              {plan.referencia ? <> · <b className="ok">referência ativa</b></> : null}
              {pending.length === 1 ? <> · <b className="pending">1 pendência: {pending[0]}</b></> : null}
            </span>
            {pending.length > 1 ? <details className="editor-reconciliation"><summary>{pending.length} pendências de reconciliação</summary><p>Abra o plano, substitua ou retire cada carta indicada e salve uma nova revisão.</p><ul>{pending.map((entry) => <li key={entry}>{entry}</li>)}</ul></details> : null}
          </div>
          <div className="editor-saved-actions">
            <button type="button" className="btn ghost" onClick={() => openPlan(view)}>abrir</button>
            {plan.referencia ? null : <button type="button" className="btn ghost" disabled={saving} onClick={() => applyReference(plan.id)}>aplicar referência</button>}
            <button type="button" className="btn ghost danger" disabled={saving} onClick={() => setPlanPendingDelete(plan)}>apagar</button>
          </div>
        </article>;
      })}
    </section>
  </div>;
}
