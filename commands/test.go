package commands

import (
	"encoding/json"
	"time"

	"github.com/lxfontes/jarbas/chat"
	"github.com/lxfontes/jarbas/store"
)

const (
	saveLog = "log save"
	showLog = "log show"
)

type testHandler struct {
}

var _ chat.ChatMessageHandler = &testHandler{}

func (th *testHandler) Name() string {
	return "test"
}

type logEntry struct {
	ID   string `json:"id"`
	User string `json:"user"`
	Text string `json:"text"`
	Time string `json:"time"`
}

var _ store.Storable = &logEntry{}

func (le *logEntry) StoreID() string {
	return le.ID
}

func (le *logEntry) StoreExpires() time.Time {
	return store.NeverExpire
}

// trying to write a room logger, that toggles
func (th *testHandler) OnChatMessage(msg *chat.ChatMessage) error {

	logs := []logEntry{}
	compileLog := func(out []byte) error {
		var le logEntry

		if err := json.Unmarshal(out, &le); err != nil {
			return err
		}
		logs = append(logs, le)

		return nil
	}

	switch msg.Match {
	case saveLog:
		namespace := msg.Bot.Store().Namespace("logs")
		le := &logEntry{
			ID:   "doesntmatter",
			User: msg.User.Name(),
			Text: msg.RawArgs,
			Time: time.Now().Format(time.RFC822),
		}
		err := namespace.Push("somelog", le)
		if err != nil {
			msg.Logger.WithError(err).Error("WHAT HAPPENEND OMG")
		}
	case showLog:
		namespace := msg.Bot.Store().Namespace("logs")
		if err := namespace.All("somelog", compileLog); err != nil {
			return err
		}

		for _, le := range logs {
			msg.ReplyInThread("%s[%s]: %s", le.User, le.Time, le.Text)
		}
	}

	return nil
}
