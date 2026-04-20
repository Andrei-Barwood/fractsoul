package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dashboard/*
var dashboardAssets embed.FS

func dashboardFileSystem() http.FileSystem {
	subFS, err := fs.Sub(dashboardAssets, "dashboard")
	if err != nil {
		panic(err)
	}
	return http.FS(subFS)
}
