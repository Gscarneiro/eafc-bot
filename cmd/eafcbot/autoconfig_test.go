package main

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/config"
)

// O bug que esta função corrige: futgg.respect_robots existia no config
// desde sempre, mas nada olhava pra ele — só a flag -ignore-robots
// decidia, sempre com default false. Setar "respect_robots": false no
// config.json não tinha efeito nenhum em autoconfig.
func TestRespectRobotsFalseNoConfigIgnoraSemPrecisarDaFlag(t *testing.T) {
	off := false
	cfg := config.Default()
	cfg.FutGG.RespectRobots = &off

	if !resolveIgnoreRobots(false, false, cfg) {
		t.Fatal("com respect_robots:false no config e a flag não passada, deveria ignorar o robots.txt")
	}
}

// O padrão do config (RespectRobots nil) continua obedecendo — a mudança
// não vira "ignorar sempre" por acidente pra quem nunca configurou nada.
func TestSemConfigNemFlagContinuaObedecendo(t *testing.T) {
	cfg := config.Default()

	if resolveIgnoreRobots(false, false, cfg) {
		t.Fatal("sem respect_robots no config e sem a flag, o padrão deveria continuar obedecendo o robots.txt")
	}
}

// A flag passada na linha de comando é o override pontual e tem que
// vencer o que estiver gravado no config, nos dois sentidos.
func TestFlagPassadaSempreVenceOConfig(t *testing.T) {
	on := true
	cfg := config.Default()
	cfg.FutGG.RespectRobots = &on // config manda obedecer

	if !resolveIgnoreRobots(true, true, cfg) {
		t.Fatal("-ignore-robots passada deveria vencer respect_robots:true do config")
	}

	off := false
	cfg.FutGG.RespectRobots = &off // config manda ignorar
	if resolveIgnoreRobots(false, true, cfg) {
		t.Fatal("-ignore-robots=false passada deveria vencer respect_robots:false do config")
	}
}
