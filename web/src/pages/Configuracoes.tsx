import { useEffect, useState, type ReactNode } from "react";
import { fetchConfig, saveConfig, triggerJob } from "../api";
import { asyncGate } from "../components/asyncGate";
import Chip from "../components/Chip";
import PageHeader from "../components/PageHeader";
import { useData } from "../useData";
import type { UISettings } from "../types";
import "../shared.css";
import "./Configuracoes.css";

export default function Configuracoes() {
  const { data, error, loading, refetch } = useData(fetchConfig, []);
  const gate = asyncGate(loading, error, !!data, refetch);
  const [form, setForm] = useState<UISettings | null>(null);
  const [saving, setSaving] = useState(false);
  const [running, setRunning] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (data) setForm(data.settings);
  }, [data]);

  if (gate) return gate;
  if (!data || !form) return null;

  const locked = new Set(data.env_locked);
  const patch = (section: keyof UISettings, field: string, value: string | number | boolean) => {
    setForm((current) => current ? ({ ...current, [section]: { ...current[section], [field]: value } } as UISettings) : current);
    setMessage("");
  };
  const save = async () => {
    setSaving(true);
    setMessage("");
    try {
      const next = await saveConfig(form);
      setForm(next.settings);
      setMessage("Configuração salva e aplicada. Faça uma nova coleta para recalcular as recomendações.");
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Não foi possível salvar a configuração.");
    } finally {
      setSaving(false);
    }
  };
  const refresh = async () => {
    setRunning(true);
    setMessage("");
    try {
      await triggerJob();
      setMessage("Coleta iniciada. O status no topo mostra quando terminar.");
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "Não foi possível iniciar a coleta.");
    } finally {
      setRunning(false);
    }
  };

  return (
    <div className="wrap settings-page">
      <PageHeader eyebrow="preferências" title="Configurações" meta="Ajuste o que muda suas decisões sem editar o arquivo à mão." />
      <div className="settings-toolbar">
        <div className="settings-note"><Chip tone="flat">uso local</Chip> Credenciais, rotas e armazenamento continuam fora da interface.</div>
        <div className="settings-actions">
          <button className="btn secondary" type="button" onClick={refresh} disabled={running}>{running ? "iniciando…" : "atualizar dados"}</button>
          <button className="btn primary" type="button" onClick={save} disabled={saving}>{saving ? "salvando…" : "salvar alterações"}</button>
        </div>
      </div>
      {message && <div className="banner" role="status">{message}</div>}

      <div className="settings-grid">
        <SettingsSection title="Mercado" description="O universo e o orçamento que entram na busca de oportunidades.">
          <Field label="Overall mínimo" hint="1–99"><input type="number" min="1" max="99" value={form.market.min_rating} onChange={(e) => patch("market", "min_rating", Number(e.target.value))} /></Field>
          <Field label="Overall máximo" hint="1–99"><input type="number" min="1" max="99" value={form.market.max_rating} onChange={(e) => patch("market", "max_rating", Number(e.target.value))} /></Field>
          <Field label="Preço máximo" hint="0 = sem limite"><input type="number" min="0" value={form.market.max_price} onChange={(e) => patch("market", "max_price", Number(e.target.value))} /></Field>
          <Field label="Orçamento extra" hint={locked.has("market.extra_budget") ? "controlado por EAFC_BUDGET" : "moedas além do saldo"}><input disabled={locked.has("market.extra_budget")} type="number" min="0" value={form.market.extra_budget} onChange={(e) => patch("market", "extra_budget", Number(e.target.value))} /></Field>
          <Field label="Páginas consultadas"><input type="number" min="1" value={form.market.pages} onChange={(e) => patch("market", "pages", Number(e.target.value))} /></Field>
          <Field label="Cartas por página"><input type="number" min="1" value={form.market.per_page} onChange={(e) => patch("market", "per_page", Number(e.target.value))} /></Field>
        </SettingsSection>

        <SettingsSection title="Critérios" description="O que precisa aparecer para virar uma recomendação.">
          <Field label="Ganho mínimo" hint="pontos de nota"><input type="number" min="0" step="0.1" value={form.report.min_gain} onChange={(e) => patch("report", "min_gain", Number(e.target.value))} /></Field>
          <Field label="Janela de tendência" hint="horas"><input type="number" min="1" value={form.report.trend_window_hours} onChange={(e) => patch("report", "trend_window_hours", Number(e.target.value))} /></Field>
          <Toggle label="Permitir fora de posição" checked={form.report.allow_out_of_position} onChange={(v) => patch("report", "allow_out_of_position", v)} />
          <Toggle label="Permitir cartas sem cotação" checked={form.report.allow_unpriced} onChange={(v) => patch("report", "allow_unpriced", v)} />
        </SettingsSection>

        <SettingsSection title="Agenda" description="Frequência da coleta e retenção do histórico.">
          <Field label="Coleta diária" hint="horário local"><input type="time" value={form.serve.daily_at} onChange={(e) => patch("serve", "daily_at", e.target.value)} /></Field>
          <Field label="Considerar snapshot velho após" hint="horas"><input type="number" min="1" value={form.serve.stale_after_hours} onChange={(e) => patch("serve", "stale_after_hours", Number(e.target.value))} /></Field>
          <Field label="Retenção" hint="dias"><input type="number" min="1" value={form.serve.retention_days} onChange={(e) => patch("serve", "retention_days", Number(e.target.value))} /></Field>
          <Field label="Overall mínimo de evolução" hint="inclui este overall; atual x potencial"><input type="number" min="1" max="99" value={form.serve.cards_min_rating} onChange={(e) => patch("serve", "cards_min_rating", Number(e.target.value))} /></Field>
          <Field label="Atualização rápida" hint="0 = desligada; minutos"><input type="number" min="0" value={form.serve.fast_refresh_minutes} onChange={(e) => patch("serve", "fast_refresh_minutes", Number(e.target.value))} /></Field>
          <Field label="Janela de momentum" hint="horas"><input type="number" min="1" value={form.serve.momentum_window_hours} onChange={(e) => patch("serve", "momentum_window_hours", Number(e.target.value))} /></Field>
        </SettingsSection>
      </div>
    </div>
  );
}

function SettingsSection({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return <section className="settings-section"><div className="settings-section-head"><h2>{title}</h2><p>{description}</p></div><div className="settings-fields">{children}</div></section>;
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return <label className="settings-field"><span>{label}</span>{children}{hint && <small>{hint}</small>}</label>;
}

function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return <label className="settings-toggle"><span>{label}</span><input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} /><span className="toggle-track" aria-hidden="true"><span /></span></label>;
}
