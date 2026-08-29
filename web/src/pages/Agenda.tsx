import { useState } from "react";
import { Link } from "react-router-dom";
import { appendFeedback, fetchAgenda } from "../api";
import Chip, { type ChipTone } from "../components/Chip";
import EmptyState from "../components/EmptyState";
import PageHeader from "../components/PageHeader";
import { formatCoins, formatDateTime } from "../format";
import type { AcaoAgenda, AgendaFaixa } from "../types";
import { useData } from "../useData";
import { asyncGate } from "../components/asyncGate";
import "../shared.css";
import "./Agenda.css";

const TONE: Record<AgendaFaixa, ChipTone> = { agora: "alert", esta_semana: "gain", observando: "flat" };
const SECTIONS: { key: AgendaFaixa; title: string; note: string }[] = [
  { key: "agora", title: "Agora", note: "Prazo curto, preço-alvo confirmado ou bloqueio crítico." },
  { key: "esta_semana", title: "Esta semana", note: "Ações com prazo de até sete dias." },
  { key: "observando", title: "Observando", note: "Watchlist, dados antigos ou confiança ainda baixa." },
];
const ACTIONS_PER_SECTION = 16;

export default function Agenda() {
  const { data, loading, error, refetch } = useData(fetchAgenda, []);
  const gate = asyncGate(loading, error, data !== null, refetch); if (gate) return gate; if (!data) return null;
  const rows: Record<AgendaFaixa, AcaoAgenda[]> = { agora: data.agora ?? [], esta_semana: data.esta_semana ?? [], observando: data.observando ?? [] };
  return <div className="wrap agenda">
    <PageHeader eyebrow="copiloto · agenda" title="O que merece sua atenção" meta="Uma sequência única de decisões já calculadas pelos módulos do bot. Nada é executado automaticamente." />
    <div className="agenda-rail" aria-label="Resumo da agenda">{SECTIONS.map((section) => <a key={section.key} href={`#${section.key}`}><span>{section.title}</span><strong>{rows[section.key].length}</strong></a>)}</div>
    {SECTIONS.map((section) => <AgendaSection key={section.key} section={section} actions={rows[section.key]} />)}
  </div>;
}

function AgendaSection({ section, actions }: { section: typeof SECTIONS[number]; actions: AcaoAgenda[] }) {
  const [visibleActions, setVisibleActions] = useState(ACTIONS_PER_SECTION);
  const actionsToShow = actions.slice(0, visibleActions);
  return <section id={section.key} className="agenda-section" aria-labelledby={`${section.key}-heading`}>
    <header><div><Chip tone={TONE[section.key]}>{section.title}</Chip><h2 id={`${section.key}-heading`}>{section.note}</h2></div><strong>{actions.length} ações</strong></header>
    {actions.length === 0 ? <EmptyState message={`Nada em ${section.title.toLowerCase()} no snapshot atual.`} /> : <><ol>{actionsToShow.map((action) => <li key={action.id}><AgendaCard action={action} /></li>)}</ol>{actionsToShow.length < actions.length ? <div className="agenda-more"><p aria-live="polite">Mostrando {actionsToShow.length} de {actions.length} ações.</p><button className="btn" type="button" onClick={() => setVisibleActions((count) => count + ACTIONS_PER_SECTION)}>mostrar mais {Math.min(ACTIONS_PER_SECTION, actions.length - actionsToShow.length)}</button></div> : null}</>}
  </section>;
}
function AgendaCard({ action }: { action: AcaoAgenda }) {
  const [sent, setSent] = useState(false); const [sending, setSending] = useState(false);
  const send = async (status: "aceita" | "adiada" | "descartada") => { if (sending || sent) return; setSending(true); try { await appendFeedback({ action_id: action.id, status }); setSent(true); } finally { setSending(false); } };
  return <article className="agenda-card"><div className="agenda-card-main"><div className="agenda-card-kicker"><Chip tone="flat">{action.tipo}</Chip><span>origem: {action.proveniencia}</span></div><h3>{action.alvo}</h3><p>{action.impacto || "Sem detalhe adicional."}</p>{action.conflitos?.length ? <p className="agenda-conflict">Conflito: {action.conflitos.join(" · ")}</p> : null}</div><div className="agenda-card-meta"><span>confiança <strong>{action.confianca}</strong></span>{action.moedas ? <span>{formatCoins(action.moedas)}</span> : null}{action.prazo ? <time dateTime={action.prazo}>até {formatDateTime(action.prazo)}</time> : null}<Link className="agenda-link" to={action.link}>ver contexto</Link><div className="agenda-feedback" aria-label={`Feedback para ${action.alvo}`}>{sent ? <span>feedback salvo</span> : <><button type="button" onClick={() => send("aceita")} disabled={sending}>aceitar</button><button type="button" onClick={() => send("adiada")} disabled={sending}>adiar</button><button type="button" onClick={() => send("descartada")} disabled={sending}>descartar</button></>}</div></div></article>;
}
