package pkg

import (
	"context"
	"time"

	"github.com/zizon/qsocks/pkg/internal"
)

// Socks5Config socks5 server config
type Socks5Config struct {
	Listen           string
	Connect          string
	Timeout          int
	StreamPerSession int
}

// SetLogLevel set the global log level
func SetLogLevel(level int) {
	internal.SetLogLevel(level)
}

// StartSocks5Server public export start interface for socks5 server
func StartSocks5Server(ctx context.Context, config Socks5Config) context.Context {
	return internal.StartSessionLimitedSocks5RaceServer(
		ctx,
		config.Listen,
		config.Connect,
		time.Duration(config.Timeout*int(time.Second)),
		config.StreamPerSession,
	)
}

// QuicConfig quic server config
type QuicConfig struct {
	Listen string
}

// StartQuicServer public export start interface for quic server
func StartQuicServer(ctx context.Context, config QuicConfig) context.Context {
	return internal.StartQuicServer(ctx, config.Listen)
}
