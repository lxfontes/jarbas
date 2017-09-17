package commands

import (
	"github.com/lxfontes/jarbas/auth"
	"github.com/lxfontes/jarbas/chat"
)

type testHandler struct {
}

var _ chat.ChatMessageHandler = &testHandler{}

func (th *testHandler) Name() string {
	return "test"
}

func (th *testHandler) OnChatMessage(msg *chat.ChatMessage) error {
	user, err := msg.AuthorizeUser("github", "team")
	if err != nil {
		return err
	}

	authData := user.(*auth.GithubAuthData)
	msg.Reply("Hello %s from %s, login count %d", user.Name(), user.Site(), authData.LoginCount)
	return nil
}
