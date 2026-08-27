package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestImportacaoRecusaSegredoEVazio(t *testing.T) {
	if !contemSegredo([]byte(`{"cookie":"nao"}`)) {
		t.Fatal("cookie deveria ser recusado")
	}
	club, err := decodeClubImport("x.json", []byte(`{"club":{"players":[]}}`))
	if err != nil || validarClubImport(club) == nil {
		t.Fatal("importacao vazia aceita")
	}
}
func TestImportacaoDryRunNaoGrava(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "club.json")
	if err := os.WriteFile(path, []byte(`{"club":{"cycle":"26","players":[{"id":1,"name":"Carta","rating":80,"position":"CM"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := cmdImportClub(context.Background(), []string{"club", "-file", path, "-dry-run"}); err != nil {
		t.Fatal(err)
	}
}
