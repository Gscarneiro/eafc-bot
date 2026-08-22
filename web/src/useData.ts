import { useCallback, useEffect, useState } from "react";

interface DataState<T> {
  data: T | null;
  error: Error | null;
  loading: boolean;
}

export interface DataResult<T> extends DataState<T> {
  // refetch força uma nova chamada sem esperar `deps` mudar — usado pelo
  // botão de "tentar de novo" do asyncGate quando o erro foi passageiro.
  refetch: () => void;
}

// useData busca uma vez na montagem (e de novo sempre que `deps` mudar,
// como o :slug da rota, ou `refetch` for chamado) — cada tela pede só o
// endpoint que ela precisa, nada do padrão antigo de buscar tudo uma vez na
// raiz do app.
export function useData<T>(fetcher: () => Promise<T>, deps: unknown[]): DataResult<T> {
  const [state, setState] = useState<DataState<T>>({ data: null, error: null, loading: true });
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let cancelled = false;
    setState((s) => ({ ...s, loading: true }));
    fetcher()
      .then((data) => {
        if (!cancelled) setState({ data, error: null, loading: false });
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({ data: null, error: err instanceof Error ? err : new Error(String(err)), loading: false });
        }
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, tick]);

  const refetch = useCallback(() => setTick((t) => t + 1), []);

  return { ...state, refetch };
}
