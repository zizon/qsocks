package internal

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync"
	"testing"
)

func TestSockstConnection(t *testing.T) {
	root := NewCanclableContext(context.TODO())

	ctx := root.Derive(func(err error) {
		//fmt.Printf("err %v type:%v\n", err, reflect.TypeOf(err))
		//defaultCollector(err)
	})

	listen := "localhost:10087"
	remote := "localhost:10088"
	httpListen := "localhost:10089"

	statusExpect := 204

	httpl, err := net.Listen("tcp", httpListen)
	if err != nil {
		t.Error(err)
	}
	ctx.Cleanup(httpl.Close)
	go func() {
		if err := http.Serve(httpl, http.HandlerFunc(func(resp http.ResponseWriter, request *http.Request) {
			LogDebug("receive http request %v", request)
			resp.WriteHeader(statusExpect)
		})); err != nil {
			t.Error(err)
		}
	}()

	go socks5Server(socks5ServerBundle{
		ctx,
		listen,
		remote,
	})

	go quicServer(quicServerBundle{
		ctx,
		remote,
	})

	proxyURL, err := url.Parse(fmt.Sprintf("socks5://%s", listen))
	if err != nil {
		t.Error(err)
	}
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	wg := &sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		concurrent(ctx, client, statusExpect, t, httpListen, wg)
	}
	wg.Wait()
	ctx.Cancle()
}

func concurrent(parent CanclableContext, client http.Client, statusExpect int, t *testing.T, httpListen string, wg *sync.WaitGroup) {
	ctx := parent.Derive(nil)

	wg.Add(1)
	ctx.Cleanup(func() error {
		wg.Done()
		return nil
	})
	go func() {
		defer ctx.Cancle()
		httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				LogDebug("GotConnInfo %v", info.Conn)
			},
			GetConn: func(hostPort string) {
				LogDebug("GetConn %v", hostPort)
			},
			ConnectStart: func(network, addr string) {
				LogDebug("ConnectStart %v %v\n", network, addr)
			},

			GotFirstResponseByte: func() {
				LogDebug("receive some")
			},
		})
		request, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s", httpListen), nil)
		if err != nil {
			ctx.CancleWithError(err)
			return
		}

		LogDebug("send http request:%v", request)
		resp, err := client.Do(request)

		LogDebug("response:%v", resp)
		if err != nil {
			ctx.CancleWithError(err)
			return
		} else if resp.StatusCode != statusExpect {
			ctx.CancleWithError(fmt.Errorf("not http 201"))
			return
		}
	}()
}
