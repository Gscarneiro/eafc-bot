import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// O build sai direto em internal/webui/dist — é o caminho que o Go embute
// via //go:embed. Não tem passo de "copiar depois"; `npm run build` já
// deixa no lugar certo.
//
// O proxy de /api é só para `npm run dev`: assim o Vite (porta 5173, com
// hot-reload) fala com a API de verdade que o `eafcbot cards` sobe na 4173,
// sem precisar mocar nada nem gerar dado fake para desenvolver a UI.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://localhost:4173",
    },
  },
});
