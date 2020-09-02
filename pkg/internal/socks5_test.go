package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"sync"
	"testing"
	"time"
)

const (
	statusExpect = 204
)

func SetupProxyTarget(ctx CanclableContext, httpListen string, t *testing.T) {
	go func() {
		if err := http.ListenAndServe(httpListen, http.HandlerFunc(func(resp http.ResponseWriter, request *http.Request) {
			LogDebug("receive http request %v", request)
			resp.WriteHeader(statusExpect)
		})); err != nil {
			t.Error(err)
		}
	}()
}

func StartClientTest(ctx CanclableContext, socks5 string, target string, t *testing.T, connections int) {
	proxyURL, err := url.Parse(fmt.Sprintf("socks5://%s", socks5))
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
	for i := 0; i < connections; i++ {
		concurrent(ctx, client, statusExpect, t, target, wg)
	}
	wg.Wait()

	ctx.Cancle()
}

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
	SetupProxyTarget(ctx, httpListen, t)

	go StartQuicServer(ctx, remote)
	go StartSocks5RaceServer(ctx, listen, remote, 0)

	StartClientTest(ctx, listen, httpListen, t, 100)
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

		LogDebug("send http request: %v", request)
		resp, err := client.Do(request)

		LogDebug("response: %v", resp)
		if err != nil {
			t.Error(err)
			ctx.CancleWithError(err)
			return
		} else if resp.StatusCode != statusExpect {
			t.Errorf("not http 204: %d", resp.StatusCode)
			ctx.CancleWithError(fmt.Errorf("not http 204: %d", resp.StatusCode))
			return
		}
	}()
}
