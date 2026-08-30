import { useCallback, useEffect, useRef, useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { fetchJob, triggerJob } from "./api";
import { formatDateTime, isZeroTime } from "./format";
import { salvarTema, temaAtual, type Tema } from "./theme";
import type { JobStatus } from "./types";
import "./theme.css";
import "./shell.css";

type IconName = "today" | "squad" | "market" | "evolution" | "settings" | "back";

function Icon({ name }: { name: IconName }) {
  const paths: Record<IconName, string> = {
    today: "M4 5h16M5 3v4m14-4v4M5 9h14M7 13h3m-3 4h5M3 5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5Z",
    squad: "M12 3 14 8l5 .5-3.8 3.3 1.2 5.1-4.4-2.8-4.4 2.8 1.2-5.1L5 8.5 10 8l2-5Z",
    market: "M4 19V5m0 14h16M7 16l3-4 3 2 5-7",
    evolution: "M12 3v4m0 10v4M5.6 5.6l2.8 2.8m7.2 7.2 2.8 2.8M3 12h4m10 0h4M5.6 18.4l2.8-2.8m7.2-7.2 2.8-2.8M12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5Z",
    settings: "M12 8.5a3.5 3.5 0 1 0 0 7 3.5 3.5 0 0 0 0-7Zm0-5v2m0 13v2m8.5-8.5h-2m-13 0h-2m13.9-6.4-1.4 1.4M7 17l-1.4 1.4m12.8 0L17 17M7 7 5.6 5.6",
    back: "m15 18-6-6 6-6",
  };
  return (
    <svg className="nav-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d={paths[name]} fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

const NAV = [
  { to: "/", label: "Hoje", end: true, icon: "today" as IconName },
  { to: "/time", label: "Elenco", icon: "squad" as IconName },
  { to: "/mercado", label: "Mercado", icon: "market" as IconName },
  { to: "/evolucoes", label: "Evoluções", icon: "evolution" as IconName },
];
const HOJE_NAV = [{ to: "/agenda", label: "Agenda" }];

const ELENCO_NAV = [
  { to: "/time", label: "Meu time", end: true },
  { to: "/time/insights", label: "Insights" },
  { to: "/time/gauntlet", label: "Gauntlet" },
  { to: "/time/planos", label: "Planejador" },
];
const AQUISICAO_NAV = [
  { to: "/mercado", label: "Oportunidades", end: true },
  { to: "/mercado/plano", label: "Mesa de decisão" },
];
const EVOLUCOES_NAV = [
  { to: "/evolucoes", label: "Análise", end: true },
  { to: "/evolucoes/catalogo", label: "Catálogo" },
  { to: "/evolucoes/salvos", label: "Salvos" },
];
const CAPITAL_NAV = [
  { to: "/capital/investimentos", label: "Investimentos" },
  { to: "/capital/vendas", label: "Vendas do banco" },
  { to: "/capital/sbcs", label: "Demanda de SBC" },
];

// App é o layout persistente: a navegação entre as telas e o indicador do
// job diário, que fica de pé independente de qual tela está aberta. Cada
// tela busca só o endpoint que ela precisa (ver useData) — nada aqui
// pré-busca dado nenhum, ao contrário do App antigo de duas telas.
export default function App() {
  const location = useLocation();
  const [tema, setTema] = useState<Tema>(temaAtual);
  const [job, setJob] = useState<JobStatus | null>(null);
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
            // As 5 telas são leves o bastante para isso não custar nada.
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

  const alternarTema = () => {
    const proximoTema = tema === "dark" ? "light" : "dark";
    salvarTema(proximoTema);
    setTema(proximoTema);
  };

  return (
    <div className="shell">
      <aside className="rail">
        <div className="rail-brand">
          <span className="brand-mark">e</span>
          <span><strong>eafc</strong><small>bot</small></span>
        </div>
        <nav className="rail-nav" aria-label="Navegação principal">
          <div className="nav-label">Workspace</div>
          {NAV.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => isActive || (item.to === "/mercado" && location.pathname.startsWith("/capital")) ? "active" : ""}>
              <Icon name={item.icon} />
              <span>{item.label}</span>
            </NavLink>
          ))}
          <div className="subnav desktop-subnav">
            {location.pathname.startsWith("/time") && <SubNav title="Elenco" items={ELENCO_NAV} />}
            {(location.pathname === "/" || location.pathname.startsWith("/agenda")) && <SubNav title="Hoje" items={HOJE_NAV} />}
            {location.pathname.startsWith("/evolucoes") && <SubNav title="Evoluções" items={EVOLUCOES_NAV} />}
            {(location.pathname.startsWith("/mercado") || location.pathname.startsWith("/capital")) && (
              <>
                <SubNav title="Aquisição" items={AQUISICAO_NAV} />
                <SubNav title="Capital" items={CAPITAL_NAV} />
              </>
            )}
          </div>
        </nav>
        <div className="rail-footer">
          <NavLink to="/configuracoes" className={({ isActive }) => (isActive ? "active" : "")}>
            <Icon name="settings" /><span>Configurações</span>
          </NavLink>
        </div>
      </aside>
      <div className="shell-body">
        <header className="topbar">
          <div className="topbar-inner">
            <div className="mobile-brand"><span className="brand-mark">e</span><span><strong>eafc</strong><small>bot</small></span></div>
            <div className="topbar-context">
              <span className="context-kicker">{location.pathname === "/" || location.pathname.startsWith("/agenda") ? "Visão geral" : location.pathname.startsWith("/time") ? "Elenco" : location.pathname.startsWith("/evolucoes") ? "Evoluções" : location.pathname.startsWith("/configuracoes") ? "Preferências" : "Mercado"}</span>
              <span className="context-title">{location.pathname === "/" ? "Hoje" : location.pathname === "/agenda" ? "Agenda" : location.pathname === "/time/insights" ? "Insights" : location.pathname.startsWith("/time") ? "Meu time" : location.pathname.startsWith("/configuracoes") ? "Configurações" : location.pathname === "/mercado/plano" ? "Mesa de decisão" : location.pathname.startsWith("/evolucoes") ? "Evoluções" : location.pathname.startsWith("/capital") ? "Capital" : "Oportunidades"}</span>
            </div>
            <div className="topbar-actions">
              <button
                type="button"
                className="theme-toggle"
                aria-label="Modo escuro"
                aria-pressed={tema === "dark"}
                title={tema === "dark" ? "Usar modo claro" : "Usar modo escuro"}
                onClick={alternarTema}
              >
                <svg className="theme-toggle-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
                  <path d="M20.4 15.1A8.5 8.5 0 0 1 8.9 3.6 8.5 8.5 0 1 0 20.4 15.1Z" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
                <span className="theme-toggle-label">modo escuro</span>
              </button>
              <div className="job">
                <span className="job-status">
                  {running ? (
                    <span className="chip turf">coletando…</span>
                  ) : job && !isZeroTime(job.last_success) ? (
                    <span className="chip flat">última coleta {formatDateTime(job.last_success!)}</span>
                  ) : job && job.last_error ? (
                    <span className="chip cost">falhou: {job.last_error}</span>
                  ) : (
                    <span className="chip flat">sem coleta ainda</span>
                  )}
                </span>
                <button className="btn" onClick={handleTrigger} disabled={running}>
                  {running ? "atualizando…" : "atualizar agora"}
                </button>
              </div>
            </div>
          </div>
        </header>
        {(location.pathname.startsWith("/time") || location.pathname.startsWith("/mercado") || location.pathname.startsWith("/evolucoes") || location.pathname.startsWith("/capital")) && (
          <div className="mobile-section-nav">
            {location.pathname.startsWith("/time") && <SubNav title="Elenco" items={ELENCO_NAV} compact />}
            {location.pathname.startsWith("/evolucoes") && <SubNav title="Evoluções" items={EVOLUCOES_NAV} compact />}
            {(location.pathname.startsWith("/mercado") || location.pathname.startsWith("/capital")) && (
              <>
                <SubNav title="Aquisição" items={AQUISICAO_NAV} compact />
                <SubNav title="Capital" items={CAPITAL_NAV} compact />
              </>
            )}
          </div>
        )}
        {location.pathname.startsWith("/agenda") && <div className="mobile-section-nav"><SubNav title="Hoje" items={HOJE_NAV} compact /></div>}
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

function SubNav({ title, items, compact = false }: { title: string; items: { to: string; label: string; end?: boolean }[]; compact?: boolean }) {
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
