package chat

type chatAction struct {
	handler ChatMessageHandler
	private bool
	mention bool
	args    []chatArg
}

type chatOpt func(*chatAction)

func WithMention() chatOpt {
	return func(ca *chatAction) {
		ca.mention = true
	}
}

func WithPrivateMessage() chatOpt {
	return func(ca *chatAction) {
		ca.private = true
	}
}

func WithOptionalArg(param string, defValue string, description string) chatOpt {
	return func(ca *chatAction) {
		arg := chatArg{
			name:        param,
			defValue:    defValue,
			description: description,
		}

		ca.args = append(ca.args, arg)
	}
}

func WithRequiredArg(param string, description string) chatOpt {
	return func(ca *chatAction) {
		arg := chatArg{
			name:        param,
			required:    true,
			description: description,
		}

		ca.args = append(ca.args, arg)
	}
}
