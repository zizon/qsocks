package cmd_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"net/url"
	"testing"

	"github.com/zizon/qsocks/pkg/client"
	"github.com/zizon/qsocks/pkg/server"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile | log.Lmsgprefix)
}

func TestProtocol(t *testing.T) {
	proxy := "127.0.0.1:10088"
	localSocks5 := "127.0.0.1:10089"
	ctx, cancle := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancle()

	// socks5 client
	if _, err := client.NewClient(client.Config{
		Context:          ctx,
		Connect:          proxy,
		Listen:           localSocks5,
		StreamPerSession: 5,
	}); err != nil {
		t.Errorf("fail to create client:%v", err)
		return
	}

	// socks5 server
	if _, err := server.NewServer(server.Config{
		Context: ctx,
		Listen:  proxy,
	}); err != nil {
		t.Errorf("fail to create server:%v", err)
	}

	// http client
	client := http.Client{
		Transport: &http.Transport{
			Proxy: func(r *http.Request) (*url.URL, error) {
				socks5, err := url.Parse(fmt.Sprintf("socks5://%s", localSocks5))
				if err != nil {
					return nil, fmt.Errorf("fail to create proxy:%v", err)
				}

				return socks5, nil
			},
		},
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		// http request
		go func() {
			defer wg.Done()
			if r, err := http.NewRequestWithContext(
				ctx,
				"GET",
				"http://www.baidu.com",
				nil,
			); err != nil {
				t.Errorf("fail to create test request:%v", err)
				return
			} else if resp, err := client.Do(r); err != nil {
				t.Errorf("fail to do http proxy reqeust:%v", err)
				return
			} else {
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Errorf("expect http:200 %v", resp.StatusCode)
				}
			}
		}()
	}

	wg.Wait()
	cancle()
	<-ctx.Done()
}
