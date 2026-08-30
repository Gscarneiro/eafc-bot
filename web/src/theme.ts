export type Tema = "light" | "dark";

const CHAVE_TEMA = "eafc-bot:tema";
const CONSULTA_TEMA_ESCURO = "(prefers-color-scheme: dark)";

function temaSalvo(): Tema | null {
  try {
    const tema = window.localStorage.getItem(CHAVE_TEMA);
    return tema === "light" || tema === "dark" ? tema : null;
  } catch {
    // O navegador pode bloquear storage; o tema continua funcional na sessão.
    return null;
  }
}

function temaDoSistema(): Tema {
  return window.matchMedia?.(CONSULTA_TEMA_ESCURO).matches ? "dark" : "light";
}

function aplicarTema(tema: Tema) {
  document.documentElement.dataset.theme = tema;
  document.documentElement.style.colorScheme = tema;

  const corDoTema = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
  if (corDoTema) corDoTema.content = tema === "dark" ? "#12140f" : "#f6f2e7";
}

export function inicializarTema(): Tema {
  const tema = temaSalvo() ?? temaDoSistema();
  aplicarTema(tema);
  return tema;
}

export function temaAtual(): Tema {
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

export function salvarTema(tema: Tema) {
  aplicarTema(tema);
  try {
    window.localStorage.setItem(CHAVE_TEMA, tema);
  } catch {
    // A troca ainda vale para a página aberta quando storage não está disponível.
  }
}
