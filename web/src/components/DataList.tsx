import type { ReactNode } from "react";
import type { OrderBy } from "../odata";

export interface DataColumn<T> {
  key: string;
  header: string;
  render: (row: T, index: number) => ReactNode;
  sortField?: string;
  optional?: boolean;
}

export default function DataList<T>({ rows, columns, rowKey, openKey, onOpen, renderExpanded, orderBy, onOrder }: {
  rows: T[];
  columns: DataColumn<T>[];
  rowKey: (row: T, index: number) => string;
  openKey?: string;
  onOpen?: (key: string) => void;
  renderExpanded?: (row: T) => ReactNode;
  orderBy?: OrderBy[];
  onOrder?: (order: OrderBy[]) => void;
}) {
  const active = (field: string) => orderBy?.find((order) => order.field === field);
  const toggle = (field: string) => {
    const current = active(field);
    onOrder?.([{ field, desc: current ? !current.desc : true }]);
  };
  return (
    <div className="data-list" role="table">
      <div className="data-list-head" role="row">
        {columns.map((column) => {
          const sortField = column.sortField ?? column.key;
          const selected = active(sortField);
          return <div className={column.optional ? "optional-metric" : undefined} key={column.key} role="columnheader" aria-sort={selected ? (selected.desc ? "descending" : "ascending") : "none"}>
            {onOrder && column.sortField ? <button type="button" onClick={() => toggle(sortField)}>{column.header}{selected ? ` ${selected.desc ? "↓" : "↑"}` : ""}</button> : column.header}
          </div>;
        })}
      </div>
      {rows.map((row, index) => {
        const key = rowKey(row, index);
        const expanded = openKey === key;
        return <div className={`data-list-row${expanded ? " open" : ""}`} role="row" key={key}>
          <div className="data-list-cells">{columns.map((column) => <div className={column.optional ? "optional-metric" : undefined} role="cell" key={column.key}>{column.render(row, index)}</div>)}</div>
          {onOpen && renderExpanded && <button type="button" className="data-list-expand" aria-expanded={expanded} onClick={() => onOpen?.(expanded ? "" : key)}>{expanded ? "recolher" : "detalhes"}</button>}
          {expanded && renderExpanded?.(row)}
        </div>;
      })}
    </div>
  );
}
