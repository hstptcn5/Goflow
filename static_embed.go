package main

import (
	"embed"
	"io/fs"
)

//go:embed ui/dist/*
var embeddedUI embed.FS

func getEmbeddedUI() fs.FS {
	sub, err := fs.Sub(embeddedUI, "ui/dist")
	if err != nil {
		return nil
	}
	return sub
}
