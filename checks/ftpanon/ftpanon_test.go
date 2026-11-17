// Tests the FTP check against a small fake FTP server.

package ftpanon

import (
	"bufio"
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

// fakeFTP is a tiny scripted FTP server. greeting is sent on connect, then
// each command gets the reply from replies (by command name). It records
// every command it receives.
type fakeFTP struct {
	greeting string
	replies  map[string]string
	silent   bool // never send anything

	mu   sync.Mutex
	got  []string
	addr string
}

func startFTP(t *testing.T, f *fakeFTP) *fakeFTP {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
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

func (f *fakeFTP) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if f.silent {
		time.Sleep(3 * time.Second)
		return
	}
	conn.Write([]byte(f.greeting))
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		line := sc.Text()
		f.mu.Lock()
		f.got = append(f.got, line)
		f.mu.Unlock()
		cmd := strings.ToUpper(strings.Fields(line + " x")[0])
		reply, ok := f.replies[cmd]
		if !ok {
			reply = "500 unknown\r\n"
		}
		if reply == "CLOSE" {
			return
		}
		conn.Write([]byte(reply))
		if cmd == "QUIT" {
			return
		}
	}
}

func (f *fakeFTP) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.got...)
}

func (f *fakeFTP) target(t *testing.T) check.Target {
	t.Helper()
	host, port, _ := net.SplitHostPort(f.addr)
	p, _ := strconv.Atoi(port)
	return check.Target{Host: host, Service: model.Service{Port: p, Protocol: "tcp", Name: "ftp"}}
}

func run(t *testing.T, f *fakeFTP) (*check.Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return Check{}.Run(ctx, f.target(t))
}

// waitFor gives the fake server a moment to record the last command.
func waitFor(f *fakeFTP, cmd string) bool {
	for range 50 {
		for _, c := range f.commands() {
			if strings.HasPrefix(c, cmd) {
				return true
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func TestInfo(t *testing.T) {
	if err := check.Validate([]check.Check{Check{}}); err != nil {
		t.Fatal(err)
	}
	if !check.Applies(Check{}.Info(), model.Service{Port: 2121, Name: "ftp"}) || !check.Applies(Check{}.Info(), model.Service{Port: 21}) {
		t.Error("should apply to ftp by name and to port 21")
	}
}

func TestAnonymousAllowed(t *testing.T) {
	f := startFTP(t, &fakeFTP{
		greeting: "220 (vsFTPd 3.0.3)\r\n",
		replies:  map[string]string{"USER": "331 Please specify the password.\r\n", "PASS": "230 Login successful.\r\n", "QUIT": "221 Goodbye.\r\n"},
	})
	res, err := run(t, f)
	if err != nil || res == nil {
		t.Fatalf("expected a finding, got %v, %v", res, err)
	}
	if !strings.Contains(res.Evidence, "230 Login successful.") {
		t.Errorf("evidence should quote the server: %q", res.Evidence)
	}
	cmds := f.commands()
	if len(cmds) < 2 || cmds[0] != "USER anonymous" || cmds[1] != "PASS nuqs@example.com" {
		t.Errorf("unexpected commands: %q", cmds)
	}
	if !waitFor(f, "QUIT") {
		t.Error("the check should log out with QUIT")
	}
}

func TestLoggedInRightAfterUser(t *testing.T) {
	f := startFTP(t, &fakeFTP{greeting: "220 hi\r\n", replies: map[string]string{"USER": "230 Welcome.\r\n", "QUIT": "221 Bye\r\n"}})
	res, err := run(t, f)
	if err != nil || res == nil {
		t.Fatalf("230 after USER is a finding, got %v, %v", res, err)
	}
	for _, c := range f.commands() {
		if strings.HasPrefix(c, "PASS") {
			t.Error("no password should be sent when USER already logged in")
		}
	}
}

func TestAnonymousRefused(t *testing.T) {
	cases := map[string]*fakeFTP{
		"refused at PASS": {greeting: "220 hi\r\n", replies: map[string]string{"USER": "331 pass?\r\n", "PASS": "530 Login incorrect.\r\n", "QUIT": "221 Bye\r\n"}},
		"refused at USER": {greeting: "220 hi\r\n", replies: map[string]string{"USER": "530 This FTP server does not allow anonymous logins.\r\n", "QUIT": "221 Bye\r\n"}},
	}
	for name, f := range cases {
		res, err := run(t, startFTP(t, f))
		if err != nil || res != nil {
			t.Errorf("%s: expected clean result, got %+v, %v", name, res, err)
		}
	}
}

func TestMultilineGreeting(t *testing.T) {
	f := startFTP(t, &fakeFTP{
		greeting: "220-Welcome to the server\r\n220-Be nice\r\n220 Ready\r\n",
		replies:  map[string]string{"USER": "331 pass?\r\n", "PASS": "230 ok\r\n", "QUIT": "221 Bye\r\n"},
	})
	if res, err := run(t, f); err != nil || res == nil {
		t.Errorf("multi-line greetings must work, got %+v, %v", res, err)
	}
}

func TestBadServers(t *testing.T) {
	cases := map[string]*fakeFTP{
		"busy greeting":       {greeting: "421 Too many users\r\n"},
		"not ftp":             {greeting: "SSH-2.0-OpenSSH_9.6\r\n"},
		"closes after USER":   {greeting: "220 hi\r\n", replies: map[string]string{"USER": "CLOSE"}},
		"closes after PASS":   {greeting: "220 hi\r\n", replies: map[string]string{"USER": "331 pass?\r\n", "PASS": "CLOSE"}},
		"silent":              {silent: true},
		"huge garbage banner": {greeting: "220-" + strings.Repeat("A", 200<<10) + "\r\n"},
	}
	for name, f := range cases {
		start := time.Now()
		res, err := run(t, startFTP(t, f))
		if err == nil || res != nil {
			t.Errorf("%s: expected an error and no finding, got %+v, %v", name, res, err)
		}
		if time.Since(start) > 3*time.Second {
			t.Errorf("%s: took too long, the check must respect its timeout", name)
		}
	}
}

func TestConnectionRefused(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	f := &fakeFTP{addr: l.Addr().String()}
	l.Close()
	if _, err := run(t, f); err == nil {
		t.Error("closed port should be an error")
	}
}

func TestTLSNotSupported(t *testing.T) {
	_, err := Check{}.Run(context.Background(), check.Target{Host: "127.0.0.1", Service: model.Service{Port: 990, Name: "ftp", Tunnel: "ssl"}})
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		t.Errorf("expected a clear 'not supported' error, got %v", err)
	}
}

func TestNoDeadlineStillWorks(t *testing.T) {
	f := startFTP(t, &fakeFTP{greeting: "220 hi\r\n", replies: map[string]string{"USER": "530 no\r\n", "QUIT": "221 Bye\r\n"}})
	if res, err := (Check{}).Run(context.Background(), f.target(t)); err != nil || res != nil {
		t.Errorf("context without deadline: %+v, %v", res, err)
	}
}
