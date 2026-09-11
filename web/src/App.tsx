import { useCallback, useEffect, useRef, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { fetchJob, fetchResumo, saveSaldo, triggerJob } from "./api";
import CardSearch from "./components/CardSearch";
import { formatCoins, formatDateTime, formatSigned, isZeroTime } from "./format";
import { salvarTema, temaAtual, type Tema } from "./theme";
import type { JobStatus, ResumoResponse } from "./types";
import "./theme.css";
import "./shell.css";
import "./shared.css"; // .btn (o botão "coletar" da topbar) mora em shared.css

type IconName =
  | "today" | "agenda" | "squad" | "insights" | "plan" | "gauntlet"
  | "market" | "mesa" | "capital" | "evolution" | "catalogo" | "salvos"
  | "settings" | "feedback";

const ICON_PATHS: Record<IconName, string> = {
  today: "M4 5h16M5 3v4m14-4v4M5 9h14M7 13h3m-3 4h5M3 5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5Z",
  agenda: "M4 6h16M4 12h16M4 18h10",
  squad: "M12 3 14 8l5 .5-3.8 3.3 1.2 5.1-4.4-2.8-4.4 2.8 1.2-5.1L5 8.5 10 8l2-5Z",
  insights: "M5 19V10m5 9V5m5 14v-6m5 6V8",
  plan: "M4 5h16M4 12h16M4 19h16M9 3v4m6 10v4",
  gauntlet: "M7 4h10v4a5 5 0 0 1-10 0V4Zm5 9v4m-3 3h6",
  market: "M4 19V5m0 14h16M7 16l3-4 3 2 5-7",
  mesa: "M3 7h18M3 12h18M3 17h18",
  capital: "M12 2v20M17 6H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6",
  evolution: "M12 3v4m0 10v4M5.6 5.6l2.8 2.8m7.2 7.2 2.8 2.8M3 12h4m10 0h4M5.6 18.4l2.8-2.8m7.2-7.2 2.8-2.8M12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Z",
  catalogo: "M4 4h7v7H4V4Zm9 0h7v7h-7V4ZM4 13h7v7H4v-7Zm9 0h7v7h-7v-7Z",
  salvos: "M6 3h12v18l-6-4.5L6 21V3Z",
  settings: "M12 8.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7Zm0-5v2m0 13v2m8.5-8.5h-2m-13 0h-2m13.9-6.4-1.4 1.4M7 17l-1.4 1.4m12.8 0L17 17M7 7 5.6 5.6",
	feedback: "M5 4h14v11H9l-4 4V4Zm3 4h8m-8 3h5",
};

function Icon({ name }: { name: IconName }) {
  return (
    <svg className="nav-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d={ICON_PATHS[name]} fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

interface NavItem {
  to: string;
  label: string;
  end?: boolean;
  icon: IconName;
  badge?: (r: ResumoResponse | null) => { text: string; tone: "alert" | "turf" | "flat" } | null;
}
interface NavGroup {
  title: string;
  items: NavItem[];
}

// GRUPOS é a fonte única do rail — cada item aparece sempre (a árvore
// contextual antiga, que só mostrava o subnav da seção ativa, virou uma
// lista plana, como o redesenho "terminal" pede). O selo de cada item vem
// de ResumoResponse — ver internal/api/resumo.go.
const GRUPOS: NavGroup[] = [
  {
    title: "Briefing",
    items: [
      { to: "/", label: "Hoje", end: true, icon: "today", badge: (r) => (r && r.avisos.length > 0 ? { text: String(r.avisos.length), tone: "alert" } : null) },
      { to: "/agenda", label: "Agenda", icon: "agenda" },
		  { to: "/feedback", label: "Gameplay", icon: "feedback" },
    ],
  },
  {
    title: "Elenco · Squad",
    items: [
      { to: "/time", label: "Meu time", end: true, icon: "squad" },
      { to: "/time/insights", label: "Insights", icon: "insights" },
      { to: "/time/planos", label: "Planejador", icon: "plan" },
		{ to: "/time/editor", label: "Editor", icon: "plan" },
      { to: "/time/gauntlet", label: "Gauntlet", icon: "gauntlet" },
    ],
  },
  {
    title: "Mercado · Market",
    items: [
      { to: "/mercado", label: "Oportunidades", end: true, icon: "market", badge: (r) => (r && r.trocas_viaveis > 0 ? { text: String(r.trocas_viaveis), tone: "turf" } : null) },
      { to: "/mercado/plano", label: "Mesa de decisão", icon: "mesa" },
      { to: "/capital", label: "Capital", icon: "capital" },
    ],
  },
  {
    title: "Evoluções · Evos",
    items: [
      { to: "/evolucoes", label: "Análise", end: true, icon: "evolution", badge: (r) => (r && r.analise_entra_no_xi > 0 ? { text: String(r.analise_entra_no_xi), tone: "turf" } : null) },
      { to: "/evolucoes/catalogo", label: "Catálogo", icon: "catalogo", badge: (r) => (r ? { text: String(r.catalogo_elegiveis), tone: "flat" } : null) },
      { to: "/evolucoes/salvos", label: "Salvos", icon: "salvos", badge: (r) => (r && r.salvos > 0 ? { text: String(r.salvos), tone: "flat" } : null) },
    ],
  },
];

// MOBILE_PRIMARY é a barra inferior — 5 destinos fixos (o primeiro item de
// cada grupo + Configurações), porque os 13 itens do rail não cabem numa
// barra de toque. O resto do grupo aparece na faixa horizontal abaixo da
// topbar (ver .mobile-section-nav), igual ao mecanismo que já existia.
const MOBILE_PRIMARY: NavItem[] = [
  GRUPOS[0]!.items[0]!,
  GRUPOS[1]!.items[0]!,
  GRUPOS[2]!.items[0]!,
  GRUPOS[3]!.items[0]!,
  { to: "/configuracoes", label: "Config", icon: "settings" },
];

function groupForPath(pathname: string): NavGroup | null {
	if (pathname === "/" || pathname.startsWith("/agenda") || pathname.startsWith("/feedback")) return GRUPOS[0]!;
  if (pathname.startsWith("/time")) return GRUPOS[1]!;
  if (pathname.startsWith("/mercado") || pathname.startsWith("/capital")) return GRUPOS[2]!;
  if (pathname.startsWith("/evolucoes")) return GRUPOS[3]!;
  return null;
}

function SaldoEditavel({ coins, onSave }: { coins: number | null; onSave: (coins: number) => Promise<void> }) {
  const [editando, setEditando] = useState(false);
  const [valor, setValor] = useState("");
  const [salvando, setSalvando] = useState(false);
  const [erro, setErro] = useState("");
  const [confirmacao, setConfirmacao] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!editando) return;
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [editando]);

  const abrir = () => {
    if (coins === null) return;
    setValor(String(coins));
    setErro("");
    setConfirmacao("");
    setEditando(true);
  };

  const cancelar = () => {
    if (salvando) return;
    setEditando(false);
    setErro("");
  };

  const salvar = async (event: React.FormEvent) => {
    event.preventDefault();
    const numero = Number(valor);
    if (!Number.isSafeInteger(numero) || numero < 0) {
      setErro("Digite um saldo inteiro maior ou igual a zero.");
      return;
    }
    setSalvando(true);
    setErro("");
    try {
      await onSave(numero);
      setConfirmacao(`Saldo atualizado para ${formatCoins(numero)} moedas.`);
      setEditando(false);
    } catch (error) {
      setErro(error instanceof Error ? error.message : "Não foi possível salvar o saldo.");
    } finally {
      setSalvando(false);
    }
  };

  return (
    <div className={`topbar-metric saldo-metric${editando ? " editing" : ""}`}>
      {editando ? (
        <form className="saldo-editor" onSubmit={salvar}>
          <label className="metric-label" htmlFor="saldo-atual">Saldo · Coins</label>
          <div className="saldo-editor-row">
            <input
              ref={inputRef}
              id="saldo-atual"
              type="number"
              min="0"
              step="1"
              inputMode="numeric"
              value={valor}
              aria-invalid={erro ? "true" : undefined}
              aria-describedby={erro ? "saldo-erro" : undefined}
              onChange={(event) => setValor(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Escape") cancelar();
              }}
            />
            <button type="submit" className="saldo-editor-action save" disabled={salvando} aria-label="Salvar saldo" title="Salvar saldo">
              {salvando ? <span className="sync-dot running" aria-hidden="true" /> : <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5 12 4 4L19 6" /></svg>}
            </button>
            <button type="button" className="saldo-editor-action" disabled={salvando} onClick={cancelar} aria-label="Cancelar edição" title="Cancelar edição">
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" /></svg>
            </button>
          </div>
          {erro && <span id="saldo-erro" className="saldo-error" role="alert">{erro}</span>}
        </form>
      ) : (
        <>
          <button type="button" className="saldo-trigger" onClick={abrir} disabled={coins === null} aria-label={coins === null ? "Saldo indisponível" : `Alterar saldo atual de ${formatCoins(coins)} moedas`} title="Clique para alterar o saldo">
            <span className="metric-label">Saldo · Coins</span>
            <span className="metric-value coin">
              {coins === null ? "—" : formatCoins(coins)}
              {coins !== null && <svg className="saldo-edit-icon" viewBox="0 0 24 24" aria-hidden="true"><path d="m4 20 4.2-1 10.9-10.9a2.1 2.1 0 0 0-3-3L5.2 16 4 20Zm10.6-13.4 3 3" /></svg>}
            </span>
          </button>
          {confirmacao && <span className="sr-only" role="status">{confirmacao}</span>}
        </>
      )}
    </div>
  );
}

// App é o layout persistente: o rail, a topbar (faixa de métricas + job) e
// a navegação entre telas. Cada tela busca só o endpoint que ela precisa
// (ver useData) — o shell busca só o /api/resumo leve, nunca o snapshot
// inteiro.
export default function App() {
  const location = useLocation();
  const [tema, setTema] = useState<Tema>(temaAtual);
  const [job, setJob] = useState<JobStatus | null>(null);
  const [resumo, setResumo] = useState<ResumoResponse | null>(null);
  const [busy, setBusy] = useState(false);
  const pollRef = useRef<number | null>(null);

  useEffect(() => {
    let cancelled = false;
    fetchJob()
      .then((s) => {
        if (!cancelled) setJob(s);
      })
      .catch(() => {
        // Estado do job é enfeite da navegação — uma falha aqui não deve
        // impedir a tela de baixo de renderizar.
      });
    fetchResumo()
      .then((r) => {
        if (!cancelled) setResumo(r);
      })
      .catch(() => {
        // Mesma lógica: sem coleta ainda (503) é normal na primeira subida,
        // e a topbar/rail já sabem mostrar "—" e nenhum selo.
      });
    return () => {
      cancelled = true;
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
  }, []);

  const handleTrigger = useCallback(async () => {
    if (busy) return;
    setBusy(true);
    try {
      await triggerJob();
      pollRef.current = window.setInterval(async () => {
        try {
          const s = await fetchJob();
          setJob(s);
          if (!s.running && pollRef.current) {
            window.clearInterval(pollRef.current);
            pollRef.current = null;
            setBusy(false);
            // A forma mais simples de "refazer o fetch da tela": recarregar.
            // As telas são leves o bastante para isso não custar nada.
            window.location.reload();
          }
        } catch {
          if (pollRef.current) window.clearInterval(pollRef.current);
          pollRef.current = null;
          setBusy(false);
        }
      }, 2000);
    } catch {
      setBusy(false);
    }
  }, [busy]);

  const running = job?.running || busy;
  const grupoAtual = groupForPath(location.pathname);

  const alternarTema = () => {
    const proximoTema = tema === "dark" ? "light" : "dark";
    salvarTema(proximoTema);
    setTema(proximoTema);
  };

  const atualizarSaldo = useCallback(async (coins: number) => {
    const salvo = await saveSaldo(coins);
    setResumo((current) => {
      if (!current) return current;
      const diferenca = salvo.coins - current.coins;
      return {
        ...current,
        coins: salvo.coins,
        capital: {
          ...current.capital,
          cash: salvo.coins,
          available: current.capital.available + diferenca,
        },
      };
    });
    fetchResumo().then(setResumo).catch(() => {
      // O valor salvo já foi aplicado localmente; uma falha no refresh não
      // deve reabrir o editor como se a gravação tivesse falhado.
    });
  }, []);

  return (
    <div className="shell">
      <aside className="rail">
        <div className="rail-brand">
          <span className="brand-mark">e</span>
          <span><strong>eafc</strong><small>bot · fc{resumo?.cycle ?? "26"}</small></span>
        </div>
        <CardSearch />
        <nav className="rail-nav" aria-label="Navegação principal">
          {GRUPOS.map((grupo) => (
            <div className="nav-group desktop-subnav" key={grupo.title}>
              <div className="nav-label">{grupo.title}</div>
              {grupo.items.map((item) => (
                <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => (isActive ? "active" : "")}>
                  <Icon name={item.icon} />
                  <span>{item.label}</span>
                  {(() => {
                    const badge = item.badge?.(resumo);
                    return badge ? <span className={`nav-badge ${badge.tone}`}>{badge.text}</span> : null;
                  })()}
                </NavLink>
              ))}
            </div>
          ))}
        </nav>
        <div className="rail-footer">
          <NavLink to="/configuracoes" className={({ isActive }) => (isActive ? "active" : "")}>
            <Icon name="settings" /><span>Configurações</span>
          </NavLink>
        </div>
        {/* Barra inferior no mobile — GRUPOS continua a fonte, só que reduzida a 5 destinos fixos. */}
        <nav className="rail-nav-mobile" aria-label="Navegação principal">
          {MOBILE_PRIMARY.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => (isActive ? "active" : "")}>
              <Icon name={item.icon} />
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="shell-body">
        <header className="topbar">
          <div className="topbar-inner">
            <div className="mobile-brand"><span className="brand-mark">e</span><span><strong>eafc</strong><small>bot</small></span></div>
            <div className="topbar-metrics">
              <SaldoEditavel coins={resumo?.coins ?? null} onSave={atualizarSaldo} />
              <div className="topbar-metric">
                <span className="metric-label">Nota · Squad GG</span>
                <span className="metric-value">
                  {resumo ? resumo.squad_score.toFixed(1) : "—"}
                  {resumo && Math.abs(resumo.squad_score_delta ?? 0) >= 0.05 && (
                    <span className={`metric-delta ${(resumo.squad_score_delta ?? 0) > 0 ? "up" : "down"}`}>{formatSigned(resumo.squad_score_delta ?? 0)}</span>
                  )}
                </span>
              </div>
              <div className="topbar-metric">
                <span className="metric-label">Química · Chem</span>
                <span className="metric-value">
                  {resumo?.chemistry ? `${resumo.chemistry.total}/${resumo.chemistry.maximo}` : "—"}
                  {resumo?.chemistry && resumo.chemistry.total === resumo.chemistry.maximo && <span className="metric-delta up">cheia</span>}
                </span>
              </div>
              <div className="topbar-metric">
                <span className="metric-label">Coleta · Sync</span>
                <span className="metric-value small">
                  <span className={`sync-dot${running ? " running" : job?.last_error ? " error" : ""}`} aria-hidden="true" />
                  {running
                    ? "coletando…"
                    : job && !isZeroTime(job.last_success)
                      ? formatDateTime(job.last_success!)
                      : job?.next_run
                        ? `próxima coleta ${formatDateTime(job.next_run)}`
                        : "sem coleta ainda"}
                </span>
              </div>
            </div>
            <div className="topbar-actions">
              <button
                type="button"
                className="theme-toggle icon-only"
                aria-label="Modo claro"
                aria-pressed={tema === "dark"}
                title={tema === "dark" ? "Usar modo claro" : "Usar modo escuro"}
                onClick={alternarTema}
              >
                <svg className="theme-toggle-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                  <path d="M20.4 15.1A8.5 8.5 0 0 1 8.9 3.6 8.5 8.5 0 1 0 20.4 15.1Z" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
              <button className="btn primary" onClick={handleTrigger} disabled={running}>
                {running ? "atualizando…" : "coletar"}
              </button>
            </div>
          </div>
        </header>
        {grupoAtual && (
          <div className="mobile-section-nav">
            <SubNav title={grupoAtual.title} items={grupoAtual.items} compact />
          </div>
        )}
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function SubNav({ title, items, compact = false }: { title: string; items: NavItem[]; compact?: boolean }) {
  return (
    <div className={`subnav-group${compact ? " compact" : ""}`}>
      {!compact && <div className="subnav-title">{title}</div>}
      <div className="subnav-items">
        {items.map((item) => (
          <NavLink key={item.to} to={item.to} end={item.end}>{item.label}</NavLink>
        ))}
      </div>
    </div>
  );
}
