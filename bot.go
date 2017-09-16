package main

import (
	"os"

	"github.com/lxfontes/jarbas/chat"
	"github.com/lxfontes/jarbas/routes"
)

func main() {
	b, _ := chat.NewChatBot(os.Getenv("SLACK_TOKEN"))
	routes.RegisterHandlers(b)

	b.Serve()
}
