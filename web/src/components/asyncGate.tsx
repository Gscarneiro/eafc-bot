import type { ReactNode } from "react";
import { ApiError } from "../api";

// asyncGate é o mesmo par loading/error em todas as 5 telas — cada página
// chama isto antes de renderizar o conteúdo de verdade. Não é um
// componente: devolve o ReactNode pronto (ou null quando a tela deve
// seguir com o dado carregado).
export function asyncGate(loading: boolean, error: Error | null, hasData: boolean, onRetry?: () => void): ReactNode | null {
  if (loading && !hasData) {
    return (
      <div className="wrap">
        <div className="skeleton" style={{ width: 120, height: 12, marginBottom: 10 }} />
        <div className="skeleton" style={{ width: 260, height: 32, marginBottom: 20 }} />
        <div className="stat-grid">
          <div className="skeleton" style={{ height: 84, borderRadius: "var(--radius-lg)" }} />
          <div className="skeleton" style={{ height: 84, borderRadius: "var(--radius-lg)" }} />
          <div className="skeleton" style={{ height: 84, borderRadius: "var(--radius-lg)" }} />
        </div>
        <div className="skeleton" style={{ height: 160, borderRadius: "var(--radius-lg)" }} />
      </div>
    );
  }
  if (error) {
    const waiting = error instanceof ApiError && error.status === 503;
    return (
      <div className="wrap">
        <h1>{waiting ? "Aguardando a primeira coleta" : "Erro carregando os dados"}</h1>
        <p>
          {waiting
            ? 'O scheduler ainda não rodou hoje, ou a última coleta veio com o clube vazio. Use "atualizar agora" no topo, ou espere o próximo horário configurado.'
            : error.message}
        </p>
        {onRetry && (
          <button className="btn" onClick={onRetry}>
            tentar de novo
          </button>
        )}
      </div>
    );
  }
  return null;
}
