package common

import (
	"strings"
	"sync"
)

const InternalProvisionTokenOptionKey = "InternalProvisionToken"

type internalProvisionTokenState struct {
	Token  string
	Source string
}

var (
	internalProvisionTokenMu sync.RWMutex
	internalProvisionToken   internalProvisionTokenState
)

func SetInternalProvisionToken(token, source string) {
	internalProvisionTokenMu.Lock()
	internalProvisionToken = internalProvisionTokenState{
		Token:  strings.TrimSpace(token),
		Source: source,
	}
	internalProvisionTokenMu.Unlock()
}

func GetInternalProvisionToken() (token, source string) {
	internalProvisionTokenMu.RLock()
	defer internalProvisionTokenMu.RUnlock()
	return internalProvisionToken.Token, internalProvisionToken.Source
}
