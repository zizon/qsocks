package client

import (
	"context"
	"net"

	"github.com/zizon/qsocks/pkg/logging"
)

type local struct {
	context.Context
	cancle context.CancelFunc
	listen string
}

func (l local) create() <-chan net.Conn {
	ch := make(chan net.Conn)

	go func() {
		nl, err := net.Listen("tcp", l.listen)
		if err != nil {
			l.cancle()
			logging.Error("fail to listen on:%v terminte:%v", l.listen, err)
			return
		}
		defer nl.Close()

		for {
			c, err := nl.Accept()
			if err != nil {
				logging.Error("fail to accept local conenction:%v terminate:%v", l.listen, err)
				return
			}

			select {
			case ch <- c:
			case <-l.Done():
				c.Close()
				logging.Info("parent context terminated, stop listening:%v", l.listen, l.Err())
				return
			}
		}
	}()

	return ch
}
