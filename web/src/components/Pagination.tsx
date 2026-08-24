export default function Pagination({ page, pages, onPage }: { page: number; pages: number; onPage: (page: number) => void }) {
  if (pages <= 1) return null;
  return (
    <nav className="pagination" aria-label="Paginação">
      <button type="button" disabled={page <= 1} onClick={() => onPage(page - 1)}>anterior</button>
      <span>Página {page} de {pages}</span>
      <button type="button" disabled={page >= pages} onClick={() => onPage(page + 1)}>próxima</button>
    </nav>
  );
}
