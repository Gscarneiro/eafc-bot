import { useCallback, useEffect, useRef, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { fetchJob, triggerJob } from "./api";
import { formatDateTime, isZeroTime } from "./format";
import type { JobStatus } from "./types";
import "./theme.css";
import "./shell.css";

const NAV = [
  { to: "/", label: "Status", end: true },
  { to: "/time", label: "Meu time" },
  { to: "/mercado", label: "Mercado" },
  { to: "/evolucoes", label: "Evoluções" },
];

// App é o layout persistente: a navegação entre as 5 telas e o indicador do
// job diário, que fica de pé independente de qual tela está aberta. Cada
// tela busca só o endpoint que ela precisa (ver useData) — nada aqui
// pré-busca dado nenhum, ao contrário do App antigo de duas telas.
export default function App() {
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

  return (
    <div className="shell">
      <aside className="rail">
        <nav className="rail-nav">
          {NAV.map((item) => (
            <NavLink key={item.to} to={item.to} end={item.end} className={({ isActive }) => (isActive ? "active" : "")}>
              {item.label}
            </NavLink>
          ))}
        </nav>
      </aside>
      <div className="shell-body">
        <header className="topbar">
          <div className="topbar-inner">
            <span className="brand">eafc-bot</span>
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
        </header>
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
