package app

import (
	"net/http"
	"os"
	"strings"
)

type App struct {
}

func (a *App) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/download", a.download)
	mux.HandleFunc("/health", a.health)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return http.ListenAndServe(addr, mux)
}

func requiresDownloadAPISecret(r *http.Request) bool {
	secret := strings.TrimSpace(os.Getenv("DOWNLOAD_API_SHARED_SECRET"))
	if secret == "" {
		return false
	}

	host := strings.ToLower(r.Host)
	if strings.HasPrefix(host, "localhost:") || host == "localhost" || strings.HasPrefix(host, "127.0.0.1:") || host == "127.0.0.1" {
		return false
	}

	return r.Header.Get("X-Download-Api-Secret") != secret
}
