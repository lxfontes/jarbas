package chat

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nlopes/slack"
)

const (
	EventConnection = "connection"
	EventReaction   = "reaction"
	EventPresence   = "presence"
)

var (
	ackTimeout = 10 * time.Second
)

type ChatHandler interface {
	Name() string
}

type ChatReply struct {
	Bot *ChatBot

	Text      string
	Target    *ChatTarget
	Timestamp string
	Id        int

	bindCallback func(ev *slack.AckMessage)
	bindErr      error
}

type ChatMessageHandler interface {
	ChatHandler
	OnChatMessage(msg *ChatMessage) error
}

type ChatEventHandler interface {
	ChatHandler
	OnChatEvent(ev *ChatEvent) error
}

type ChatEvent struct {
	Bot  *ChatBot
	Type string
	Data interface{}
}

type ChatEventConnection struct {
	Connected bool
}

type ChatEventPresence struct {
	Status string
	User   *ChatTarget
}

type ChatEventReaction struct {
	Timestamp string
	User      *ChatTarget
	Channel   *ChatTarget
	Reaction  string
	Removed   bool
}

type ChatAction struct {
	handler ChatHandler
	command bool
}

type chatArg struct {
	name        string
	required    bool
	defValue    string
	description string
}

type ChatErrorHandler func(handler ChatHandler, err error)

type ChatBot struct {
	chatHandlers   map[string][]*chatAction // indexed by command, ex: 'say'
	eventHandlers  map[string][]ChatEventHandler
	defaultHandler *chatAction
	errorHander    *ChatErrorHandler

	slackAPI *slack.Client
	slackRTM *slack.RTM

	outgoingIDs     sync.Map
	channelIDToName map[string]string
	userIDToName    map[string]string
}

func NewChatBot(token string) (*ChatBot, error) {
	return &ChatBot{
		chatHandlers:    map[string][]*chatAction{},
		eventHandlers:   map[string][]ChatEventHandler{},
		slackAPI:        slack.New(token),
		channelIDToName: map[string]string{},
		userIDToName:    map[string]string{},
	}, nil
}

func (cb *ChatBot) connectionSetup(ev *slack.ConnectedEvent) {
	cb.channelIDToName = map[string]string{}
	cb.userIDToName = map[string]string{}

	for _, user := range ev.Info.Users {
		cb.userIDToName[user.ID] = user.Name
	}

	for _, channel := range ev.Info.Channels {
		cb.channelIDToName[channel.ID] = channel.Name
	}
}

func (cb *ChatBot) NameForID(id string) (string, bool) {
	if user, ok := cb.userIDToName[id]; ok {
		return user, ok
	}

	if channel, ok := cb.channelIDToName[id]; ok {
		return channel, ok
	}

	return "", false
}

func (cb *ChatBot) Serve() {
	cb.slackRTM = cb.slackAPI.NewRTM()
	go cb.slackRTM.ManageConnection()

	for msg := range cb.slackRTM.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.HelloEvent:
			// Ignore hello

		case *slack.ConnectedEvent:
			cr := &ChatEventConnection{
				Connected: true,
			}
			cb.connectionSetup(ev)
			go cb.emitEvent(EventConnection, cr)

		case *slack.DisconnectedEvent:
			cr := &ChatEventConnection{
				Connected: false,
			}
			go cb.emitEvent(EventConnection, cr)

		case *slack.MessageEvent:
			if ev.SubType == "message_replied" {
				continue
			}
			go cb.handleMessage(ev)

		case *slack.PresenceChangeEvent:
			name, _ := cb.NameForID(ev.User)
			cr := &ChatEventPresence{
				Status: ev.Presence,
				User: &ChatTarget{
					id:   ev.User,
					name: name,
				},
			}
			go cb.emitEvent(EventPresence, cr)

		case *slack.LatencyReport:
			fmt.Printf("Current latency: %v\n", ev.Value)

		case *slack.RTMError:
			fmt.Printf("Error: %s\n", ev.Error())

		case *slack.InvalidAuthEvent:
			fmt.Printf("Invalid credentials")
			return

		case *slack.ReactionAddedEvent:
			userName, _ := cb.NameForID(ev.User)
			channelName, _ := cb.NameForID(ev.Item.Channel)
			cr := &ChatEventReaction{
				Timestamp: ev.Item.Timestamp,
				Reaction:  ev.Reaction,
				User: &ChatTarget{
					id:   ev.User,
					name: userName,
				},
				Channel: &ChatTarget{
					id:   ev.Item.Channel,
					name: channelName,
				},
			}
			go cb.emitEvent(EventReaction, cr)

		case *slack.ReactionRemovedEvent:
			userName, _ := cb.NameForID(ev.User)
			channelName, _ := cb.NameForID(ev.Item.Channel)
			cr := &ChatEventReaction{
				Timestamp: ev.Item.Timestamp,
				Reaction:  ev.Reaction,
				Removed:   true,
				User: &ChatTarget{
					id:   ev.User,
					name: userName,
				},
				Channel: &ChatTarget{
					id:   ev.Item.Channel,
					name: channelName,
				},
			}
			go cb.emitEvent(EventReaction, cr)

		case *slack.AckMessage:
			// map our internal id to a slack timestamp
			item, ok := cb.outgoingIDs.Load(ev.ReplyTo)
			if !ok {
				fmt.Println("don't know about this message", ev.ReplyTo)
				continue
			}
			cb.outgoingIDs.Delete(ev.ReplyTo)
			cr := item.(*ChatReply)
			cr.bindCallback(ev)

		default:

			// Ignore other events..
			//			fmt.Printf("Unexpected: %s %v\n", msg.Type, msg.Data)
		}
	}
}

func (cb *ChatBot) emitEvent(eventType string, data interface{}) {
	ev := &ChatEvent{
		Bot:  cb,
		Type: eventType,
		Data: data,
	}

	for _, handler := range cb.eventHandlers[eventType] {
		if err := handler.OnChatEvent(ev); err != nil {
			// TODO: not much we can recover
			return
		}
	}
}

func formatTarget(name string) string {
	return name
}

func (cb *ChatBot) Send(target *ChatTarget, threadTimestamp string, s string, args ...interface{}) (*ChatReply, error) {
	text := fmt.Sprintf(s, args...)

	cr := &ChatReply{
		Bot:    cb,
		Text:   text,
		Target: target,
	}

	msg := cb.slackRTM.NewOutgoingMessage(text, formatTarget(target.Id()))
	msg.ThreadTimestamp = threadTimestamp

	ch := make(chan struct{})
	cr.bindCallback = func(ev *slack.AckMessage) {
		cr.Timestamp = ev.Timestamp
		close(ch)
	}

	cb.outgoingIDs.Store(msg.ID, cr)

	cb.slackRTM.SendMessage(msg)

	select {
	case <-ch:
		return cr, cr.bindErr
	case <-time.After(ackTimeout):
		fmt.Println("didn't ack msg on time")
	}

	return nil, errors.New("could not confirm msg was sent")
}

func (cb *ChatBot) handleMessage(ev *slack.MessageEvent) {
	userName, _ := cb.NameForID(ev.User)
	userTarget := &ChatTarget{
		id:   ev.User,
		name: userName,
	}

	channelName, _ := cb.NameForID(ev.Channel)
	// this is a direct message
	if ev.Channel[0] == 'D' {
		channelName = userName
	}

	channelTarget := &ChatTarget{
		id:   ev.Channel,
		name: channelName,
	}

	var handlers []*chatAction
	var params string

	for pattern, ch := range cb.chatHandlers {
		if strings.HasPrefix(ev.Text, pattern) {
			handlers = ch
			params = strings.TrimPrefix(ev.Text, pattern)
			break
		}
	}

	if len(handlers) == 0 {
		if cb.defaultHandler != nil {
			msg := &ChatMessage{
				Timestamp:       ev.Timestamp,
				ThreadTimestamp: ev.ThreadTimestamp,
				Bot:             cb,
				Args:            ChatArgs{},
				User:            userTarget,
				Channel:         channelTarget,
			}
			cb.defaultHandler.handler.OnChatMessage(msg)

		}

		return
	}

	for _, ca := range handlers {
		msg := &ChatMessage{
			Timestamp:       ev.Timestamp,
			ThreadTimestamp: ev.ThreadTimestamp,
			Bot:             cb,
			Args:            ChatArgs{},
			User:            userTarget,
			Channel:         channelTarget,
		}

		scanner := bufio.NewScanner(strings.NewReader(params))
		scanner.Split(ScanQuotedWords)

		argStack := make([]chatArg, len(ca.args))
		copy(argStack, ca.args)
		// we cannot rely on positional arg after a named arg
		canNamed := true
		for scanner.Scan() {
			token := scanner.Text()

			if HasMarker(token) {
				if !canNamed {
					fmt.Println("switching from positional to named not allowed")
				}
				argName, argValue := SplitMarker(token)

				for i := 0; i < len(argStack); i++ {
					arg := argStack[i]
					if strings.ToLower(argName) == arg.name {
						msg.Args[arg.name] = argValue
						argStack = append(argStack[:i], argStack[i+1:]...)
						break
					}
				}

				continue
			}

			// no gymnastics, just pop an argument
			var arg chatArg
			canNamed = false
			arg, argStack = argStack[0], argStack[1:]
			msg.Args[arg.name] = token
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "scanning arguments:", err)
		}

		// apply optionals & fail defaults
		for _, arg := range argStack {
			if arg.required == true {
				fmt.Println("missed required arg", arg.name)
				continue // remove this
			}

			msg.Args[arg.name] = arg.defValue
		}

		ca.handler.OnChatMessage(msg)
	}
}

func (cb *ChatBot) AddEventHandler(eventType string, handler ChatEventHandler) error {
	cb.eventHandlers[eventType] = append(cb.eventHandlers[eventType], handler)
	return nil
}

func (cb *ChatBot) AddMessageHandler(pattern string, handler ChatMessageHandler, opts ...chatOpt) error {

	ca := &chatAction{
		handler: handler,
	}

	for _, opt := range opts {
		opt(ca)
	}

	if len(ca.args) > 0 {
		pattern = pattern + " " // this seems wrong
	}

	cb.chatHandlers[pattern] = append(cb.chatHandlers[pattern], ca)
	return nil
}

type ChatTarget struct {
	name string
	id   string
}

func (ct *ChatTarget) Name() string {
	return ct.name
}

func (ct *ChatTarget) Id() string {
	return ct.id
}
