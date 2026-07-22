package main

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed ui/dist/*
var embeddedUI embed.FS

func getEmbeddedUI() fs.FS {
	// Môi trường Development: Đọc trực tiếp từ ổ đĩa cứng ui/dist để Hot Reload tức thì khi sửa code UI (không cần restart Go)
	if _, err := os.Stat("ui/dist/index.html"); err == nil {
		return os.DirFS("ui/dist")
	}

	// Môi trường Production Single Binary: Đọc từ Go embed.FS tích hợp sẵn trong file binary executable
	sub, err := fs.Sub(embeddedUI, "ui/dist")
	if err != nil {
		return nil
	}
	return sub
}
