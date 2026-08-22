// Package scheduler decide QUANDO rodar a coleta diária — o cálculo puro
// (Next, ShouldRunNow) é isolado do laço que de fato espera (Scheduler.Run),
// para o primeiro ser testável sem depender de relógio nenhum.
package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Next calcula o próximo disparo às `dailyAt` ("05:00", 24h) a partir de
// `now`, no fuso de `now`. Se o horário de hoje já passou (inclusive
// exatamente em cima dele), o próximo disparo é amanhã.
func Next(now time.Time, dailyAt string) (time.Time, error) {
	clock, err := time.Parse("15:04", dailyAt)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"daily_at %q inválido — use o formato \"05:00\" (HH:MM, 24h): %w", dailyAt, err)
	}
	next := time.Date(now.Year(), now.Month(), now.Day(),
		clock.Hour(), clock.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

// ShouldRunNow diz se, na subida do processo, a coleta deve disparar de
// imediato em vez de esperar o próximo Next() — quando não existe snapshot
// bom nenhum ainda, ou quando o último passou de `staleAfter`.
func ShouldRunNow(now, lastGood time.Time, staleAfter time.Duration) bool {
	if lastGood.IsZero() {
		return true
	}
	return now.Sub(lastGood) > staleAfter
}

// Clock isola o laço de espera do relógio de verdade. Só Run() usa isto —
// Next() e ShouldRunNow() já recebem `now` como parâmetro e não precisam.
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// Scheduler dispara Job uma vez por dia às DailyAt, e imediatamente na
// subida quando LastGood() indica um snapshot velho ou inexistente.
type Scheduler struct {
	DailyAt    string
	StaleAfter time.Duration
	Clock      Clock // nil usa o relógio de verdade
	LastGood   func(ctx context.Context) time.Time
	Job        func(ctx context.Context)
	// Log recebe uma linha por decisão relevante — a subida decidindo rodar
	// já ou esperar, e o horário do próximo disparo já resolvido (com fuso),
	// para um erro de TZ no container aparecer no primeiro `docker logs` em
	// vez de só se revelar pela hora real em que o job rodou.
	Log func(line string)
}

func (s *Scheduler) clock() Clock {
	if s.Clock != nil {
		return s.Clock
	}
	return realClock{}
}

func (s *Scheduler) log(line string) {
	if s.Log != nil {
		s.Log(line)
	}
}

// Run bloqueia disparando Job repetidamente até ctx cancelar.
func (s *Scheduler) Run(ctx context.Context) {
	now := s.clock().Now()
	if ShouldRunNow(now, s.LastGood(ctx), s.StaleAfter) {
		s.log("snapshot ausente ou velho na subida — coletando agora")
		s.Job(ctx)
	}

	for {
		now = s.clock().Now()
		next, err := Next(now, s.DailyAt)
		if err != nil {
			// daily_at malformado já é barrado em config.Validate(); chegar
			// aqui assim mesmo não trava o processo, só encerra o scheduler.
			s.log(err.Error())
			return
		}
		s.log(fmt.Sprintf("próxima coleta: %s", next.Format("2006-01-02 15:04 -07:00")))

		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.Job(ctx)
		}
	}
}
