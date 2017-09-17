package routes

import (
	"fmt"

	"github.com/lxfontes/jarbas/chat"
	"github.com/lxfontes/jarbas/store"
)

const (
	githubCollectionName = "github_links"

	githubSaveCommand  = "github link"
	githubFetchCommand = "github list"
)

type githubLink struct {
}

type githubLinkData struct {
	ID   string `json:"id"`
	Link string `json:"link"`
}

var _ store.Storable = &githubLinkData{}

func (gd *githubLinkData) StoreID() string {
	return gd.ID
}

var _ chat.ChatMessageHandler = &githubLink{}

func (gl *githubLink) Name() string {
	return "github_link"
}

func (gl *githubLink) authHandler(user *chat.ChatUser, role string) (*chat.ChatExternalUser, error) {
	if user.Name() == "lxfontes" {
		return chat.NewChatExternalUser(user, "external_name", "external_id", "external_token"), nil
	}

	return nil, chat.ErrUserAuthNeeded
}

func (gl *githubLink) OnChatMessage(msg *chat.ChatMessage) error {
	linkedUser, err := msg.Bot.AuthUser(msg.User, "github", "someteam")
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", err)
	fmt.Printf("%+v\n", linkedUser)
	return nil
}
