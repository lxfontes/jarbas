package main

import (
	"os"

	"github.com/lxfontes/jarbas/auth"
	"github.com/lxfontes/jarbas/chat"
	"github.com/lxfontes/jarbas/routes"
)

type pluginInitializer func(*chat.ChatBot) error

func main() {
	b, _ := chat.NewChatBot(os.Getenv("SLACK_TOKEN"))

	for _, initializer := range []pluginInitializer{
		auth.RegisterHandlers,
		routes.RegisterHandlers,
	} {
		if err := initializer(b); err != nil {
			panic(err)
		}
	}

	b.Serve()
}
