package peer

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"

	quic "github.com/lucas-clemente/quic-go"
)

// ServerPeer server side of quic peering
type ServerPeer interface {
	PollNewChannel() (io.ReadWriteCloser, error)
}

// ServerPeerConfig config for generateing new server peer
type ServerPeerConfig struct {
	quic.Config
	Addr string
}

type peerStream struct {
	CanclableContext
	io.ReadWriteCloser
}

type serverPeer struct {
	CanclableContext
	quic.Listener
	streamC chan peerStream
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
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

// NewServerPeer create new server side peer
func NewServerPeer(ctx CanclableContext, serverPeerConfig ServerPeerConfig) (ServerPeer, error) {
	lquic, err := quic.ListenAddr(
		serverPeerConfig.Addr,
		generateTLSConfig(),
		&serverPeerConfig.Config,
	)
	if err != nil {
		return nil, err
	}

	serverPeer := serverPeer{
		ctx.Derive(nil),
		lquic,
		make(chan peerStream),
	}

	go serverPeer.serveQuic()

	serverPeer.Cleancup(func() {
		if err := lquic.Close(); err != nil {
			serverPeer.CollectError(err)
		}

		close(serverPeer.streamC)
	})

	return serverPeer, nil
}

func (server serverPeer) serveQuic() {
	defer server.Cancle()

	for {
		session, err := server.Accept(server)
		if err != nil {
			server.CollectError(err)
			break
		}

		sessionCtx := server.Derive(nil)
		go peerSession{
			sessionCtx,
			session,
			server.streamC,
		}.serveQuicSession()

		sessionCtx.Cleancup(func() {
			if err := session.CloseWithError(0, "close"); err != nil {
				server.CollectError(err)
			}
		})
	}
}

type peerSession struct {
	CanclableContext
	quic.Session
	streamC chan peerStream
}

func (session peerSession) serveQuicSession() {
	defer session.Cancle()

	for {
		stream, err := session.AcceptStream(session)
		if err != nil {
			session.CollectError(err)
			break
		}

		streamCtx := session.Derive(nil)
		session.streamC <- peerStream{
			streamCtx,
			stream,
		}

		streamCtx.Cleancup(func() {
			if err := stream.Close(); err != nil {
				session.CollectError(err)
			}
		})
	}
}

// PollNewChannel poll a new channel
func (server serverPeer) PollNewChannel() (io.ReadWriteCloser, error) {
	stream, ok := <-server.streamC
	if ok {
		return stream, nil
	}

	return nil, server.Err()
}

func (stream peerStream) Close() error {
	stream.Cancle()
	return stream.ReadWriteCloser.Close()
}
