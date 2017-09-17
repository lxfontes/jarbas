package reactions

import (
	"strings"

	"github.com/lxfontes/jarbas/chat"
)

type assignPR struct {
}

var _ chat.ChatMessageHandler = &assignPR{}
var _ chat.ChatEventHandler = &assignPR{}

func (apr *assignPR) Name() string {
	return "assign_pr"
}

func (apr *assignPR) OnChatEvent(ev *chat.ChatEvent) error {
	return nil
}

func (apr *assignPR) OnChatMessage(msg *chat.ChatMessage) error {
	if !strings.HasPrefix(msg.PlainText, "github.com") {
		msg.Logger.Info("reacting")
		msg.AddReaction("rage")
	}
	return nil
}
