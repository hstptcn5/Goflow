package main

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed ui/dist/*
var embeddedUI embed.FS

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
