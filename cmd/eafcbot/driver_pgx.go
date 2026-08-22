//go:build postgres

package main

// O driver do Postgres entra só nesta build tag, para o bot continuar
// compilando com zero dependências no modo padrão (histórico em JSON).
//
// Para habilitar:
//
//	go get github.com/jackc/pgx/v5
//	go build -tags postgres ./cmd/eafcbot
//
// Depois é só apontar o DSN:
//
//	export EAFC_DSN="postgres://user:senha@localhost:5432/eafc?sslmode=disable"
//	psql "$EAFC_DSN" -f migrations/001_init.sql
//	eafcbot run

import _ "github.com/jackc/pgx/v5/stdlib"
