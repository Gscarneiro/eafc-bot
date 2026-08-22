// EmptyState é o "nada aqui hoje" das 5 telas — sempre com um hint que
// ensina o próximo passo (convenção do CLAUDE.md pra mensagem de erro,
// aplicada aqui a estado vazio: não é erro, mas também não deveria ser um
// beco sem saída).
export default function EmptyState({ message, hint }: { message: string; hint?: string }) {
  return (
    <div className="empty">
      <div>{message}</div>
      {hint && <div className="hint">{hint}</div>}
    </div>
  );
}
