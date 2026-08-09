package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"goflow/internal/nodes"
	"goflow/internal/packrun"
)

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := packrun.Run(ctx, packrun.Options{
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
	})
	if err != nil && ctx.Err() == nil && err != http.ErrServerClosed {
		log.SetOutput(io.Discard)
		fmt.Fprintf(os.Stderr, "dailyops appliance harness failed: %v\n", err)
		os.Exit(1)
	}
}
