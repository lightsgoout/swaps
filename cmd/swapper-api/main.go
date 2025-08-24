package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net"
	"net/http"
	"os/signal"
	"swaps/internal/vm"
	"syscall"
	"time"
)

func main() {
	var (
		updIntervalMs = flag.Int("upd", 100, "how often to send stats (ms)")
		httpPort      = flag.String("http-port", "8085", "http port to listen on")
		httpAddr      = flag.String("http-addr", "", "http addr to bind listen on")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	victoria := vm.New("http://victoriametrics:8428")

	lis, err := net.Listen("tcp", *httpAddr+":"+*httpPort)
	if err != nil {
		log.Fatalf("net.Listen: %s", err.Error())
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", index)
	mux.HandleFunc("/ws", ws(ctx, time.Duration(*updIntervalMs)*time.Millisecond, victoria))

	srv := http.Server{
		Handler: mux,
	}
	go func() {
		if e := srv.Serve(lis); !errors.Is(e, http.ErrServerClosed) {
			log.Printf("http.Serve: %s\n", e.Error())
		}
	}()

	log.Printf("listening on %s\n", lis.Addr())

	<-ctx.Done()
	if err = srv.Shutdown(ctx); err != nil {
		log.Printf("srv.Shutdown: %s\n", err.Error())
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write(indexHTML); err != nil {
		log.Printf("serving index.html: %s\n", err.Error())
	}
}

func ws(ctx context.Context, upd time.Duration, victoria Victoria) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websockets: upgrade error: %s\n", err.Error())
			return
		}
		defer conn.Close() //nolint

		tick := time.NewTicker(upd)
		defer tick.Stop()

		loop := true
		for loop {
			select {
			case <-ctx.Done():
				loop = false
			case <-tick.C:
				func() {
					deadline, cancel := context.WithTimeout(ctx, 1*time.Second)
					defer cancel()

					now := time.Now()
					stats := Stats{
						Timestamp: now.Format(time.StampMilli),
					}
					for _, token := range []string{"BTC", "ETH", "KOL"} {
						st, err := getStats(deadline, token, victoria)
						if err != nil {
							log.Printf("vm get stats(%s): %s\n", token, err.Error())
							continue
						}
						stats.ByToken = append(stats.ByToken, st)
					}
					if err := conn.WriteJSON(&stats); err != nil {
						log.Printf("(probably dead conn) ws wire: %s\n", err.Error())
						loop = false
						tick.Stop()
					}
				}()
			}
		}
	}
}

type Victoria interface {
	GetWindow(ctx context.Context, token string, interval string) (vm.Window, error)
}

func getStats(ctx context.Context, token string, vm Victoria) (TokenStats, error) {
	w1m, err := vm.GetWindow(ctx, token, "1m")
	if err != nil {
		return TokenStats{}, fmt.Errorf("w1m: %w", err)
	}
	w5m, err := vm.GetWindow(ctx, token, "5m")
	if err != nil {
		return TokenStats{}, fmt.Errorf("w5m: %w", err)
	}
	w1h, err := vm.GetWindow(ctx, token, "1h")
	if err != nil {
		return TokenStats{}, fmt.Errorf("w1h: %w", err)
	}
	w24h, err := vm.GetWindow(ctx, token, "24h")
	if err != nil {
		return TokenStats{}, fmt.Errorf("w24h: %w", err)
	}

	return TokenStats{
		Token: token,
		W1M:   w1m,
		W5M:   w5m,
		W1H:   w1h,
		W24H:  w24h,
	}, nil
}

type Stats struct {
	Timestamp string       `json:"timestamp"`
	ByToken   []TokenStats `json:"by_token"`
}

type TokenStats struct {
	Token string `json:"token"`

	W1M  vm.Window `json:"w1m"`
	W5M  vm.Window `json:"w5m"`
	W1H  vm.Window `json:"w1h"`
	W24H vm.Window `json:"w24h"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

//go:embed static/index.html
var indexHTML []byte
