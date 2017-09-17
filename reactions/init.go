package reactions

import "github.com/lxfontes/jarbas/chat"

func RegisterHandlers(bot *chat.ChatBot) error {
	assignPR := &assignPR{}
	bot.AddMessageHandler("git", assignPR)
	return nil
}
