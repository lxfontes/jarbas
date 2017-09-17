package routes

import (
	"fmt"

	"github.com/lxfontes/jarbas/chat"
)

type trackHandler struct {
	trackMoji map[string]int
}

var _ chat.ChatMessageHandler = &trackHandler{}
var _ chat.ChatEventHandler = &trackHandler{}

func (th *trackHandler) Name() string {
	return "track"
}

func (th *trackHandler) OnChatMessage(msg *chat.ChatMessage) error {
	cr, err := msg.Reply("tag this with reaction")
	if err != nil {
		return err
	}

	th.trackMoji[cr.Timestamp] = 0

	return msg.Bot.ReactToMessage(msg, "aw_yeah")
}

func (th *trackHandler) OnChatEvent(ev *chat.ChatEvent) error {
	switch data := ev.Data.(type) {
	case *chat.ChatEventReaction:
		curVal, ok := th.trackMoji[data.Timestamp]
		if !ok {
			fmt.Println("not tracking", data.Timestamp)
			return nil
		}

		if data.Removed {
			if curVal > 0 {
				th.trackMoji[data.Timestamp]--
			}
		} else {
			th.trackMoji[data.Timestamp]++
		}

		ev.Bot.Send(data.Channel, data.Timestamp, "thx for reaction .... counting %d", th.trackMoji[data.Timestamp])
	default:
		fmt.Println("wut?", ev.Type)
	}
	return nil
}

func RegisterHandlers(b *chat.ChatBot) error {
	th := &trackHandler{
		trackMoji: map[string]int{},
	}
	b.AddMessageHandler("track", th)

	b.AddEventHandler(chat.EventReaction, th)

	tt := &testHandler{}
	b.AddMessageHandler("auth github", tt)

	return nil
}
