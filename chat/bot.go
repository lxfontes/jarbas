package chat

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lxfontes/jarbas/logger"
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

type ChatExternalUser interface {
	Site() string
	Name() string
	ID() string
	Token() string
}

var ErrUserAuthNeeded = errors.New("need auth for site")

type directory struct {
	// keeps an in-memory representation of our workspace
	channelIDToName map[string]string
	userIDToName    map[string]string
	slackAPI        *slack.Client
	mtx             sync.RWMutex
}

func newDirectory(slackAPI *slack.Client) *directory {
	return &directory{
		slackAPI:        slackAPI,
		channelIDToName: map[string]string{},
		userIDToName:    map[string]string{},
	}
}

func (d *directory) setup(ev *slack.ConnectedEvent) {
	d.mtx.Lock()
	defer d.mtx.Unlock()

	d.channelIDToName = map[string]string{}
	d.userIDToName = map[string]string{}

	for _, user := range ev.Info.Users {
		d.userIDToName[user.ID] = user.Name
	}

	for _, channel := range ev.Info.Channels {
		d.channelIDToName[channel.ID] = channel.Name
	}
}

func (d *directory) userForID(id string) (string, bool) {
	d.mtx.RLock()
	defer d.mtx.RUnlock()

	name, ok := d.userIDToName[id]
	return name, ok
}

func (d *directory) channelForID(id string) (string, bool) {
	d.mtx.RLock()
	defer d.mtx.RUnlock()

	name, ok := d.channelIDToName[id]
	if ok {
		return name, ok
	}

	return d.userForID(id)
}

type ChatAuthHandler interface {
	Authorize(user *ChatUser, role string) (ChatExternalUser, error)
	Name() string
}

type ChatBot struct {
	chatHandlers   map[string][]*chatAction // indexed by command, ex: 'say'
	eventHandlers  map[string][]ChatEventHandler
	authHandlers   map[string]ChatAuthHandler
	defaultHandler *chatAction
	errorHander    *ChatErrorHandler

	slackAPI *slack.Client
	slackRTM *slack.RTM

	outgoingIDs sync.Map // used to track outgoing message timestamps (ChatReply)
	directory   *directory

	store  store.Store
	logger logger.Log
}

func NewChatBot(token string) (*ChatBot, error) {
	apiClient := slack.New(token)
	return &ChatBot{
		chatHandlers:  map[string][]*chatAction{},
		eventHandlers: map[string][]ChatEventHandler{},
		authHandlers:  map[string]ChatAuthHandler{},
		slackAPI:      apiClient,
		store:         store.NewMemoryStore(),
		logger:        logger.DefaultLogger(),
		directory:     newDirectory(apiClient),
	}, nil
}

func (cb *ChatBot) Store() store.Store {
	return cb.store
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
			cb.directory.setup(ev)
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
			name, _ := cb.directory.userForID(ev.User)
			cr := &ChatEventPresence{
				Status: ev.Presence,
				User:   cb.userFor(ev.User, name),
			}
			go cb.emitEvent(EventPresence, cr)

		case *slack.LatencyReport:
			cb.Logger().WithField("latency", ev.Value).Info("latency report")

		case *slack.RTMError:
			cb.Logger().WithError(ev).Error("rtm error")

		case *slack.InvalidAuthEvent:
			cb.Logger().Error("invalid credentials")
			return

		case *slack.ReactionAddedEvent:
			userName, _ := cb.directory.userForID(ev.User)
			channelName, _ := cb.directory.channelForID(ev.Item.Channel)
			cr := &ChatEventReaction{
				Timestamp: ev.Item.Timestamp,
				Reaction:  ev.Reaction,
				User:      cb.userFor(ev.User, userName),
				Channel: &ChatChannel{
					id:   ev.Item.Channel,
					name: channelName,
				},
			}
			go cb.emitEvent(EventReaction, cr)

		case *slack.ReactionRemovedEvent:
			userName, _ := cb.directory.userForID(ev.User)
			channelName, _ := cb.directory.channelForID(ev.Item.Channel)
			cr := &ChatEventReaction{
				Timestamp: ev.Item.Timestamp,
				Reaction:  ev.Reaction,
				Removed:   true,
				User:      cb.userFor(ev.User, userName),
				Channel: &ChatChannel{
					id:   ev.Item.Channel,
					name: channelName,
				},
			}
			go cb.emitEvent(EventReaction, cr)

		case *slack.AckMessage:
			// map our internal id to a slack timestamp
			item, ok := cb.outgoingIDs.Load(ev.ReplyTo)
			if !ok {
				cb.Logger().WithField("message_id", ev.ReplyTo).Warning("received ack for unknown")
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

func (cb *ChatBot) Logger() logger.Log {
	return cb.logger
}

func (cb *ChatBot) SendPrivately(user *ChatUser, threadTimestamp string, s string, args ...interface{}) (*ChatReply, error) {
	// FUUUUUUUUUUUUUUU
	// need to reach out via regular api in order to open a channel with user
	// it *might* be already open, but we don't care
	_, _, channelID, err := cb.slackAPI.OpenIMChannel(user.ID())
	if err != nil {
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

	ll := cb.Logger().WithField("target_id", target.ID()).WithField("thread", threadTimestamp).WithField("text", text)
	ll.Debug("outgoing message")
	cb.slackRTM.SendMessage(msg)

	select {
	case <-ch:
		return cr, cr.bindErr
	case <-time.After(ackTimeout):
		ll.Error("did not ack message")
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
				msg.Logger.WithField("args", msg.RawArgs).Error("switching from positional to name not allowed")
				return errors.New("mixed argument mode")
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

func (cb *ChatBot) userFor(id string, name string) *ChatUser {
	return &ChatUser{
		ll:   cb.Logger(),
		bot:  cb,
		id:   id,
		name: name,
	}
}

func (cb *ChatBot) handleMessage(ev *slack.MessageEvent) {
	isPrivate := false

	userName, _ := cb.directory.userForID(ev.User)
	userTarget := cb.userFor(ev.User, userName)

	channelName, _ := cb.directory.channelForID(ev.Channel)

	// this is a direct message
	if ev.Channel[0] == 'D' {
		isPrivate = true
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

	ll := cb.Logger().
		WithField("from", userTarget.Name()).
		WithField("channel", channelTarget.Name()).
		WithField("text", ev.Text)

	ll.Info("incoming message")

	if len(handlers) == 0 {
		if cb.defaultHandler != nil {
			msg := &ChatMessage{
				Logger:          ll,
				Body:            ev.Text,
				Timestamp:       ev.Timestamp,
				ThreadTimestamp: ev.ThreadTimestamp,
				Bot:             cb,
				IsPrivate:       isPrivate,
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
			Logger:          ll,
			Body:            ev.Text,
			RawArgs:         rawArgs,
			Match:           pattern,
			Timestamp:       ev.Timestamp,
			ThreadTimestamp: ev.ThreadTimestamp,
			Bot:             cb,
			IsPrivate:       isPrivate,
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

func (cb *ChatBot) AddAuthHandler(authHandler ChatAuthHandler) error {
	if _, ok := cb.authHandlers[authHandler.Name()]; ok {
		return errors.New("site already present")
	}

	cb.authHandlers[authHandler.Name()] = authHandler
	return nil
}

func (cb *ChatBot) AuthorizeUser(user *ChatUser, site string, role string) (ChatExternalUser, error) {
	handler, ok := cb.authHandlers[site]
	if !ok {
		return nil, errors.New("no handler for site")
	}

	return handler.Authorize(user, role)
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
