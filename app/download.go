package app

import (
	"archive/zip"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/restartfu/decryptmypack/app/minecraft"
)

var (
	commonServers = []string{
		"zeqa.net",
		"play.galaxite.net",
		"play.cubecraft.net",
		"hesu.org",
	}
	specialServers = []string{
		"play.rustmc.online",
		"geo.hivebedrock.network",
	}
	downloading = sync.Map{}

	concurrentMax = 5
	concurrent    int
)

func init() {
	for _, server := range commonServers {
		go periodicallyDownloadPacks(server)
	}
}

func periodicallyDownloadPacks(server string) {
	for {
		time.Sleep(time.Minute)
		filePath := "packs/" + server + "/19132/" + server + ".zip"

		if err := os.MkdirAll("packs/"+server+"/19132", 0777); err != nil {
			fmt.Println(err)
			continue
		}

		if err := downloadPacksFromServer(filePath, server+":19132"); err != nil {
			// Log the error (could use a proper logging framework)
			continue
		}
		time.Sleep(time.Duration(60/len(commonServers)) * time.Minute)
	}
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
	if concurrent >= concurrentMax {
		http.Error(w, "too many concurrent downloads", http.StatusServiceUnavailable)
		return
	}

	concurrent++
	defer func() {
		concurrent--
	}()

	target := r.FormValue("target")
	if target == "" {
		http.Error(w, "missing target", http.StatusBadRequest)
		return
	}

	var port = "19132"
	split := strings.Split(target, ":")
	target = split[0]

	if strings.Contains(strings.ToLower(target), "hivebedrock.network") {
		target = "geo.hivebedrock.network"
	}

	addrs, _ := net.LookupHost(target)
	if len(addrs) == 0 {
		http.Error(w, "invalid target", http.StatusBadRequest)
		return
	}

	if len(split) > 1 {
		_, err := strconv.Atoi(split[1])
		if err != nil {
			http.Error(w, "invalid port", http.StatusBadRequest)
			return
		}
		port = split[1]
	}

	if c, ok := downloading.Load(target); ok {
		<-c.(chan struct{})
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Expose-Headers", "Content-Disposition")

	for _, server := range specialServers {
		if strings.EqualFold(target, server) {
			serveFile(w, r, "packs/"+server+"/19132/"+server+".zip")
			return
		}
	}

	filePath := "packs/" + target + "/" + port + "/" + target + ".zip"
	if fileExistsAndFresh(filePath, time.Minute*60) {
		serveFile(w, r, filePath)
		return
	}

	c := make(chan struct{})
	downloading.Store(target, c)
	defer func() {
		close(c)
		downloading.Delete(target)
	}()

	if err := os.MkdirAll("packs/"+target+"/"+port, 0777); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := downloadPacksFromServer(filePath, target+":"+port); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	serveFile(w, r, filePath)
}

func fileExistsAndFresh(filePath string, maxAge time.Duration) bool {
	if fi, err := os.Stat(filePath); err == nil {
		return time.Since(fi.ModTime()) <= maxAge
	}
	return false
}

func serveFile(w http.ResponseWriter, r *http.Request, filePath string) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+strings.Split(filePath, "/")[1]+".zip\"")
	http.ServeFile(w, r, filePath)
}
