package tcp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type ClientOption func(*ClientOptions)

type ClientOptions struct {
	RemoteAddress string
	OnMessage     OnMessageFunc
	OnConnStat    OnConnStatFunc

	Timeout time.Duration
}

func NewClient(opts *ClientOptions) *Client {
	if opts.Timeout < DefaultMinTimeout {
		opts.Timeout = DefaultTimeout
	}

	ret := &Client{
		Opt: opts,
	}
	return ret
}

type Client struct {
	*Socket
	Opt   *ClientOptions
	mutex sync.Mutex
}

func (c *Client) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Socket != nil {
		c.Socket.Close()
		c.Socket = nil
	}

	if c.Opt.RemoteAddress == "" {
		return fmt.Errorf("remote address is empty")
	}

	conn, err := net.DialTimeout("tcp", c.Opt.RemoteAddress, c.Opt.Timeout)
	if err != nil {
		return err
	}

	socket := NewSocket(conn, SocketOptions{
		Timeout: c.Opt.Timeout,
	})

	//send ack
	p := NewEmptyTHVPacket()
	p.SetType(PacketTypAck)
	err = socket.writePacket(p)
	if err != nil {
		socket.Close()
		return err
	}

	//read ack
	err = socket.readPacket(p)
	if err != nil {
		socket.Close()
		return err
	}
	if p.GetType() != PacketTypAck {
		socket.Close()
		return fmt.Errorf("read ack failed, typ: %d", p.GetType())
	}
	//set socket id
	if len(p.body) > 0 {
		socket.id = string(p.body)
	}

	//here is connect finished
	c.Socket = socket

	go func() {
		defer socket.Close()

		go socket.writeWork()

		if c.Opt.OnConnStat != nil {
			c.Opt.OnConnStat(c.Socket, Connected)

			defer func() {
				c.Opt.OnConnStat(c.Socket, Disconnected)
			}()
		}

		go func() {
			tk := time.NewTicker(30 * time.Second)
			defer tk.Stop()

			heartbeatPakcet := NewEmptyTHVPacket()
			heartbeatPakcet.SetType(PacketTypHeartbeat)

			for {
				select {
				case <-tk.C:
					nowUnix := uint64(time.Now().Unix())
					lastSendAt := atomic.LoadUint64(&socket.lastSendAt)
					if nowUnix-lastSendAt > 30 {
						socket.chSend <- heartbeatPakcet
					}
				case <-socket.chClosed:
					fmt.Println("closed heartbeatPakcet")
					return
				}
			}
		}()

		var socketErr error = nil
		for {
			p := NewEmptyTHVPacket()
			if socketErr = socket.readPacket(p); socketErr != nil {
				//todo: print out error
				break
			}
			typ := p.GetType()
			if typ > PacketTypInnerEndAt_ {
				if c.Opt.OnMessage != nil {
					c.Opt.OnMessage(socket, p)
				}
			} else {

			}
		}
		fmt.Println("socket read error: ", socketErr)
	}()
	return nil
}

func (c *Client) Close() {
	if c.Socket != nil {
		c.Socket.Close()
	}
}
