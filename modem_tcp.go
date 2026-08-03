package main

import (
	"net"
	"sync"
	"time"
)

type modemTCPLink struct {
	conn net.Conn
	rx   chan []byte
	tx   chan []byte
	done chan struct{}
	once sync.Once
}

func dialModemTCP(address string) (*modemTCPLink, error) {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		return nil, err
	}
	return newModemTCPLink(conn), nil
}

func newModemTCPLink(conn net.Conn) *modemTCPLink {
	link := &modemTCPLink{conn: conn, rx: make(chan []byte, 32), tx: make(chan []byte, 64), done: make(chan struct{})}
	go link.readLoop()
	go link.writeLoop()
	return link
}

func (l *modemTCPLink) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := l.conn.Read(buf)
		if n > 0 {
			packet := append([]byte(nil), buf[:n]...)
			select {
			case l.rx <- packet:
			case <-l.done:
				return
			}
		}
		if err != nil {
			l.stop()
			return
		}
	}
}

func (l *modemTCPLink) writeLoop() {
	for {
		select {
		case packet := <-l.tx:
			if _, err := l.conn.Write(packet); err != nil {
				l.stop()
				return
			}
		case <-l.done:
			return
		}
	}
}

func (l *modemTCPLink) send(data []byte) bool {
	packet := append([]byte(nil), data...)
	select {
	case l.tx <- packet:
		return true
	case <-l.done:
		return false
	}
}

func (l *modemTCPLink) poll() []byte {
	var data []byte
	for {
		select {
		case packet := <-l.rx:
			data = append(data, packet...)
		default:
			return data
		}
	}
}

func (l *modemTCPLink) stop() {
	l.once.Do(func() {
		close(l.done)
		_ = l.conn.Close()
	})
}

func (l *modemTCPLink) close() {
	l.stop()
}
