import { spawnSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const raiz = dirname(dirname(fileURLToPath(import.meta.url)));
const noWindows = process.platform === "win32";

function executar(comando, argumentos, diretorio = raiz, usarShell = false) {
  const resultado = spawnSync(comando, argumentos, {
    cwd: diretorio,
    stdio: "inherit",
    shell: usarShell,
  });

  if (resultado.error) {
    console.error(`erro: não foi possível executar ${comando}: ${resultado.error.message}`);
    process.exit(1);
  }
  if (resultado.signal) {
    process.exit(resultado.signal === "SIGINT" ? 130 : 1);
  }
  if (resultado.status !== 0) {
    process.exit(resultado.status ?? 1);
  }
}

console.log("\n→ compilando a interface…");
executar(noWindows ? "npm.cmd" : "npm", ["run", "build"], join(raiz, "web"), noWindows);

console.log("\n→ compilando o eafc-bot…");
executar("go", ["build", "./cmd/eafcbot"]);

console.log("\n→ iniciando o servidor…");
const executavel = join(raiz, noWindows ? "eafcbot.exe" : "eafcbot");
executar(executavel, ["serve", ...process.argv.slice(2)]);
