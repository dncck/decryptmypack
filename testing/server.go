package main

import (
	"fmt"
	"log/slog"

	"github.com/df-mc/dragonfly/server"
	"github.com/df-mc/dragonfly/server/player/chat"
)

func main() {
	chat.Global.Subscribe(chat.StdoutSubscriber{})

	cf := server.DefaultConfig()
	cf.Network.Address = ":19169"

	conf, err := cf.Config(slog.Default())
	if err != nil {
		slog.Error("config error", "err", err)
		return
	}

	srv := conf.New()
	srv.CloseOnProgramEnd()

	srv.Listen()

	for p := range srv.Accept() {
		fmt.Println(p.Addr().String())
	}
}