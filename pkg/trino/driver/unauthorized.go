package driver

import (
	"io"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

// maxUnauthorizedReasonBytes bounds how much of a 401 body is kept, like
// trino-go-client does when it quotes the body in its error.
const maxUnauthorizedReasonBytes = 8 * 1024

// unauthorizedMiddleware adds to the body of a 401 response which of the data
// source's credentials Trino rejected, since trino-go-client quotes that body
// in its error. Trino only says why, when it says anything: a password sent to
// a coordinator without password authentication is "not allowed", which does
// not tell the user to remove it from the data source.
func unauthorizedMiddleware() httpclient.Middleware {
	return httpclient.NamedMiddlewareFunc("trino-unauthorized", func(_ httpclient.Options, next http.RoundTripper) http.RoundTripper {
		return httpclient.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			resp, err := next.RoundTrip(req)
			if err != nil || resp.StatusCode != http.StatusUnauthorized {
				return resp, err
			}
			reason, err := io.ReadAll(io.LimitReader(resp.Body, maxUnauthorizedReasonBytes))
			_ = resp.Body.Close()
			if err != nil {
				return nil, err
			}
			explanation := explainUnauthorized(req.Header.Get("Authorization"), string(reason))
			resp.Body = io.NopCloser(strings.NewReader(explanation))
			resp.ContentLength = int64(len(explanation))
			return resp, nil
		})
	})
}

func explainUnauthorized(authorization, reason string) string {
	scheme, _, _ := strings.Cut(authorization, " ")
	var rejected string
	switch strings.ToLower(scheme) {
	case "basic":
		rejected = "Trino rejected the basic auth password of the data source: correct it, or remove it if Trino does not authenticate with passwords"
	case "bearer":
		rejected = "Trino rejected the bearer token: the access token of the data source, the token from its OAuth client credentials, or the forwarded OAuth identity"
	case "negotiate":
		rejected = "Trino rejected the Kerberos credentials of the data source"
	default:
		rejected = "Trino requires authentication, but the data source sent no credentials"
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return rejected
	}
	return strings.TrimSuffix(reason, ".") + ". " + rejected
}
