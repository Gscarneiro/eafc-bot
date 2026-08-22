import type { ReactNode } from "react";

interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  meta?: ReactNode;
  actions?: ReactNode;
}

// PageHeader é o "<h1> + linha de metadado" que as 5 telas repetiam
// coladas — extraído pra não copiar a mesma marcação de novo a cada tela
// nova, e pra dar um lugar único para ações de tela (o toggle
// escalação/tabela do /time, por exemplo).
export default function PageHeader({ eyebrow, title, meta, actions }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="titles">
        {eyebrow && <div className="eyebrow">{eyebrow}</div>}
        <h1>{title}</h1>
        {meta && <div className="meta">{meta}</div>}
      </div>
      {actions && <div className="actions">{actions}</div>}
    </div>
  );
}
