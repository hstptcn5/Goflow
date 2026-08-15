package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"goflow/config"
	"goflow/internal/buildinfo"
	"goflow/internal/cli"
	"goflow/internal/packrun"
	"goflow/internal/serverapp"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	identity := buildinfo.Current(getAppVersion())
	if len(args) > 1 && args[1] != "serve" {
		return cli.Runner{Stdout: stdout, Stderr: stderr, Stdin: os.Stdin, UIFS: getEmbeddedUI(), AppVersion: identity.Version, BuildInfo: identity}.Run(args[1:])
	}

	log.Println("==================================================")
	log.Println("[INFO] Starting Goflow Workflow Automation Engine...")
	log.Println("==================================================")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(args) == 1 {
		if bundleDir, ok := detectExtractedBundle(); ok {
			if err := packrun.RunExtractedBundle(ctx, bundleDir, packrun.Options{
				UIFS:       getEmbeddedUI(),
				AppVersion: identity.Version,
				Stdout:     stdout,
				Stderr:     stderr,
			}); err != nil {
				fmt.Fprintf(stderr, "[ERROR] %v\n", err)
				return 1
			}
			return 0
		}
	}

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

func detectExtractedBundle() (string, bool) {
	exePath, err := os.Executable()
	if err != nil {
		return "", false
	}
	return detectExtractedBundleDir(filepath.Dir(exePath))
}

func detectExtractedBundleDir(dir string) (string, bool) {
	if _, err := os.Stat(filepath.Join(dir, "PACK_INFO.json")); err != nil {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(dir, "pack", "pack.json")); err != nil {
		return "", false
	}
	return dir, true
}
