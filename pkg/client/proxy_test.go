package client

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/zizon/qsocks/pkg/internal"
)

var (
	check = []byte("hello kitty")
)

func TestClient(t *testing.T) {
	clientConfig := ClientConfig{
		ListenHost: "localhost",
		ListenPort: 10087,
		RemoteHost: "localhost",
		RemotePort: 10089,
	}
	rootCtx := internal.NewCanclableContext(context.TODO(), func(err error) {
		fmt.Printf("%v", err)
	})

	quicServer := fmt.Sprintf("%s:%d", clientConfig.RemoteHost, clientConfig.RemotePort)
	go startQUIC(rootCtx, quicServer, t)

	clientCancler := StartClient(rootCtx, clientConfig, nil)
	rootCtx.Cleanup(func() error {
		clientCancler()
		return nil
	})

	if conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", clientConfig.ListenHost, clientConfig.ListenPort)); err != nil {
		t.Error(err)
	} else if n, err := conn.Write(check); err != nil || n != len(check) {
		t.Errorf("fail write %v", err)
	}

	<-rootCtx.Done()
}

func startQUIC(ctx internal.CanclableContext, addr string, t *testing.T) {
	defer ctx.Cancle()

	server, err := internal.SimpleQuicServer(addr)
	if err != nil {
		t.Error(err)
		return
	}

	serverCtx := ctx.Derive(nil)
	serverCtx.Cleanup(server.Close)

	session, err := server.Accept(serverCtx)
	if err != nil {
		t.Error(err)
		return
	}

	sessionCtx := serverCtx.Derive(nil)
	sessionCtx.Cleanup(func() error {
		return session.CloseWithError(0, "")
	})

	stream, err := session.AcceptStream(sessionCtx)
	if err != nil {
		t.Error(err)
		return
	}

	streamCtx := sessionCtx.Derive(nil)
	streamCtx.Cleanup(stream.Close)

	buf := make([]byte, len(check))
	if n, err := stream.Read(buf); err != nil || n != len(buf) || bytes.Compare(check, buf) != 0 {
		t.Error(fmt.Errorf("not match"))
		return
	}

}
