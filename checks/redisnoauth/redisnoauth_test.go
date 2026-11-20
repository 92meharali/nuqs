// Tests the Redis unauthenticated access check against fake Redis servers.

package redisnoauth

import (
	"context"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nuqs-org/nuqs/internal/check"
	"github.com/nuqs-org/nuqs/internal/model"
)

// fakeRedis is a small scripted Redis server for testing.
type fakeRedis struct {
	reply      string
	silent     bool
	closeEarly bool

	mu   sync.Mutex
	got  []string
	addr string
}

func startRedis(t *testing.T, f *fakeRedis) *fakeRedis {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	f.addr = l.Addr().String()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go f.handle(conn)
		}
	}()
	return f
}

func (f *fakeRedis) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if f.closeEarly {
		return
	}
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	f.mu.Lock()
	f.got = append(f.got, string(buf[:n]))
	f.mu.Unlock()

	if f.silent {
		// Read until the client closes the connection or deadline expires.
		_, _ = conn.Read(buf)
		return
	}
	_, _ = conn.Write([]byte(f.reply))
}

func (f *fakeRedis) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.got...)
}

func (f *fakeRedis) target(t *testing.T) check.Target {
	t.Helper()
	host, port, _ := net.SplitHostPort(f.addr)
	p, _ := strconv.Atoi(port)
	return check.Target{Host: host, Service: model.Service{Port: p, Protocol: "tcp", Name: "redis"}}
}

func run(t *testing.T, f *fakeRedis) (*check.Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return Check{}.Run(ctx, f.target(t))
}

func TestInfo(t *testing.T) {
	in := Check{}.Info()
	if in.ID != "redis-no-auth" {
		t.Errorf("expected ID %q, got %q", "redis-no-auth", in.ID)
	}
	if in.Title != "Redis server has no password" {
		t.Errorf("expected Title %q, got %q", "Redis server has no password", in.Title)
	}
	if in.Severity != model.High {
		t.Errorf("expected Severity High, got %v", in.Severity)
	}
	if in.Fix != "Set requirepass in redis.conf, or bind Redis to localhost only." {
		t.Errorf("unexpected Fix: %q", in.Fix)
	}
	if !check.Applies(in, model.Service{Port: 6379, Name: "redis"}) {
		t.Error("should apply to redis on port 6379")
	}
	if !check.Applies(in, model.Service{Port: 6379}) {
		t.Error("should apply to port 6379 regardless of service name")
	}
	if !check.Applies(in, model.Service{Port: 16379, Name: "redis"}) {
		t.Error("should apply to redis on non-standard port")
	}
	if check.Applies(in, model.Service{Port: 80, Name: "http"}) {
		t.Error("should not apply to unrelated service")
	}
}

func TestRegistration(t *testing.T) {
	var c check.Check = Check{}
	if err := check.Validate([]check.Check{c}); err != nil {
		t.Fatalf("check failed validation: %v", err)
	}
}

func TestPositive(t *testing.T) {
	f := startRedis(t, &fakeRedis{reply: "+PONG\r\n"})
	res, err := run(t, f)
	if err != nil || res == nil {
		t.Fatalf("expected a finding, got res=%v, err=%v", res, err)
	}
	expectedEvidence := "sent PING without a password and got +PONG"
	if res.Evidence != expectedEvidence {
		t.Errorf("expected evidence %q, got %q", expectedEvidence, res.Evidence)
	}
	cmds := f.commands()
	if len(cmds) == 0 || cmds[0] != "PING\r\n" {
		t.Errorf("expected PING\\r\\n sent to server, got %q", cmds)
	}
}

func TestNegative(t *testing.T) {
	cases := map[string]string{
		"standard noauth":   "-NOAUTH Authentication required.\r\n",
		"bare noauth":       "-NOAUTH\r\n",
		"auth required err": "-ERR operation not permitted, authentication required\r\n",
		"protected mode":    "-DENIED Redis is running in protected mode\r\n",
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			f := startRedis(t, &fakeRedis{reply: reply})
			res, err := run(t, f)
			if err != nil || res != nil {
				t.Errorf("expected clean result (no finding, no error), got res=%+v, err=%v", res, err)
			}
		})
	}
}

func TestServerDown(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close() // closed immediately, nothing listening

	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(port)
	tgt := check.Target{Host: host, Service: model.Service{Port: p, Protocol: "tcp", Name: "redis"}}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := Check{}.Run(ctx, tgt)
	if err == nil || res != nil {
		t.Fatalf("expected error for closed port, got res=%+v, err=%v", res, err)
	}
}

func TestGarbageResponse(t *testing.T) {
	cases := map[string]string{
		"ssh banner":        "SSH-2.0-OpenSSH_9.6\r\n",
		"http response":     "HTTP/1.1 400 Bad Request\r\n",
		"random text":       "SOME_GARBAGE_DATA\r\n",
		"redis generic err": "-ERR unknown command 'PING'\r\n",
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			f := startRedis(t, &fakeRedis{reply: reply})
			res, err := run(t, f)
			if err == nil || res != nil {
				t.Errorf("%s: expected error and no finding, got res=%+v, err=%v", name, res, err)
			}
		})
	}
}

func TestSlowServer(t *testing.T) {
	f := startRedis(t, &fakeRedis{silent: true})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	res, err := Check{}.Run(ctx, f.target(t))
	if err == nil || res != nil {
		t.Fatalf("expected timeout error and no finding, got res=%+v, err=%v", res, err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("check took %v, should have aborted quickly on context timeout", elapsed)
	}
}

func TestPartialOrIncorrectPONG(t *testing.T) {
	cases := []string{
		"+PON",
		"+PON\r\n",
		"+POMG",
		"+POMG\r\n",
		"+PONGextra\r\n",
		"+",
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			f := startRedis(t, &fakeRedis{reply: tc})
			res, err := run(t, f)
			if err == nil || res != nil {
				t.Errorf("reply %q: expected error and no finding, got res=%+v, err=%v", tc, res, err)
			}
		})
	}
}

func TestFragmentedPONG(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		_, _ = conn.Read(buf) // read PING\r\n
		_, _ = conn.Write([]byte("+PO"))
		time.Sleep(20 * time.Millisecond)
		_, _ = conn.Write([]byte("NG\r\n"))
	}()

	host, port, _ := net.SplitHostPort(l.Addr().String())
	p, _ := strconv.Atoi(port)
	tgt := check.Target{Host: host, Service: model.Service{Port: p, Protocol: "tcp", Name: "redis"}}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := Check{}.Run(ctx, tgt)
	if err != nil || res == nil {
		t.Fatalf("expected finding on fragmented PONG, got res=%v, err=%v", res, err)
	}
	expectedEvidence := "sent PING without a password and got +PONG"
	if res.Evidence != expectedEvidence {
		t.Errorf("expected evidence %q, got %q", expectedEvidence, res.Evidence)
	}
}

func TestTLSNotSupported(t *testing.T) {
	_, err := Check{}.Run(context.Background(), check.Target{
		Host:    "127.0.0.1",
		Service: model.Service{Port: 6379, Name: "redis", Tunnel: "ssl"},
	})
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Errorf("expected TLS not supported error, got %v", err)
	}
}

func TestNoDeadlineStillWorks(t *testing.T) {
	f := startRedis(t, &fakeRedis{reply: "-NOAUTH\r\n"})
	res, err := Check{}.Run(context.Background(), f.target(t))
	if err != nil || res != nil {
		t.Errorf("context without deadline: got res=%+v, err=%v", res, err)
	}
}

func TestEarlyClose(t *testing.T) {
	f := startRedis(t, &fakeRedis{closeEarly: true})
	res, err := run(t, f)
	if err == nil || res != nil {
		t.Errorf("server closing immediately: expected error, got res=%+v, err=%v", res, err)
	}
}
