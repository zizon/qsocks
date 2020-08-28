package pkg

import (
	"context"

	"github.com/zizon/qsocks/pkg/internal"
)

// Socks5Config socks5 server config
type Socks5Config struct {
	Listen  string
	Connect string
}

// SetLogLevel set the global log level
func SetLogLevel(level int) {
	internal.SetLogLevel(level)
}

// StartSocks5Server public export start interface for socks5 server
func StartSocks5Server(ctx context.Context, config Socks5Config) context.Context {
	return internal.StartSocks5RaceServer(ctx, config.Listen, config.Connect)
}

// QuicConfig quic server config
type QuicConfig struct {
	Listen string
}

// StartQuicServer public export start interface for quic server
func StartQuicServer(ctx context.Context, config QuicConfig) context.Context {
	return internal.StartQuicServer(ctx, config.Listen)
}
