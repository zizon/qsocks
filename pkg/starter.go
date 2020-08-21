package pkg

import (
	"context"

	"github.com/zizon/qsocks/pkg/internal"
)

type Socks5Config struct {
	Listen  string
	Connect string
}

func StartSocks5Server(ctx context.Context, config Socks5Config) {
	internal.StartSocks5Server(ctx, config.Listen, config.Connect)
}

type QuicConfig struct {
	Listen string
}

func StartQuicServer(ctx context.Context, config QuicConfig) {
	internal.StartQuicServer(ctx, config.Listen)
}
