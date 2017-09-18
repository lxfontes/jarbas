package chat

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
)

type shellHandler struct {
	name    string
	command string
}

var _ ChatMessageHandler = &shellHandler{}

func (sh *shellHandler) Name() string {
	return sh.name
}

func NewShellHandler(name string, command string) *shellHandler {
	return &shellHandler{
		name:    name,
		command: command,
	}
}

func (sh *shellHandler) OnChatMessage(msg *ChatMessage) error {
	msg.AddReaction("timer_clock")
	go func() {
		defer msg.RemoveReaction("timer_clock")
		parsedCmd, err := shlex.Split(sh.command)
		if err != nil {
			msg.AddReaction("cry")
			msg.ReplyPrivately("error parsing command: `%s`", err)
		}

		cmd := exec.Command(parsedCmd[0], parsedCmd[1:]...)
		for _, key := range msg.Args.Keys() {
			envKey := fmt.Sprintf("JARBAS_ARG_%s", envify(key))
			envVal, _ := msg.StringArg(key)
			msg.Logger.WithField("key", envKey).WithField("val", envVal).Info("env var")
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envKey, envVal))
		}

		out, err := cmd.Output()
		if err != nil {
			msg.AddReaction("cry")
			msg.ReplyPrivately("error running command: `%s`", err)
			return
		}

		msg.AddReaction("joy")
		msg.ReplyInThread("%s", fmt.Sprintf("```\n%s\n```", string(out)))
	}()
	return nil
}

func envify(k string) string {
	return strings.Replace(strings.ToUpper(k), "-", "_", -1)
}
