// Package webui embute o build do app React (web/, Vite + TypeScript) no
// binário do Go. O que está em dist/ é gerado — `npm run build` dentro de
// web/ escreve direto aqui (ver web/vite.config.ts, build.outDir) — e não é
// versionado; um `go build` do zero precisa desse passo ter rodado antes,
// senão //go:embed não encontra nada pra embutir (ver README, "Começando").
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// DistFS devolve o conteúdo do build, com "dist/" já removido do caminho —
// index.html fica na raiz do fs.FS, do jeito que http.FileServer espera.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
