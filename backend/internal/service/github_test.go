package service

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	myoauth "sentinelgo/pkg/oauth"
)

// ghRouteTransport redirects github.com / api.github.com requests to the test server.
type ghRouteTransport struct{ target string }

func (r ghRouteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, _ := url.Parse(r.target)
	u := *req.URL
	u.Scheme = target.Scheme
	u.Host = target.Host
	req.URL = &u
	return http.DefaultTransport.RoundTrip(req)
}

func newGitHubTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"gh_token_123","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), "gh_token_123") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":99999,"login":"ghuser","name":"GH User","avatar_url":"http://x/a.png"}`))
	})
	return httptest.NewServer(mux)
}

func newTestGitHubOAuth(serverURL string) *myoauth.GitHubOAuth {
	g := myoauth.NewGitHubOAuth("test", "test", "http://localhost/callback", "http://localhost:3001", "")
	cl := &http.Client{Transport: ghRouteTransport{target: serverURL}}
	g.SetHTTPClient(cl)
	return g
}
