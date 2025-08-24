package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/robaho/fixed"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"swaps/internal/storage"
	"swaps/internal/storage/migrate"
	"swaps/internal/swap"
	"syscall"
	"time"
)

func main() {
	var (
		sps       = flag.Int("sps", 1000, "swaps per second")
		seed      = flag.Int64("seed", 42, "random seed")
		waitForDB = flag.Int("wait", 0, "seconds to wait for database (only for docker-compose)")
		truncate  = flag.Bool("truncate", false, "truncate swaps table on start")
	)
	flag.Parse()

	// When running in docker-compose, postgres takes time to boot up, wait for it to come up.
	time.Sleep(time.Duration(*waitForDB) * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	rnd := rand.New(rand.NewSource(*seed))

	db := pg.Connect(&pg.Options{
		User:     os.Getenv("POSTGRES_USER"),
		Database: os.Getenv("POSTGRES_DB"),
		Password: os.Getenv("POSTGRES_PASSWORD"),
		Addr:     os.Getenv("POSTGRES_HOST") + ":" + os.Getenv("POSTGRES_PORT"),
	})
	defer db.Close() //nolint

	log.Printf("ensuring migrations...\n")
	if err := migrate.Migrate(db, "up"); err != nil {
		log.Fatalf("migrate: %s\n", err.Error())
	}

	if *truncate {
		log.Printf("truncating db...\n")
		if _, err := db.ExecContext(ctx, "truncate table swap, swap_outbox;"); err != nil {
			log.Fatalf("truncate: %s\n", err)
		}
	}

	st := storage.New(db)

	log.Printf("generating %d swaps per second...\n", *sps)
	loop := true
	for loop {
		select {
		case <-ctx.Done():
			log.Printf("cancelled, exit")
			loop = false
		case <-tick.C:
			func() {
				t0 := time.Now()
				deadline, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()

				err := generateSwaps(deadline, st, *sps, rnd)
				if err != nil {
					log.Printf("swap generation error: %s\n", err.Error())
				}

				elapsed := time.Since(t0)
				log.Printf("%d swaps generated and persisted in %s\n", *sps, elapsed.Truncate(time.Millisecond))
				if elapsed > 1*time.Second {
					log.Printf("warning: generation takes longer than 1 second\n")
				}
			}()
		}
	}
}

type Storage interface {
	Persist(context.Context, swap.Swap) error
}

func generateSwaps(ctx context.Context, st Storage, n int, rnd *rand.Rand) error {
	for range n {
		sw := randomSwap(rnd)
		if err := st.Persist(ctx, sw); err != nil {
			return fmt.Errorf("persist swap: %w", err)
		}
	}
	return nil
}

func randomSwap(rnd *rand.Rand) swap.Swap {
	var side bool
	if rnd.Float64() >= 0.5 {
		side = true
	}

	who := ""
	for range 10 {
		who += string(rune(97 + rnd.Intn(26)))
	}

	amount := fixed.NewF(rand.Float64())

	token := ""
	switch rnd.Intn(3) {
	case 0:
		token = "BTC"
	case 1:
		token = "ETH"
	default:
		token = "KOL"
	}

	usd := amount.Mul(fakeUSDRates[token])

	dat := swap.Data{
		Who:    who,
		Token:  token,
		Amount: amount.Round(prec),
		USD:    usd.Round(prec),
		Side:   side,
	}

	return swap.New(dat)
}

var fakeUSDRates = map[string]fixed.Fixed{
	"BTC": fixed.NewF(0.12),
	"ETH": fixed.NewF(0.06),
	"KOL": fixed.NewF(0.99),
}

const prec = 6
