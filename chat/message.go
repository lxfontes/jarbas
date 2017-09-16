package chat

type ChatMessage struct {
	Timestamp       string
	ThreadTimestamp string
	Bot             *ChatBot
	Channel         *ChatTarget
	User            *ChatTarget
	Args            ChatArgs

	Body      string
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

func (cm *ChatMessage) Thread(s string, args ...interface{}) (*ChatReply, error) {
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

func (cm *ChatMessage) ReplyWithMention(msg string) error {
	return nil
}

func (cm *ChatMessage) ReplyPrivately(msg string) error {
	return nil
}
