package httpheaders

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/PremadeS/nuqs/internal/check"
	"github.com/PremadeS/nuqs/internal/model"
)

// targetFor turns a test server URL into a check target.
func targetFor(t *testing.T, raw string, svc model.Service) check.Target {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	host, port, _ := net.SplitHostPort(u.Host)
	svc.Port, _ = strconv.Atoi(port)
	svc.Protocol = "tcp"
	if svc.Name == "" {
		svc.Name = "http"
	}
	return check.Target{Host: host, Service: svc}
}

// serve starts a test server that sends the given headers.
func serve(t *testing.T, tls bool, headers map[string]string) *httptest.Server {
	t.Helper()
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		for k, v := range headers {
			w.Header().Set(k, v)
		}
		w.Write([]byte("hello"))
	})
	var s *httptest.Server
	if tls {
		s = httptest.NewTLSServer(h)
	} else {
		s = httptest.NewServer(h)
	}
	t.Cleanup(s.Close)
	return s
}

var allHeaders = map[string]string{
	"Content-Security-Policy":   "default-src 'self'",
	"X-Content-Type-Options":    "nosniff",
	"X-Frame-Options":           "DENY",
	"Strict-Transport-Security": "max-age=31536000",
}

func TestInfo(t *testing.T) {
	if err := check.Validate([]check.Check{Check{}}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"http", "https", "http-alt", "http-proxy"} {
		if !check.Applies(Check{}.Info(), model.Service{Port: 1, Name: name}) {
			t.Errorf("should apply to %q", name)
		}
	}
	if check.Applies(Check{}.Info(), model.Service{Port: 80, Name: "ssh"}) {
		t.Error("should not apply by port alone")
	}
}

func TestMissingHeaders(t *testing.T) {
	s := serve(t, false, nil)
	res, err := Check{}.Run(context.Background(), targetFor(t, s.URL, model.Service{}))
	if err != nil || res == nil {
		t.Fatalf("expected a finding, got %v, %v", res, err)
	}
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options"} {
		if !strings.Contains(res.Evidence, h) {
			t.Errorf("evidence should list %s: %q", h, res.Evidence)
		}
	}
	if strings.Contains(res.Evidence, "Strict-Transport-Security") {
		t.Error("HSTS only matters over HTTPS")
	}
	if !strings.Contains(res.Evidence, "returned 200") {
		t.Errorf("evidence should include the status code: %q", res.Evidence)
	}
}

func TestAllHeadersPresent(t *testing.T) {
	s := serve(t, false, allHeaders)
	res, err := Check{}.Run(context.Background(), targetFor(t, s.URL, model.Service{}))
	if err != nil || res != nil {
		t.Errorf("expected no finding, got %+v, %v", res, err)
	}
}

func TestPartialHeaders(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		missing string
	}{
		{"wrong nosniff value", map[string]string{"Content-Security-Policy": "default-src 'self'", "X-Content-Type-Options": "yes", "X-Frame-Options": "DENY"}, "X-Content-Type-Options"},
		{"no frame protection", map[string]string{"Content-Security-Policy": "default-src 'self'", "X-Content-Type-Options": "nosniff"}, "X-Frame-Options"},
		{"empty CSP", map[string]string{"Content-Security-Policy": " ", "X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY"}, "Content-Security-Policy"},
	}
	for _, c := range cases {
		s := serve(t, false, c.headers)
		res, err := Check{}.Run(context.Background(), targetFor(t, s.URL, model.Service{}))
		if err != nil || res == nil || !strings.Contains(res.Evidence, c.missing) {
			t.Errorf("%s: expected %s to be reported, got %+v, %v", c.name, c.missing, res, err)
		}
	}
}

func TestFrameAncestorsCountsAsFrameProtection(t *testing.T) {
	s := serve(t, false, map[string]string{"Content-Security-Policy": "frame-ancestors 'none'", "X-Content-Type-Options": "NoSniff"})
	res, err := Check{}.Run(context.Background(), targetFor(t, s.URL, model.Service{}))
	if err != nil || res != nil {
		t.Errorf("CSP frame-ancestors should satisfy X-Frame-Options, got %+v, %v", res, err)
	}
}

func TestHTTPSNeedsHSTS(t *testing.T) {
	noHSTS := map[string]string{"Content-Security-Policy": "a", "X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY"}
	s := serve(t, true, noHSTS)

	// Nmap reports TLS services as name "http" with tunnel "ssl".
	res, err := Check{}.Run(context.Background(), targetFor(t, s.URL, model.Service{Name: "http", Tunnel: "ssl"}))
	if err != nil || res == nil || !strings.Contains(res.Evidence, "Strict-Transport-Security") || !strings.Contains(res.Evidence, "https://") {
		t.Fatalf("expected missing HSTS over https, got %+v, %v", res, err)
	}

	// Some services are just called "https".
	res, err = Check{}.Run(context.Background(), targetFor(t, s.URL, model.Service{Name: "https"}))
	if err != nil || res == nil {
		t.Errorf("name https should use TLS, got %+v, %v", res, err)
	}

	full := serve(t, true, allHeaders)
	res, err = Check{}.Run(context.Background(), targetFor(t, full.URL, model.Service{Tunnel: "ssl"}))
	if err != nil || res != nil {
		t.Errorf("all headers over TLS should be clean, got %+v, %v", res, err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	good := serve(t, false, allHeaders)
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, good.URL, http.StatusFound)
	}))
	defer redirect.Close()
	res, err := Check{}.Run(context.Background(), targetFor(t, redirect.URL, model.Service{}))
	if err != nil || res == nil || !strings.Contains(res.Evidence, "returned 302") {
		t.Errorf("we judge the server we were pointed at, not where it redirects: %+v, %v", res, err)
	}
}

func TestUnreachable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close() // nothing listens here now
	_, err = Check{}.Run(context.Background(), targetFor(t, "http://"+addr, model.Service{}))
	if err == nil {
		t.Error("a closed port should be an error, not a clean result")
	}
}

func TestRespectsContext(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Check{}.Run(ctx, targetFor(t, slow.URL, model.Service{}))
	if err == nil || time.Since(start) > 3*time.Second {
		t.Errorf("check should give up when ctx ends, err=%v after %s", err, time.Since(start))
	}
}

func TestCustomClient(t *testing.T) {
	s := serve(t, false, allHeaders)
	used := false
	c := Check{Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		used = true
		return http.DefaultTransport.RoundTrip(r)
	})}}
	if _, err := c.Run(context.Background(), targetFor(t, s.URL, model.Service{})); err != nil || !used {
		t.Errorf("custom client not used: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
