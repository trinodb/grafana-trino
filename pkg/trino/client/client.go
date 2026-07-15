package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Client struct {
	*http.Client
	ClientId          string
	ClientSecret      string
	Url               string
	ImpersonationUser string

	// lock and token are per-instance so that concurrent datasource instances
	// with different credentials never share or clobber each other's token.
	lock  sync.Mutex
	token *Token
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if c.ImpersonationUser != "" {
		req.Header.Set("X-Trino-User", c.ImpersonationUser)
	}
	return c.Client.Do(req)
}

func (c *Client) getToken() (*Token, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.token != nil && !c.token.isAlmostExpired() {
		return c.token, nil
	}
	newToken, err := c.retrieveToken()
	if err != nil {
		return nil, err
	}
	c.token = newToken
	return c.token, nil
}

func (c *Client) retrieveToken() (*Token, error) {
	log.DefaultLogger.Debug("Try retrieve token")
	values := url.Values{
		"client_id":     []string{c.ClientId},
		"client_secret": []string{c.ClientSecret},
		"grant_type":    []string{"client_credentials"},
	}

	token := &Token{}
	response, err := c.PostForm(c.Url, values)
	if err != nil {
		return nil, fmt.Errorf("failed to request the token response: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, errors.New("Cannot obtain token from IDP. Status code=" + response.Status)
	}
	var jsonResponse []byte
	if jsonResponse, err = io.ReadAll(response.Body); err != nil {
		return nil, fmt.Errorf("failed to read the token response: %w", err)
	}
	err = json.Unmarshal(jsonResponse, token)
	if err != nil {
		return nil, fmt.Errorf("failed to decode the token response: %w", err)
	}
	token.ExpiresAt = time.Now().Add(time.Second * time.Duration(token.ExpiresIn))
	log.DefaultLogger.Debug("Token will expire at:", "date", token.ExpiresAt.Format(time.RFC1123Z))
	return token, nil
}
