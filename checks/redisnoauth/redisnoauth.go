// Package redisnoauth checks if a Redis server allows unauthenticated access.
//
// It sends a harmless PING command without credentials. If the server replies
// with +PONG, unauthenticated access is possible. If it replies with -NOAUTH,
// the server requires authentication and is not vulnerable.
package redisnoauth

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/nuqs-org/nuqs/internal/check"
	"github.com/nuqs-org/nuqs/internal/model"
)

// maxReply caps how much we read from the server so hostile servers cannot
// exhaust memory.
const maxReply = 256

// Check reports Redis servers with no password.
type Check struct{}

// Info describes the check.
func (Check) Info() check.Info {
	return check.Info{
		ID:       "redis-no-auth",
		Title:    "Redis server has no password",
		Severity: model.High,
		Fix:      "Set requirepass in redis.conf, or bind Redis to localhost only.",
		Ports:    []int{6379},
		Services: []string{"redis"},
	}
}

// Run sends PING and reports a finding if Redis answers with +PONG.
func (Check) Run(ctx context.Context, t check.Target) (*check.Result, error) {
	if t.Service.Tunnel == "ssl" {
		return nil, fmt.Errorf("redis over TLS isn't supported yet")
	}

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", t.Address())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	}

	// Close the connection if ctx is cancelled, which unblocks any pending I/O.
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	if _, err := conn.Write([]byte("PING\r\n")); err != nil {
		return nil, err
	}

	r := bufio.NewReader(io.LimitReader(conn, maxReply))
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}

	reply := strings.TrimRight(line, "\r\n")
	if reply == "+PONG" {
		return &check.Result{Evidence: "sent PING without a password and got +PONG"}, nil
	}
	if strings.HasPrefix(reply, "-NOAUTH") || strings.Contains(strings.ToLower(reply), "authentication required") {
		return nil, nil // server requires a password, no finding
	}

	return nil, fmt.Errorf("unexpected Redis reply: %q", reply)
}
