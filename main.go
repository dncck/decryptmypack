package main

import (
	"log"
	"os"
	"strings"

	"github.com/restartfu/decryptmypack/app"
)

func main() {
	args := os.Args
	if len(args) >= 2 && !strings.EqualFold(args[1], "dev") {
		log.Fatalf("unsupported argument %q", args[1])
	}

	installWorkerProxy()

	addr := ":8080"
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		if strings.HasPrefix(port, ":") {
			addr = port
		} else {
			addr = ":" + port
		}
	}
	log.Printf("Server listening on %s\n", addr)

	a := app.App{}
	err := a.ListenAndServe(addr)
	if err != nil {
		panic(err)
	}
}
