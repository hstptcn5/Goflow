package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"goflow/config"
	"goflow/internal/cli"
	"goflow/internal/serverapp"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 1 && args[1] != "serve" {
		return cli.Run(args[1:], stdout, stderr)
	}

	log.Println("==================================================")
	log.Println("[INFO] Starting Goflow Workflow Automation Engine...")
	log.Println("==================================================")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := serverapp.Run(ctx, serverapp.Options{
		Config: config.LoadConfig(),
		UIFS:   getEmbeddedUI(),
		Logger: log.Default(),
	}); err != nil {
		fmt.Fprintf(stderr, "[ERROR] %v\n", err)
		return 1
	}
	log.Println("[INFO] Goflow stopped successfully.")
	return 0
}
