package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// A diferença que importa em FastTicker: ele não espera o primeiro
// intervalo passar pra rodar job a primeira vez — diferente de esperar
// "de hora em hora" a partir de agora, que deixaria o sinal (momentum,
// custo de SBC) velho por até um intervalo inteiro logo na subida.
func TestFastTickerDisparaImediatoNaSubida(t *testing.T) {
	var calls int32
	ctx, cancel := context.WithCancel(context.Background())

	go FastTicker(ctx, time.Hour, func(context.Context) {
		if atomic.AddInt32(&calls, 1) == 1 {
			cancel() // já provou o que importa: rodou sem esperar 1h
		}
	})

	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		select {
		case <-deadline:
			t.Fatal("job não rodou imediatamente na subida")
		case <-time.After(time.Millisecond):
		}
	}
}

// interval <= 0 não gira em loop nenhum — evita um ticker de período zero
// (que time.NewTicker rejeita com panic) se algum dia FastRefreshMinutes
// chegar aqui zerado por engano.
func TestFastTickerIntervaloInvalidoNaoRoda(t *testing.T) {
	var calls int32
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	FastTicker(ctx, 0, func(context.Context) { atomic.AddInt32(&calls, 1) })

	if calls != 0 {
		t.Fatalf("interval<=0 não deveria chamar job nenhuma vez, chamou %d", calls)
	}
}
