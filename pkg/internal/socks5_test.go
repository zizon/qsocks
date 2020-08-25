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
	"time"
)

func TestSockstConnection(t *testing.T) {
	SetDefaultCollector(func(err error) {
		//fmt.Printf("err %v type:%v\n", err, reflect.TypeOf(err))
		//defaultCollector(err)
	})

	root := NewCanclableContext(context.TODO())
	ctx := root.Derive(nil)

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

	go StartQuicServer(ctx, remote)
	go StartSocks5RaceServer(ctx, listen, remote)

	proxyURL, err := url.Parse(fmt.Sprintf("socks5://%s", listen))
	if err != nil {
		t.Error(err)
	}
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	// wait a littel to startup
	<-time.NewTimer(5 * time.Second).C

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
			t.Error(err)
			ctx.CancleWithError(err)
			return
		}

		LogDebug("send http request:%v", request)
		resp, err := client.Do(request)

		LogDebug("response:%v", resp)
		if err != nil {
			t.Error(err)
			ctx.CancleWithError(err)
			return
		} else if resp.StatusCode != statusExpect {
			t.Errorf("not http 204")
			ctx.CancleWithError(fmt.Errorf("not http 204"))
			return
		}
	}()
}
