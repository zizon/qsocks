package internal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"

	quic "github.com/lucas-clemente/quic-go"
)

var (
	// PeerQuicProtocol quic peer protocl
	PeerQuicProtocol = []string{"quic-peer"}
)

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		//NotAfter:     time.Now().AddDate(99, 0, 0),
		//DNSNames:     []string{"*"},
	}
	certDER, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   PeerQuicProtocol,
	}
}

type quicProxyBundle struct {
	proxyCtx CanclableContext
	from     quic.Stream
}

func quicProxy(bundle quicProxyBundle) {
	from := bundle.from
	LogInfo("start quic proxy for:%s\n", from)

	// 1. decode remote
	packet := &QsockPacket{}
	if err := packet.Decode(from); err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}

	LogInfo("decode quic request type:%v\n", packet.TYPE)

	// 2. build remote addr
	addr := fmt.Sprintf("%s:%d", packet.HOST, packet.PORT)

	// 3. connect
	to, err := net.Dial("tcp", addr)
	if err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}
	bundle.proxyCtx.Cleanup(to.Close)

	LogInfo("quic tcp -> %s", addr)
	BiCopy(bundle.proxyCtx, from, to, io.Copy)
	return
}

type quicServerBundle struct {
	serverCtx CanclableContext
	listen    string
}

// StartQuicServer start a quic proxy server
func StartQuicServer(ctx context.Context, listen string) CanclableContext {
	serverCtx := NewCanclableContext(ctx)

	go quicServer(quicServerBundle{
		serverCtx,
		listen,
	})

	return serverCtx
}

func quicServer(bundle quicServerBundle) {
	server, err := quic.ListenAddr(bundle.listen, generateTLSConfig(), &quic.Config{
		MaxIdleTimeout: 30 * time.Second,
	})
	if err != nil {
		bundle.serverCtx.CancleWithError(err)
		return
	}
	bundle.serverCtx.Cleanup(server.Close)
	LogInfo("quic server listening:%s", server.Addr())

	for {
		session, err := server.Accept(bundle.serverCtx)
		if err != nil {
			bundle.serverCtx.CancleWithError(err)
			return
		}
		LogInfo("start quic server session:%v\n", session)

		sessionCtx := bundle.serverCtx.Derive(nil)
		sessionCtx.Cleanup(func() error {
			session.CloseWithError(0, "")
			return nil
		})

		go func() {
			for {
				stream, err := session.AcceptStream(sessionCtx)
				if err != nil {
					sessionCtx.CancleWithError(err)
					return
				}
				LogInfo("start quic server stream:%v\n", stream)

				streamCtx := sessionCtx.Derive(nil)
				streamCtx.Cleanup(stream.Close)

				go quicProxy(quicProxyBundle{
					streamCtx,
					stream,
				})
			}
		}()
	}
}
