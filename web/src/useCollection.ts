import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { fetchCollection } from "./api";
import { fromSearchParams, toSearchParams, type Filter, type ODataQuery, type OrderBy } from "./odata";
import type { Facet, ODataPage } from "./types";

export interface CollectionOptions {
  defaultOrderBy?: OrderBy[];
  pageSize?: number;
  debounceSearch?: number;
  enabled?: boolean;
}

export interface CollectionResult<T> {
  rows: T[];
  count: number;
  page: number;
  pages: number;
  facets: Record<string, Facet[]>;
  raw: ODataPage<T> | null;
  query: ODataQuery;
  loading: boolean;
  error: Error | null;
  refetch: () => void;
  setFilter: (filter: Filter | undefined) => void;
  setSearch: (search: string) => void;
  setOrder: (orderBy: OrderBy[]) => void;
  setPage: (page: number) => void;
  clear: () => void;
}

export function useCollection<T>(path: string, options: CollectionOptions = {}): CollectionResult<T> {
  const [searchParams, setSearchParams] = useSearchParams();
  const [data, setData] = useState<ODataPage<T> | null>(null);
  const [error, setError] = useState<Error | null>(null);
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);
  const query = useMemo(() => {
    try { return fromSearchParams(searchParams); } catch { return {}; }
  }, [searchParams]);
  const pageSize = query.top ?? options.pageSize ?? 20;
  const [requestSearch, setRequestSearch] = useState(query.search ?? "");

  useEffect(() => {
    const timer = window.setTimeout(() => setRequestSearch(query.search ?? ""), options.debounceSearch ?? 300);
    return () => window.clearTimeout(timer);
  }, [query.search, options.debounceSearch]);

  const requestQuery = useMemo(() => ({ ...query, orderBy: query.orderBy ?? options.defaultOrderBy, search: requestSearch, top: pageSize, count: true }), [options.defaultOrderBy, pageSize, query, requestSearch]);
  const requestKey = useMemo(() => toSearchParams(requestQuery).toString(), [requestQuery]);

  useEffect(() => {
    let cancelled = false;
    if (options.enabled === false) {
      setLoading(false);
      return () => { cancelled = true; };
    }
    setLoading(true);
    fetchCollection<T>(path, requestQuery)
      .then((next) => { if (!cancelled) { setData(next); setError(null); setLoading(false); } })
      .catch((next: unknown) => { if (!cancelled) { setError(next instanceof Error ? next : new Error(String(next))); setLoading(false); } });
    return () => { cancelled = true; };
    // requestKey inclui todos os parâmetros, inclusive o search já debounced.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [options.enabled, path, requestKey, tick]);

  const update = useCallback((change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(searchParams);
    change(next);
    setSearchParams(next, { replace });
  }, [searchParams, setSearchParams]);
  const setFilter = useCallback((filter: Filter | undefined) => update((next) => { if (filter) next.set("$filter", toSearchParams({ filter }).get("$filter")!); else next.delete("$filter"); next.delete("$skip"); }), [update]);
  const setSearch = useCallback((value: string) => update((next) => { if (value.trim()) next.set("$search", value); else next.delete("$search"); next.delete("$skip"); }, true), [update]);
  const setOrder = useCallback((orderBy: OrderBy[]) => update((next) => { const value = toSearchParams({ orderBy }).get("$orderby"); if (value) next.set("$orderby", value); else next.delete("$orderby"); next.delete("$skip"); }), [update]);
  const setPage = useCallback((page: number) => update((next) => next.set("$skip", String(Math.max(0, (page - 1) * pageSize)))), [pageSize, update]);
  const clear = useCallback(() => update((next) => { next.delete("$filter"); next.delete("$orderby"); next.delete("$search"); next.delete("$skip"); }), [update]);

  return {
    rows: data?.value ?? [], count: data?.["@odata.count"] ?? 0,
    page: Math.floor((data?.["@eafc.skip"] ?? query.skip ?? 0) / pageSize) + 1,
    pages: Math.ceil((data?.["@odata.count"] ?? 0) / pageSize),
    facets: data?.["@eafc.facets"] ?? {}, query, loading, error,
    raw: data,
    refetch: useCallback(() => setTick((value) => value + 1), []),
    setFilter, setSearch, setOrder, setPage, clear,
  };
}
