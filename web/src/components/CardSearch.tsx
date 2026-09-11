import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchCollection } from "../api";
import type { RosterCard, StarterCard } from "../types";

interface Hit {
  key: string;
  name: string;
  meta: string;
  slug?: string;
}

function toHit(card: RosterCard, meta: string): Hit | null {
  if (!card.card_slug) return null;
  return { key: card.card_slug, name: card.player.common_name || card.player.name, meta, slug: card.card_slug };
}

// CardSearch é o campo "buscar carta… ⌘K" do rail. Cobre titulares e todo o
// banco; só oferece navegação quando há detalhe de carta, pois uma carta sem
// CardReport ainda não tem rota de detalhe para abrir.
export default function CardSearch() {
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<Hit[]>([]);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const boxRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        inputRef.current?.focus();
        inputRef.current?.select();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (boxRef.current && !boxRef.current.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("mousedown", onClick);
    return () => window.removeEventListener("mousedown", onClick);
  }, []);

  useEffect(() => {
    const q = query.trim();
    if (q.length < 2) {
      setHits([]);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      Promise.all([
        fetchCollection<StarterCard>("/api/elenco/titulares", { search: q, top: 4 }),
        fetchCollection<RosterCard>("/api/elenco/reservas", { search: q, top: 4 }),
      ])
        .then(([titulares, reservas]) => {
          if (cancelled) return;
          const seen = new Set<string>();
          const out: Hit[] = [];
          for (const t of titulares.value) {
            const hit = toHit(t, `${t.position} · titular`);
            if (hit && !seen.has(hit.key)) {
              seen.add(hit.key);
              out.push(hit);
            }
          }
          for (const b of reservas.value) {
            const hit = toHit(b, `${b.player.position} · banco`);
            if (hit && !seen.has(hit.key)) {
              seen.add(hit.key);
              out.push(hit);
            }
          }
          setHits(out.slice(0, 8));
          setActive(0);
        })
        .catch(() => {
          if (!cancelled) setHits([]);
        });
    }, 250);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [query]);

  const go = (hit: Hit) => {
    if (!hit.slug) return;
    navigate(`/time/${encodeURIComponent(hit.slug)}`);
    setQuery("");
    setHits([]);
    setOpen(false);
    inputRef.current?.blur();
  };

  return (
    <div className="card-search" ref={boxRef}>
      <label className="card-search-field">
        <svg className="card-search-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
          <path d="M10.5 17a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13Zm4.8-1.7L21 21" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
        </svg>
        <span className="sr-only">Buscar carta</span>
        <input
          ref={inputRef}
          type="search"
          placeholder="buscar carta…"
          value={query}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
          onKeyDown={(e) => {
            if (e.key === "Escape") {
              setOpen(false);
              inputRef.current?.blur();
            } else if (e.key === "ArrowDown") {
              e.preventDefault();
              setActive((i) => Math.min(i + 1, hits.length - 1));
            } else if (e.key === "ArrowUp") {
              e.preventDefault();
              setActive((i) => Math.max(i - 1, 0));
            } else if (e.key === "Enter" && hits[active]) {
              go(hits[active]);
            }
          }}
        />
        <kbd className="card-search-kbd">⌘K</kbd>
      </label>
      {open && query.trim().length >= 2 && (
        <div className="card-search-results" role="listbox">
          {hits.length === 0 ? (
            <div className="card-search-empty">Nada com análise detalhada entre titulares ou banco para “{query.trim()}”.</div>
          ) : (
            hits.map((hit, i) => (
              <button
                key={hit.key}
                type="button"
                role="option"
                aria-selected={i === active}
                className={i === active ? "active" : ""}
                onMouseEnter={() => setActive(i)}
                onClick={() => go(hit)}
              >
                <span className="card-search-name">{hit.name}</span>
                <span className="card-search-meta">{hit.meta}</span>
              </button>
            ))
          )}
        </div>
      )}
    </div>
  );
}
