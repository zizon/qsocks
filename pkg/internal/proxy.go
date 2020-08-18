package internal

import (
	"net"
)

type Proxy interface {
	Accept() (net.Conn, error)
	Connect() (net.Conn, error)
}

func StartProxy(ctx CanclableContext, proxy Proxy) CanclableContext {
	proxyCtx := ctx.Derive(nil)

	go func() {
		from, err := proxy.Accept()
		if err != nil {
			proxyCtx.CancleWithError(err)
			return
		}
		proxyCtx.Cleanup(from.Close)

		to, err := proxy.Connect()
		if err != nil {
			proxyCtx.CancleWithError(err)
			return
		}
		proxyCtx.Cleanup(to.Close)

		BiCopy(proxyCtx, from, to).Cleanup(func() error {
			proxyCtx.Cancle()
			return nil
		})
	}()

	return proxyCtx
}
