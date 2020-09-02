package internal

import (
	"context"
	"fmt"
	"testing"
)

func TestHttp(t *testing.T) {
	SetLogLevel(debug)
	/*
		SetDefaultCollector(func(err error) {
			fmt.Printf("err %v type:%v\n", err, reflect.TypeOf(err))
			defaultCollector(err)
		})
	*/
	server := "localhost:10087"
	local := "localhost:10088"
	rootCtx := NewCanclableContext(context.TODO())

	httpListen := "localhost:10089"
	go SetupProxyTarget(rootCtx, httpListen, t)
	go httpProxy(httpBundle{
		rootCtx.Derive(nil),
		server,
	})

	go StartSocks5RaceServer(rootCtx, local, fmt.Sprintf("http://%s", server), 0)

	StartClientTest(rootCtx, local, httpListen, t, 1)
}
