package driver

import (
	"net/http"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	trinoClient "github.com/trinodb/grafana-trino/pkg/trino/client"
	"github.com/trinodb/grafana-trino/pkg/trino/models"
)

// tokenMiddleware sends the signed-in user's forwarded OAuth token, or else the
// access token of the data source, like the basic auth middleware sends the
// password. trino-go-client refuses to send either over HTTP, while Trino
// accepts them there behind a proxy that terminates TLS, so it never sees
// them. Credentials set on the request already, the OAuth client token or
// Kerberos, take precedence, and no token goes to any host but Trino's.
func tokenMiddleware(settings models.TrinoDatasourceSettings) httpclient.Middleware {
	return httpclient.NamedMiddlewareFunc("trino-token", func(_ httpclient.Options, next http.RoundTripper) http.RoundTripper {
		return httpclient.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			token := trinoClient.ForwardedToken(req.Context())
			if token == "" {
				token = settings.AccessToken
			}
			if token == "" || req.Header.Get("Authorization") != "" || req.URL.Host != settings.URL.Host {
				return next.RoundTrip(req)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			return next.RoundTrip(req)
		})
	})
}
