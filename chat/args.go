package chat

import (
	"fmt"
	"strconv"
	"strings"
)

type ChatArgs map[string]string

func (ca ChatArgs) String(parameter string) (string, bool) {
	s, ok := ca[parameter]
	return s, ok
}

func (ca ChatArgs) Keys() []string {
	ret := []string{}
	for k, _ := range ca {
		ret = append(ret, k)
	}

	return ret
}

func (ca ChatArgs) Int(parameter string) (int, bool) {
	v, ok := ca.String(parameter)
	i, err := strconv.Atoi(v)
	if err != nil {
		fmt.Println("seriously?", err)
		return 0, false
	}

	return i, ok
}

func (ca ChatArgs) Bool(parameter string) (bool, bool) {
	v, ok := ca.String(parameter)
	if strings.TrimSpace(strings.ToLower(v)) == "true" {
		return true, ok
	}

	return false, ok
}

func (ca ChatArgs) Inclusion(parameter string, validValues ...string) (string, bool) {
	rawValue, ok := ca.String(parameter)
	if !ok {
		return "", false
	}

	for _, vv := range validValues {
		if rawValue == vv {
			return vv, true
		}
	}

	return "", false
}
