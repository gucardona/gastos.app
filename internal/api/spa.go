package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serve arquivos de fsys; caminhos sem arquivo correspondente recebem index.html.
func spaHandler(fsys fs.FS) http.Handler {
	files := http.FileServerFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if info, err := fs.Stat(fsys, p); err != nil || info.IsDir() {
			if p != "index.html" {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				w.Header().Set("Cache-Control", "no-cache")
				files.ServeHTTP(w, r2)
				return
			}
		}
		if strings.HasPrefix(p, "assets/") { // arquivos com hash do Vite
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}
