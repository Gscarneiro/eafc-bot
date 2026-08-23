import type { ReactNode } from "react";

export interface SortOption { value: string; label: string; }

export default function RankingControls({ count, sort, onSort, options, children, onClear, hasFilters }: {
  count: number;
  sort: string;
  onSort: (value: string) => void;
  options: SortOption[];
  children?: ReactNode;
  onClear?: () => void;
  hasFilters?: boolean;
}) {
  return (
    <div className="ranking-controls" aria-label="Controles da lista">
      <div className="ranking-count"><strong>{count}</strong> resultado{count === 1 ? "" : "s"}</div>
      <div className="ranking-filters">
        {children}
        {options.length > 0 && <label className="control-select"><span>ordenar por</span><select value={sort} onChange={(e) => onSort(e.target.value)}>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>}
        {hasFilters && <button className="clear-control" type="button" onClick={onClear}>limpar filtros</button>}
      </div>
    </div>
  );
}

export function FilterSelect({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: { value: string; label: string }[] }) {
  return <label className="control-select"><span>{label}</span><select value={value} onChange={(e) => onChange(e.target.value)}>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>;
}
