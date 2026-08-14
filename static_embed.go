package main

import (
	"embed"
	"io/fs"
	"os"
	"strings"
)

//go:embed ui/dist/*
var embeddedUI embed.FS

//go:embed VERSION
var embeddedVersion string

func getAppVersion() string {
	version := strings.TrimSpace(embeddedVersion)
	if version == "" {
		return "development"
	}
	return version
}

func getEmbeddedUI() fs.FS {
	// Development: serve ui/dist directly when it exists.
	if _, err := os.Stat("ui/dist/index.html"); err == nil {
		return os.DirFS("ui/dist")
	}

	// Production: serve the bundled UI from the single executable.
	sub, err := fs.Sub(embeddedUI, "ui/dist")
	if err != nil {
		return nil
	}
	return sub
}
