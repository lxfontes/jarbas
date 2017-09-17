package chat

import "github.com/lxfontes/jarbas/logger"

type ChatTarget interface {
	Name() string
	ID() string
}

type ChatChannel struct {
	name string
	id   string
}

func (ct *ChatChannel) Name() string {
	return ct.name
}

func (ct *ChatChannel) ID() string {
	return ct.id
}

type ChatUser struct {
	bot  *ChatBot
	name string
	id   string
	ll   logger.Log
}

func (ct *ChatUser) Name() string {
	return ct.name
}

func (ct *ChatUser) ID() string {
	return ct.id
}

func (ct *ChatUser) Bot() *ChatBot {
	return ct.bot
}

func (ct *ChatUser) Logger() logger.Log {
	return ct.ll
}
