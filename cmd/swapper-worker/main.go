package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-pg/pg/v10"
	"log"
	"os"
	"os/signal"
	"swaps/internal/storage"
	"swaps/internal/swap"
	"swaps/internal/vm"
	"syscall"
	"time"
)

func main() {
	var (
		pollIntervalMs = flag.Int("poll-interval-ms", 100, "how often to poll for new swaps (ms)")
		workers        = flag.Int("workers", 2, "how many workers to do polling")
		waitForDB      = flag.Int("wait", 0, "seconds to wait for database (only for docker-compose)")
	)
	flag.Parse()

	// When running in docker-compose, postgres takes time to boot up, wait for it to come up.
	time.Sleep(time.Duration(*waitForDB) * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db := pg.Connect(&pg.Options{
		User:     os.Getenv("POSTGRES_USER"),
		Database: os.Getenv("POSTGRES_DB"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Addr:     os.Getenv("POSTGRES_HOST") + ":" + os.Getenv("POSTGRES_PORT"),
	})
	defer db.Close() //nolint

	st := storage.New(db)
	victoria := vm.New("http://victoriametrics:8428")

	log.Printf("booting up %d workers with %dms interval\n", *workers, *pollIntervalMs)
	for range *workers {
		go worker(ctx, time.Duration(*pollIntervalMs)*time.Millisecond, st, victoria)
	}

	<-ctx.Done()
}

type Storage interface {
	ProcessSwaps(context.Context, func([]swap.Swap) (int, error)) (int, error)
}

func worker(ctx context.Context, interval time.Duration, st Storage, victoria Victoria) {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	loop := true
	for loop {
		select {
		case <-ctx.Done():
			loop = false
		case <-tick.C:
			func() {
				t0 := time.Now()
				deadline, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()

				n, err := st.ProcessSwaps(deadline, process(ctx, victoria))
				if err != nil {
					log.Printf("process error: %s\n", err.Error())
					return
				}

				if n > 0 {
					elapsed := time.Since(t0)
					log.Printf("%d swaps processed in %s\n", n, elapsed.Truncate(time.Millisecond))
				}
			}()
		}
	}
}

type Victoria interface {
	PushBatch(context.Context, []swap.Swap) error
}

func process(ctx context.Context, vm Victoria) func([]swap.Swap) (int, error) {
	return func(sw []swap.Swap) (int, error) {
		if err := vm.PushBatch(ctx, sw); err != nil {
			return 0, fmt.Errorf("vm: %w", err)
		}
		return len(sw), nil
	}
}
