package client

import (
	"time"
)

// UntilExpirationInSeconds time has to be a greater than HTTP request timeout
// In most HTTP clients HTTP request timeout is 30 seconds.
const UntilExpirationInSeconds = 60

type Token struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	ExpiresAt   time.Time
}

func (token *Token) isAlmostExpired() bool {
	if token.AccessToken == "" {
		return true
	} else {
		if time.Now().Add(time.Second * time.Duration(UntilExpirationInSeconds)).After(token.ExpiresAt) {
			return true
		}
	}
	return false
}
