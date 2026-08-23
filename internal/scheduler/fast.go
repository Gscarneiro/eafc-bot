package scheduler

import (
	"context"
	"time"
)

// FastTicker roda job repetidamente a cada interval, até ctx cancelar — é
// o ciclo leve (momentum, custo de solução de SBC) que quer "de tempos em
// tempos", não "uma vez por dia às DailyAt" como Scheduler.Run. Dispara a
// primeira vez de imediato na subida, sem esperar o primeiro intervalo
// passar, porque o objetivo é ter o sinal fresco o quanto antes — o
// fut.gg recalcula momentum a cada poucos minutos do lado deles; esperar
// uma hora de propósito só pra começar já jogaria fora esse frescor.
//
// Zero saber de fut.gg ou de análise, mesmo espírito de Scheduler: só
// decide QUANDO chamar job.
func FastTicker(ctx context.Context, interval time.Duration, job func(ctx context.Context)) {
	if interval <= 0 {
		return
	}
	job(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job(ctx)
		}
	}
}

// FastTickerDynamic é a variante usada pelo serve: quando a preferência muda
// na UI, o próximo ciclo leve respeita o novo intervalo sem reiniciar o
// processo. Um intervalo zero mantém o ticker quieto, mas continua observando
// a configuração para poder ser ligado depois.
func FastTickerDynamic(ctx context.Context, interval func() time.Duration, job func(ctx context.Context)) {
	current := interval()
	if current > 0 {
		job(ctx)
	}
	next := time.NewTimer(fastWait(current))
	defer next.Stop()
	watch := time.NewTicker(15 * time.Second)
	defer watch.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-next.C:
			current = interval()
			if current > 0 {
				job(ctx)
			}
			next.Reset(fastWait(current))
		case <-watch.C:
			updated := interval()
			if updated == current {
				continue
			}
			wasDisabled := current <= 0
			current = updated
			if wasDisabled && current > 0 {
				job(ctx)
			}
			if !next.Stop() {
				select {
				case <-next.C:
				default:
				}
			}
			next.Reset(fastWait(current))
		}
	}
}

func fastWait(interval time.Duration) time.Duration {
	if interval <= 0 {
		return 15 * time.Second
	}
	return interval
}
