package client

import (
	"context"
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

// Client authenticates requests to Trino with a token from the OAuth client
// credentials flow. It sends the requests, and the token requests, through
// the embedded client's transport, and adds the token only to requests to
// TrinoHost, so that a redirect never carries it to another host.
type Client struct {
	*http.Client
	ClientId          string
	ClientSecret      string
	Url               string
	ImpersonationUser string
	TrinoHost         string

	// lock and token are per-instance so that concurrent datasource instances
	// with different credentials never share or clobber each other's token.
	lock  sync.Mutex
	token *Token
}

type sessionUserKey struct{}

func WithSessionUser(ctx context.Context) context.Context {
	return context.WithValue(ctx, sessionUserKey{}, true)
}

func hasSessionUser(ctx context.Context) bool {
	set, _ := ctx.Value(sessionUserKey{}).(bool)
	return set
}

type forwardedTokenKey struct{}

// WithForwardedToken passes the signed-in user's OAuth token to every request
// of the queries run with ctx.
func WithForwardedToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, forwardedTokenKey{}, token)
}

func ForwardedToken(ctx context.Context) string {
	token, _ := ctx.Value(forwardedTokenKey{}).(string)
	return token
}

func (c *Client) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported Trino URL scheme %q", req.URL.Scheme)
	}
	if req.URL.Host == "" {
		return nil, errors.New("Trino URL must include a host")
	}
	if req.URL.Host != c.TrinoHost {
		return c.transport().RoundTrip(req)
	}

	token, err := c.getToken()
	if err != nil {
		return nil, err
	}
	// A RoundTripper must not modify the request it was given.
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	if c.ImpersonationUser != "" && !hasSessionUser(req.Context()) {
		req.Header.Set("X-Trino-User", c.ImpersonationUser)
	}
	return c.transport().RoundTrip(req)
}

func (c *Client) transport() http.RoundTripper {
	if c.Client.Transport == nil {
		return http.DefaultTransport
	}
	return c.Client.Transport
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
