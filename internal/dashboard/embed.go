package dashboard

import (
	"embed"
	"io/fs"
)

//go:embed frontend/dist/*
var frontendFS embed.FS

func GetFrontendFS() (fs.FS, error) {
	return fs.Sub(frontendFS, "frontend/dist")
}
