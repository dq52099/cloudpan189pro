package embed

import (
	"embed"
	"io/fs"
)

//go:embed all:fe/dist
var topFileFs embed.FS

func StaticFS() (fs.FS, bool) {
	staticFS, err := fs.Sub(topFileFs, "fe/dist")
	if err != nil {
		return nil, false
	}

	if _, err = fs.Stat(staticFS, "index.html"); err != nil {
		return nil, false
	}

	return staticFS, true
}
