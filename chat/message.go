package chat

import "fmt"

type ChatMessage struct {
	Bot             *ChatBot
	Timestamp       string
	Channel         ChatTarget
	User            *ChatUser
	Args            ChatArgs
	ThreadTimestamp string

	Match     string
	Body      string
	RawArgs   string
	IsPrivate bool
}

func (cm *ChatMessage) StringArg(arg string) (string, bool) {
	return cm.Args.String(arg)
}

func (cm *ChatMessage) IntArg(arg string) (int, bool) {
	return cm.Args.Int(arg)
}

func (cm *ChatMessage) InclusionArg(arg string, vals ...string) (string, bool) {
	return cm.Args.Inclusion(arg, vals...)
}

func (cm *ChatMessage) ReplyInThread(s string, args ...interface{}) (*ChatReply, error) {
	thread := cm.Timestamp

	// respond to the main thread if we are already in one
	if cm.ThreadTimestamp != "" {
		thread = cm.ThreadTimestamp
	}

	return cm.Bot.Send(cm.Channel, thread, s, args...)
}

func (cm *ChatMessage) Reply(s string, args ...interface{}) (*ChatReply, error) {
	return cm.Bot.Send(cm.Channel, "", s, args...)
}

func (cm *ChatMessage) ReplyWithMention(s string, args ...interface{}) (*ChatReply, error) {
	combined := fmt.Sprintf("<@%s> %s", cm.User.ID(), s)
	return cm.Bot.Send(cm.Channel, "", combined, args...)
}

func (cm *ChatMessage) ReplyPrivately(s string, args ...interface{}) (*ChatReply, error) {
	return cm.Bot.SendPrivately(cm.User, "", s, args...)
}

func (cm *ChatMessage) AddReaction(reaction string) error {
	return cm.Bot.ReactToMessage(cm, reaction)
}
