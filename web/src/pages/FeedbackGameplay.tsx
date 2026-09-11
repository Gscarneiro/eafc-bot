import { useMemo, useState } from "react";
import { fetchGameplayFeedback, fetchGameplayQuality, fetchSquadEditor, saveGameplayFeedback } from "../api";
import { asyncGate } from "../components/asyncGate";
import PageHeader from "../components/PageHeader";
import { useData } from "../useData";
import type { ClubPlayer, GameplayFeedback, Position } from "../types";
import "../shared.css";

const POSITIONS: Position[] = ["GK", "RB", "CB", "LB", "RWB", "LWB", "CDM", "CM", "CAM", "RM", "LM", "RW", "LW", "CF", "ST"];
const initial: GameplayFeedback = { comparacao_id: "", carta_a: "", carta_b: "", preferencia: "insuficiente" };

function nomeCarta(value: string) { return value.trim().toLocaleLowerCase(); }
function rotuloCarta(player: ClubPlayer) { return `${player.common_name || player.name} · ${player.version || "base"} · ${player.position} · ${player.club_item_id || player.id}`; }

export default function FeedbackGameplay() {
  const feedbackData = useData(fetchGameplayFeedback, []);
  const clubData = useData(fetchSquadEditor, []);
	const qualityData = useData(fetchGameplayQuality, []);
  const [form, setForm] = useState<GameplayFeedback>(initial);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
	const cartas = useMemo(() => (clubData.data?.cartas ?? []).map(({ player }) => ({ rotulo: rotuloCarta(player), player })).sort((a, b) => a.rotulo.localeCompare(b.rotulo)), [clubData.data]);
  const gate = asyncGate(feedbackData.loading || clubData.loading, feedbackData.error || clubData.error, !!feedbackData.data && !!clubData.data, () => { void feedbackData.refetch(); void clubData.refetch(); });
  if (gate || !feedbackData.data || !clubData.data) return gate;

  const update = <K extends keyof GameplayFeedback>(key: K, value: GameplayFeedback[K]) => setForm((current) => ({ ...current, [key]: value }));
  const comparisonID = [nomeCarta(form.carta_a), nomeCarta(form.carta_b), form.patch?.trim(), form.plataforma?.trim(), form.posicao, form.funcao?.trim(), form.estilo_jogo?.trim()].join("|");
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form.carta_a.trim() || !form.carta_b.trim()) { setMessage("Escolha as duas cartas antes de registrar."); return; }
    if (nomeCarta(form.carta_a) === nomeCarta(form.carta_b)) { setMessage("A comparação precisa de duas cartas diferentes."); return; }
    setSaving(true); setMessage("");
    try {
		const cartaA = cartas.find((entry) => entry.rotulo === form.carta_a)?.player;
		const cartaB = cartas.find((entry) => entry.rotulo === form.carta_b)?.player;
		if (!cartaA || !cartaB) { setMessage("Escolha cartas da lista do clube para preservar a identidade da versão."); setSaving(false); return; }
		await saveGameplayFeedback({ ...form, comparacao_id: comparisonID, carta_a_id: cartaA.id, carta_b_id: cartaB.id, carta_a_club_item_id: cartaA.club_item_id || undefined, carta_b_club_item_id: cartaB.club_item_id || undefined });
      setMessage("Comparação registrada. Um novo envio no mesmo contexto corrige esta evidência, sem duplicá-la.");
		setForm(initial); await feedbackData.refetch(); await qualityData.refetch();
    } catch (error) { setMessage(error instanceof Error ? error.message : "Não foi possível registrar a comparação."); }
    finally { setSaving(false); }
  };

  return <div className="wrap">
    <PageHeader eyebrow="evidência de gameplay" title="Comparar cartas" meta="Registre uma impressão de campo no mesmo contexto. Isso gera evidência para avaliar o perfil; não muda pesos nem o meta automaticamente." />
    <form className="settings-section" onSubmit={submit}>
      <div className="settings-section-head"><h2>Comparação curta</h2><p>Preferência esportiva, compra e resultado financeiro ficam separados.</p></div>
      <div className="settings-fields">
        <Field label="Carta A"><input list="cartas-clube" value={form.carta_a} onChange={(e) => update("carta_a", e.target.value)} required /></Field>
        <Field label="Carta B"><input list="cartas-clube" value={form.carta_b} onChange={(e) => update("carta_b", e.target.value)} required /></Field>
        <Field label="Preferência"><select value={form.preferencia} onChange={(e) => update("preferencia", e.target.value as GameplayFeedback["preferencia"])}><option value="a">Carta A</option><option value="b">Carta B</option><option value="empate">Empate</option><option value="insuficiente">Experiência insuficiente</option></select></Field>
        <Field label="Posição"><select value={form.posicao ?? ""} onChange={(e) => update("posicao", (e.target.value || undefined) as Position | undefined)}><option value="">não informada</option>{POSITIONS.map((position) => <option key={position}>{position}</option>)}</select></Field>
        <Field label="Função"><input value={form.funcao ?? ""} onChange={(e) => update("funcao", e.target.value)} placeholder="ex.: volante marcador" /></Field>
		<Field label="Química"><input type="number" min="0" max="3" value={form.quimica ?? ""} onChange={(e) => update("quimica", e.target.value === "" ? undefined : Number(e.target.value))} /></Field>
        <Field label="Patch"><input value={form.patch ?? ""} onChange={(e) => update("patch", e.target.value)} placeholder="ex.: 1.2" /></Field>
        <Field label="Plataforma"><select value={form.plataforma ?? ""} onChange={(e) => update("plataforma", e.target.value)}><option value="">não informada</option><option value="ps5">PS5</option><option value="xbox_series">Xbox Series</option><option value="pc">PC</option></select></Field>
        <Field label="Estilo de jogo"><select value={form.estilo_jogo ?? ""} onChange={(e) => update("estilo_jogo", e.target.value)}><option value="">não informado</option><option value="meta_competitivo">Meta competitivo</option><option value="posse">Posse</option><option value="contra_ataque">Contra-ataque</option><option value="jogo_pelas_pontas">Jogo pelas pontas</option></select></Field>
		<Field label="Perfil usado"><input value={form.perfil ?? ""} onChange={(e) => update("perfil", e.target.value)} placeholder="ex.: meta_competitivo" /></Field>
        <Field label="Uso no campo"><input value={form.uso ?? ""} onChange={(e) => update("uso", e.target.value)} placeholder="ex.: 10 partidas em Rivals" /></Field>
        <Field label="Decisão de compra"><input value={form.decisao_compra ?? ""} onChange={(e) => update("decisao_compra", e.target.value)} placeholder="ex.: comprei a carta B" /></Field>
        <Field label="Resultado financeiro"><input value={form.resultado_financeiro ?? ""} onChange={(e) => update("resultado_financeiro", e.target.value)} placeholder="ex.: venda com lucro" /></Field>
      </div>
	  <datalist id="cartas-clube">{cartas.map((entry) => <option value={entry.rotulo} key={entry.rotulo} />)}</datalist>
      <div className="settings-actions"><button className="btn primary" disabled={saving} type="submit">{saving ? "registrando…" : "registrar comparação"}</button></div>
      {message ? <p className="banner" role="status">{message}</p> : null}
    </form>
    {qualityData.data ? <section className="settings-section"><div className="settings-section-head"><h2>Qualidade do ranking</h2><p>Amostra reservada: {qualityData.data.candidata.cobertas}/{qualityData.data.candidata.comparacoes} comparações cobertas.</p></div><div className="settings-fields"><div className="settings-field"><span>Perfil {qualityData.data.perfil}</span><strong>{qualityData.data.candidata.concordancia === undefined ? "cobertura insuficiente" : `${(qualityData.data.candidata.concordancia * 100).toFixed(0)}% de concordância`}</strong></div><div className="settings-field"><span>FUT.GG</span><strong>{qualityData.data.externa_gg.concordancia === undefined ? "cobertura insuficiente" : `${(qualityData.data.externa_gg.concordancia * 100).toFixed(0)}% de concordância`}</strong></div></div></section> : null}
    <section className="settings-section"><div className="settings-section-head"><h2>Evidências registradas</h2><p>{feedbackData.data["@odata.count"]} contexto(s) independente(s).</p></div><div className="data-table"><table><thead><tr><th>Comparação</th><th>Contexto</th><th>Preferência</th><th>Registro</th></tr></thead><tbody>{(feedbackData.data.value ?? []).length === 0 ? <tr><td colSpan={4}>Nenhuma comparação registrada.</td></tr> : (feedbackData.data.value ?? []).map((entry) => <tr key={entry.id ?? entry.comparacao_id}><td>{entry.carta_a} × {entry.carta_b}</td><td>{[entry.posicao, entry.funcao, entry.patch, entry.plataforma].filter(Boolean).join(" · ") || "contexto incompleto"}</td><td>{entry.preferencia}</td><td>{entry.registrado_em ? new Date(entry.registrado_em).toLocaleString("pt-BR") : "—"}</td></tr>)}</tbody></table></div></section>
  </div>;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label className="settings-field"><span>{label}</span>{children}</label>; }
