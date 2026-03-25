package app

import (
	"archive/zip"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/restartfu/decryptmypack/app/minecraft"
)

var (
	downloading   = sync.Map{}
	generationSem = make(chan struct{}, 5)
)

type packRequest struct {
	Target    string
	Port      string
	Address   string
	ObjectKey string
}

func downloadPacksFromServer(filePath string, server string) error {
	filePath = strings.ToLower(filePath)
	conn, err := minecraft.Connect(server)
	if err != nil {
		return err
	}
	defer conn.Close()

	packs := conn.ResourcePacks()
	if len(packs) == 0 {
		return nil
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	zipFile := zip.NewWriter(f)
	defer zipFile.Close()

	for _, pack := range packs {
		buf, err := minecraft.EncodePack(pack)
		if err != nil {
			return err
		}
		if pack.Encrypted() {
			buf, err = minecraft.DecryptPack(buf, pack.ContentKey())
			if err != nil {
				return err
			}
		}

		p, err := zipFile.Create(pack.Name() + ".zip")
		if err != nil {
			return err
		}
		if _, err = p.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) download(w http.ResponseWriter, r *http.Request) {
	if requiresDownloadAPISecret(r) {
		writeJSON(w, http.StatusUnauthorized, packStatusResponse{
			Status: "error",
			Error:  "unauthorized",
		})
		return
	}

	req, err := parsePackRequest(r.FormValue("target"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, packStatusResponse{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	if _, inFlight := downloading.Load(req.ObjectKey); !inFlight {
		if !startPackRefresh(req) {
			writeJSON(w, http.StatusServiceUnavailable, packStatusResponse{
				Status: "error",
				Error:  "too many concurrent downloads",
			})
			return
		}
	}

	writeJSON(w, http.StatusAccepted, packStatusResponse{
		Status:         "processing",
		URL:            packPublicURL(req.ObjectKey),
		Key:            req.ObjectKey,
		PollIntervalMS: 3000,
	})
}

func parsePackRequest(target string) (packRequest, error) {
	if strings.TrimSpace(target) == "" {
		return packRequest{}, errors.New("missing target")
	}

	port := "19132"
	split := strings.Split(target, ":")
	target = strings.TrimSpace(split[0])

	if strings.Contains(strings.ToLower(target), "hivebedrock.network") {
		target = "geo.hivebedrock.network"
	}

	addrs, err := net.LookupHost(target)
	if err != nil || len(addrs) == 0 {
		return packRequest{}, errors.New("invalid target")
	}

	if len(split) > 1 {
		if _, err := strconv.Atoi(split[1]); err != nil {
			return packRequest{}, errors.New("invalid port")
		}
		port = split[1]
	}

	return packRequest{
		Target:    target,
		Port:      port,
		Address:   target + ":" + port,
		ObjectKey: packObjectKey(target, port),
	}, nil
}

func startPackRefresh(req packRequest) bool {
	done := make(chan struct{})
	if _, loaded := downloading.LoadOrStore(req.ObjectKey, done); loaded {
		return true
	}

	select {
	case generationSem <- struct{}{}:
		go func() {
			defer func() {
				<-generationSem
				close(done)
				downloading.Delete(req.ObjectKey)
			}()

			if err := refreshPack(req); err != nil {
				fmt.Println(req.Target, err)
				return
			}
		}()
		return true
	default:
		downloading.Delete(req.ObjectKey)
		return false
	}
}

func refreshPack(req packRequest) error {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
		if err := refreshPackOnce(req); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

func refreshPackOnce(req packRequest) error {
	tempDir, err := os.MkdirTemp("", "decryptmypack-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, filepath.Base(req.ObjectKey))
	if err := downloadPacksFromServer(filePath, req.Address); err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("server returned no packs")
		}
		return err
	}
	defer file.Close()

	return uploadPackToR2(req.ObjectKey, file)
}

func (a *App) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
