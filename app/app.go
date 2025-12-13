package app

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/restartfu/decryptmypack/app/template"
)

var (
	router = mux.NewRouter()
)

type App struct {
}

func (a *App) ListenAndServe(addr string, dev bool) error {
	serverAddr := "https://decryptmypack.com:443"
	if dev {
		serverAddr = "http://localhost:8080"
	}

	router.HandleFunc("/download", a.download).Queries("target", "{target}")

	router.HandleFunc("/", serveFileFunc("./frontend/static/home.html"))
	router.HandleFunc("/style.css", serveFileFunc("./frontend/static/style.css"))
	router.HandleFunc("/src/script.js", template.NewFS("./frontend/src/script.js", strings.NewReplacer(
		"$SERVER_ADDR", serverAddr,
	)))
	router.HandleFunc("/assets/{path}", serveDirFunc("./frontend"))

	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusFound)
	})

	if dev {
		return http.ListenAndServe(addr, router)
	}
	return http.ListenAndServeTLS(addr, "./certificate.crt", "./private.key", router)
}

// serverDirFunc serves files from the given directory.
func serveDirFunc(dir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, dir+r.URL.Path)
	}
}

func serveFileFunc(name string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, name)
	}
}
