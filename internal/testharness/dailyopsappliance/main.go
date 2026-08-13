package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/packrun"
	"goflow/internal/scheduler"
)

type controlledClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (c *controlledClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *controlledClock) Set(value time.Time) {
	c.mu.Lock()
	c.now = value.UTC()
	c.mu.Unlock()
}

func main() {
	packDir := flag.String("pack-dir", "", "pack directory")
	dataDir := flag.String("data-dir", "", "pack run data directory")
	uiDir := flag.String("ui-dir", filepath.Join("ui", "dist"), "built UI directory")
	telegramBaseURL := flag.String("telegram-base-url", "", "test-only Telegram API base URL")
	port := flag.Int("port", 0, "loopback port")
	flag.Parse()

	if *packDir == "" || *dataDir == "" || *telegramBaseURL == "" {
		fmt.Fprintln(os.Stderr, "--pack-dir, --data-dir, and --telegram-base-url are required")
		os.Exit(2)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	registry := nodes.NewBuiltinRegistryWithTelegramExecutor(nodes.NewTelegramBotExecutorWithClient(client, *telegramBaseURL))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	clock := &controlledClock{now: time.Date(2026, 8, 9, 1, 0, 0, 0, time.UTC)}
	wake := make(chan scheduler.WakeRequest)
	controlServer, controlURL, err := startScheduleControl(ctx, clock, wake)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dailyops schedule control failed: %v\n", err)
		os.Exit(1)
	}
	defer controlServer.Shutdown(context.Background())
	fmt.Fprintf(os.Stdout, "CONTROL: %s\n", controlURL)

	err = packrun.Run(ctx, packrun.Options{
		PackDir:              *packDir,
		DataDir:              *dataDir,
		Port:                 *port,
		NoOpen:               true,
		UIFS:                 os.DirFS(*uiDir),
		Stdout:               os.Stdout,
		Stderr:               os.Stderr,
		Registry:             registry,
		TelegramAPIBaseURL:   *telegramBaseURL,
		ConnectionTestClient: client,
		ScheduleClock:        clock,
		ScheduleWake:         wake,
	})
	if err != nil && ctx.Err() == nil && err != http.ErrServerClosed {
		log.SetOutput(io.Discard)
		fmt.Fprintf(os.Stderr, "dailyops appliance harness failed: %v\n", err)
		os.Exit(1)
	}
}

func startScheduleControl(ctx context.Context, clock *controlledClock, wake chan<- scheduler.WakeRequest) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/tick", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			Now string `json:"now"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		now, err := time.Parse(time.RFC3339, request.Now)
		if err != nil {
			http.Error(w, "invalid clock instant", http.StatusBadRequest)
			return
		}
		clock.Set(now)
		reply := make(chan scheduler.WakeResult, 1)
		select {
		case wake <- scheduler.WakeRequest{Reply: reply}:
		case <-r.Context().Done():
			return
		case <-ctx.Done():
			http.Error(w, "harness stopping", http.StatusServiceUnavailable)
			return
		}
		select {
		case result := <-reply:
			if result.Err != nil {
				http.Error(w, "scheduler tick failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(result.Result)
		case <-r.Context().Done():
		case <-ctx.Done():
			http.Error(w, "harness stopping", http.StatusServiceUnavailable)
		}
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		_ = server.Serve(listener)
	}()
	go func() {
		<-ctx.Done()
		_ = server.Shutdown(context.Background())
	}()
	return server, "http://" + listener.Addr().String(), nil
}
