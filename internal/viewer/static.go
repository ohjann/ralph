package viewer

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// DistFS returns the embedded frontend build tree rooted at dist/.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// StaticHandler serves the embedded frontend at /, falling back to index.html
// for unknown paths so client-side routing works. /api/** paths are passed
// through untouched to the provided apiHandler.
func StaticHandler(apiHandler http.Handler) (http.Handler, error) {
	sub, err := DistFS()
	if err != nil {
		return nil, err
	}
	fileSrv := http.FileServerFS(sub)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" {
			fileSrv.ServeHTTP(w, r)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(sub, name); err == nil {
			fileSrv.ServeHTTP(w, r)
			return
		}
		// SPA fallback: unknown paths go to the client router.
		http.ServeFileFS(w, r, sub, "index.html")
	}), nil
}
