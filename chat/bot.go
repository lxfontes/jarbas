package chat

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lxfontes/jarbas/store"
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
	Target    ChatTarget
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
	User   *ChatUser
}

type ChatEventReaction struct {
	Timestamp string
	User      *ChatUser
	Channel   ChatTarget
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

type ChatExternalUser struct {
	user  *ChatUser
	site  string
	name  string
	id    string
	token string
}

func (ceu *ChatExternalUser) User() *ChatUser {
	return ceu.user
}

func (ceu *ChatExternalUser) Site() string {
	return ceu.site
}

func (ceu *ChatExternalUser) Name() string {
	return ceu.name
}

func (ceu *ChatExternalUser) ID() string {
	return ceu.id
}

func (ceu *ChatExternalUser) Token() string {
	return ceu.token
}

func NewChatExternalUser(user *ChatUser, name string, id string, token string) *ChatExternalUser {
	return &ChatExternalUser{
		user:  user,
		name:  name,
		id:    id,
		token: token,
	}
}

var ErrUserAuthNeeded = errors.New("need auth for site")

type ChatAuthHandler func(user *ChatUser, role string) (*ChatExternalUser, error)

type ChatBot struct {
	chatHandlers   map[string][]*chatAction // indexed by command, ex: 'say'
	eventHandlers  map[string][]ChatEventHandler
	authHandlers   map[string]ChatAuthHandler
	defaultHandler *chatAction
	errorHander    *ChatErrorHandler

	slackAPI *slack.Client
	slackRTM *slack.RTM

	outgoingIDs sync.Map // used to track outgoing message timestamps (ChatReply)

	// keeps an in-memory representation of our workspace
	channelIDToName map[string]string
	userIDToName    map[string]string

	store store.Store
}

func NewChatBot(token string) (*ChatBot, error) {
	return &ChatBot{
		chatHandlers:    map[string][]*chatAction{},
		eventHandlers:   map[string][]ChatEventHandler{},
		authHandlers:    map[string]ChatAuthHandler{},
		slackAPI:        slack.New(token),
		channelIDToName: map[string]string{},
		userIDToName:    map[string]string{},
		store:           store.NewMemoryStore(),
	}, nil
}

func (cb *ChatBot) connectionSetup(ev *slack.ConnectedEvent) {
	cb.channelIDToName = map[string]string{}
	cb.userIDToName = map[string]string{}

	for _, user := range ev.Info.Users {
		fmt.Println(user.ID, user.Name)
		cb.userIDToName[user.ID] = user.Name
	}

	for _, channel := range ev.Info.Channels {
		cb.channelIDToName[channel.ID] = channel.Name
	}
}

func (cb *ChatBot) Store() store.Store {
	return cb.store
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
			fmt.Printf("%+v\n", ev)
			go cb.handleMessage(ev)

		case *slack.PresenceChangeEvent:
			name, _ := cb.NameForID(ev.User)
			cr := &ChatEventPresence{
				Status: ev.Presence,
				User: &ChatUser{
					bot:  cb,
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
				User: &ChatUser{
					bot:  cb,
					id:   ev.User,
					name: userName,
				},
				Channel: &ChatChannel{
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
				User: &ChatUser{
					bot:  cb,
					id:   ev.User,
					name: userName,
				},
				Channel: &ChatUser{
					bot:  cb,
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

func (cb *ChatBot) SendPrivately(user *ChatUser, threadTimestamp string, s string, args ...interface{}) (*ChatReply, error) {
	// FUUUUUUUUUUUUUUU
	// need to reach out via regular api in order to open a channel with user
	// it *might* be already open, but we don't care
	_, _, channelID, err := cb.slackAPI.OpenIMChannel(user.ID())
	if err != nil {
		fmt.Println("im.open", err)
		return nil, err
	}

	target := &ChatChannel{
		name: user.Name(),
		id:   channelID,
	}

	return cb.Send(target, threadTimestamp, s, args...)
}

func (cb *ChatBot) Send(target ChatTarget, threadTimestamp string, s string, args ...interface{}) (*ChatReply, error) {
	text := fmt.Sprintf(s, args...)

	cr := &ChatReply{
		Bot:    cb,
		Text:   text,
		Target: target,
	}

	msg := cb.slackRTM.NewOutgoingMessage(text, target.ID())
	msg.ThreadTimestamp = threadTimestamp

	ch := make(chan struct{})
	cr.bindCallback = func(ev *slack.AckMessage) {
		cr.Timestamp = ev.Timestamp
		close(ch)
	}

	cb.outgoingIDs.Store(msg.ID, cr)

	fmt.Printf(">> %s: %s\n", target.ID(), text)
	cb.slackRTM.SendMessage(msg)

	select {
	case <-ch:
		return cr, cr.bindErr
	case <-time.After(ackTimeout):
		fmt.Println("didn't ack msg on time")
	}

	return nil, errors.New("could not confirm msg was sent")
}

func (cb *ChatBot) ReactToMessage(msg *ChatMessage, reaction string) error {
	msgRef := slack.NewRefToMessage(msg.Channel.ID(), msg.Timestamp)
	return cb.slackAPI.AddReaction(reaction, msgRef)
}

func parseArguments(specArgs []chatArg, msg *ChatMessage) error {
	scanner := bufio.NewScanner(strings.NewReader(msg.RawArgs))
	scanner.Split(ScanQuotedWords)

	argStack := make([]chatArg, len(specArgs))
	copy(argStack, specArgs)
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
		return err
	}

	// apply optionals & fail defaults
	for _, arg := range argStack {
		if arg.required == true {
			return errors.New("missed required arg" + arg.name)
			continue // remove this
		}

		msg.Args[arg.name] = arg.defValue
	}

	return nil
}

func (cb *ChatBot) handleMessage(ev *slack.MessageEvent) {
	userName, _ := cb.NameForID(ev.User)
	userTarget := &ChatUser{
		id:   ev.User,
		name: userName,
	}

	channelName, _ := cb.NameForID(ev.Channel)
	// this is a direct message
	if ev.Channel[0] == 'D' {
		channelName = userName
	}

	channelTarget := &ChatChannel{
		id:   ev.Channel,
		name: channelName,
	}

	var handlers []*chatAction
	var rawArgs string
	var pattern string

	for p, ch := range cb.chatHandlers {
		if strings.HasPrefix(ev.Text, p) {
			handlers = ch
			pattern = p
			rawArgs = strings.TrimSpace(strings.TrimPrefix(ev.Text, p))
			break
		}
	}

	if len(handlers) == 0 {
		if cb.defaultHandler != nil {
			msg := &ChatMessage{
				Body:            ev.Text,
				Timestamp:       ev.Timestamp,
				ThreadTimestamp: ev.ThreadTimestamp,
				Bot:             cb,
				Args:            ChatArgs{},
				User:            userTarget,
				Channel:         channelTarget,
			}
			cb.handleError(msg, cb.defaultHandler.handler.OnChatMessage(msg))
		}

		return
	}

	for _, ca := range handlers {
		msg := &ChatMessage{
			Body:            ev.Text,
			RawArgs:         rawArgs,
			Match:           pattern,
			Timestamp:       ev.Timestamp,
			ThreadTimestamp: ev.ThreadTimestamp,
			Bot:             cb,
			Args:            ChatArgs{},
			User:            userTarget,
			Channel:         channelTarget,
		}

		if len(ca.args) > 0 {
			parseArguments(ca.args, msg)
		}

		cb.handleError(msg, ca.handler.OnChatMessage(msg))
	}
}

func (cb *ChatBot) handleError(msg *ChatMessage, err error) {
	fmt.Println("asdlfasdkjfhaskjldhfasdf")
	switch err {
	case nil:
		return
	case ErrUserAuthNeeded:
		msg.ReplyPrivately("Auth needed")
	default:
		msg.ReplyPrivately("Your last command emmited an error")
		msg.ReplyPrivately("%+v", err)
	}
}

func (cb *ChatBot) AddAuthHandler(site string, authHandler ChatAuthHandler) error {
	if _, ok := cb.authHandlers[site]; ok {
		return errors.New("site already present")
	}

	cb.authHandlers[site] = authHandler
	return nil
}

func (cb *ChatBot) AuthUser(user *ChatUser, site string, role string) (*ChatExternalUser, error) {
	handler, ok := cb.authHandlers[site]
	if !ok {
		return nil, errors.New("no handler for site")
	}

	ceu, err := handler(user, role)
	if err != nil {
		return nil, err
	}

	ceu.site = site
	return ceu, nil
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

	cb.chatHandlers[pattern] = append(cb.chatHandlers[pattern], ca)
	return nil
}

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
