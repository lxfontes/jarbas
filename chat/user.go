package chat

type ChatProfile struct {
}

type ChatUser struct {
	bot     *ChatBot
	name    string
	id      string
	profile *ChatProfile
}

func (ct *ChatUser) Name() string {
	return ct.name
}

func (ct *ChatUser) ID() string {
	return ct.id
}
