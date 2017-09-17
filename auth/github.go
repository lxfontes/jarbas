package auth

import (
	"github.com/lxfontes/jarbas/chat"
	"github.com/lxfontes/jarbas/store"
)

const (
	githubAuthCollection = "github_auth_data"
)

type githubAuth struct {
}

type GithubAuthData struct {
	UserID      string `json:"user_id"`
	GithubLogin string `json:"github_login"`
	GithubToken string `json:"github_token"`
	LoginCount  int    `json:"login_count"`
}

var _ store.Storable = &GithubAuthData{}
var _ chat.ChatExternalUser = &GithubAuthData{}

func (gd *GithubAuthData) ID() string {
	return gd.GithubLogin
}

func (gd *GithubAuthData) Name() string {
	return gd.GithubLogin
}

func (gd *GithubAuthData) Site() string {
	return "github"
}

func (gd *GithubAuthData) Token() string {
	return gd.GithubToken
}

func (gd *GithubAuthData) StoreID() string {
	return gd.UserID
}

func (gd *GithubAuthData) Validate() error {
	if gd.LoginCount > 5 {
		return chat.ErrUserAuthNeeded
	}

	// check with github if this token is still valid
	return nil
}

var _ chat.ChatAuthHandler = &githubAuth{}

func (gl *githubAuth) Name() string {
	return "github"
}

func (gl *githubAuth) Authorize(user *chat.ChatUser, role string) (chat.ChatExternalUser, error) {
	authData := &GithubAuthData{}
	userStore := user.Bot().Store()
	err := userStore.FindByID(githubAuthCollection, user.ID(), authData)

	if err != nil && err != store.ErrItemNotFound {
		return nil, err
	}

	if err == store.ErrItemNotFound {
		// onboard
		authData.UserID = user.ID()
	}

	if err = authData.Validate(); err != nil && err != chat.ErrUserAuthNeeded {
		return nil, err
	}

	if err == chat.ErrUserAuthNeeded {
		// delete local token, tell user to go through auth again
		return nil, err
	}

	//  update login counter
	authData.LoginCount++
	if err = userStore.Save(githubAuthCollection, authData); err != nil {
		return nil, err
	}

	return authData, nil
}
