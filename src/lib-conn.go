package main

import (
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
)

var conns = make(map[string]net.Conn)
var connsMutex sync.Mutex

func ConnStateEvent(conn net.Conn, event http.ConnState) {
	connsMutex.Lock()
	defer connsMutex.Unlock()
	if event == http.StateActive {
		conns[conn.RemoteAddr().String()] = conn
	} else if event == http.StateHijacked || event == http.StateClosed {
		delete(conns, conn.RemoteAddr().String())
	}
}
func GetConn(r *http.Request) net.Conn {
	connsMutex.Lock()
	c := conns[r.RemoteAddr]
	connsMutex.Unlock()
	/*if c != nil {
		err := connCheck(c)
		if err != nil {
			return nil
		}
	}*/
	return c
}

func connCheck(conn net.Conn) error {
	var sysErr error = nil
	rc, err := conn.(syscall.Conn).SyscallConn()
	if err != nil {
		return err
	}
	err = rc.Read(func(fd uintptr) bool {
		var buf []byte = []byte{0}
		n, _, err := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch {
		case n == 0 && err == nil:
			sysErr = io.EOF
		case err == syscall.EAGAIN || err == syscall.EWOULDBLOCK:
			sysErr = nil
		default:
			sysErr = err
		}
		return true
	})
	if err != nil {
		return err
	}

	return sysErr
}
