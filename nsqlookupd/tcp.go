package nsqlookupd

import (
	"io"
	"net"
	"sync"

	"github.com/nsqio/nsq/internal/protocol"
)

type tcpServer struct {
	nsqlookupd *NSQLookupd
	conns      sync.Map
}

func (p *tcpServer) Handle(clientConn net.Conn) {
	p.nsqlookupd.logf(LOG_INFO, "TCP: new client(%s)", clientConn.RemoteAddr())

	// The client should initialize itself by sending a 4 byte sequence indicating
	// the version of the protocol that it intends to communicate, this will allow us
	// to gracefully upgrade the protocol away from text/line oriented to whatever...
	buf := make([]byte, 4)
	_, err := io.ReadFull(clientConn, buf)
	if err != nil {
		p.nsqlookupd.logf(LOG_ERROR, "failed to read protocol version - %s", err)
		clientConn.Close()
		return
	}
	protocolMagic := string(buf)

	p.nsqlookupd.logf(LOG_INFO, "CLIENT(%s): desired protocol magic '%s'",
		clientConn.RemoteAddr(), protocolMagic)

	var prot protocol.Protocol
	switch protocolMagic {
	case "  V1":
		prot = &LookupProtocolV1{nsqlookupd: p.nsqlookupd}
	default:
		protocol.SendResponse(clientConn, []byte("E_BAD_PROTOCOL"))
		clientConn.Close()
		p.nsqlookupd.logf(LOG_ERROR, "client(%s) bad protocol magic '%s'",
			clientConn.RemoteAddr(), protocolMagic)
		return
	}

	p.conns.Store(clientConn.RemoteAddr(), clientConn)

	err = prot.IOLoop(clientConn)
	if err != nil {
		p.nsqlookupd.logf(LOG_ERROR, "client(%s) - %s", clientConn.RemoteAddr(), err)
	}

	p.conns.Delete(clientConn.RemoteAddr())
}

func (p *tcpServer) CloseAll() {
	p.conns.Range(func(k, v interface{}) bool {
		v.(net.Conn).Close()
		return true
	})
}
