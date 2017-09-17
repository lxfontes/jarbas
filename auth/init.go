package auth

import "github.com/lxfontes/jarbas/chat"

func RegisterHandlers(bot *chat.ChatBot) error {
	github := &githubAuth{}
	bot.AddAuthHandler(github)
	return nil
}
