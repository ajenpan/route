package server

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestTcpSocketClose(t *testing.T) {
	const pk = 1000
	s := &tcpSocket{
		chWrite:  make(chan Packet, pk),
		chClosed: make(chan struct{}),
		status:   Connected,
	}

	for i := 0; i < 2; i++ {
		go func() {
			for p := range s.chWrite {
				fmt.Println(p.(*RoutePacket).GetUid())
			}
			fmt.Println("exit")
		}()
	}

	closer := func() {
		select {
		case <-s.chClosed:
			return
		default:
			close(s.chClosed)
			close(s.chWrite)
		}
	}

	go func() {
		time.Sleep(time.Duration(rand.Int31n(100)) * time.Microsecond)
		closer()
	}()

	defer closer()
	i := 0

	for {
		i++
		p := NewRoutePacket()
		p.SetUid(uint32(i))

		select {
		case <-s.chClosed:
			return
		case s.chWrite <- p:
		}
	}
}
