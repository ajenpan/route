package server

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type ClientOption func(*ClientOptions)

type ClientOptions struct {
	RemoteAddress        string
	Token                string
	Timeout              time.Duration
	ReconnectDelaySecond int32

	OnSocketMessage func(*Client, Packet)
	OnSocketConn    func(*Client)
	OnSocketDisconn func(*Client, error)
}

func NewClient(opts *ClientOptions) *Client {
	if opts.Timeout < time.Duration(DefaultMinTimeoutSec)*time.Second {
		opts.Timeout = time.Duration(DefaultTimeoutSec) * time.Second
	}

	ret := &Client{
		Opt: opts,
	}

	return ret
}

type Client struct {
	*tcpSocket
	Opt   *ClientOptions
	mutex sync.Mutex
}

func doAckAction(c net.Conn, body []byte, timeout time.Duration) error {
	p := newHVPacket()
	p.SetType(hvPacketFlagDoAction)
	p.SetBody(body)
	return WritePacket(c, timeout, p)
}

func (c *Client) doconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.tcpSocket != nil && c.tcpSocket.IsValid() {
		c.tcpSocket.Close()
	}

	conn, err := net.DialTimeout("tcp", c.Opt.RemoteAddress, c.Opt.Timeout)
	if err != nil {
		return err
	}
	socket, err := c.doHandShake(conn)
	if err != nil {
		conn.Close()
		return err
	}
	c.tcpSocket = socket

	go func() {
		tk := time.NewTicker(c.Opt.Timeout / 3)
		defer tk.Stop()
		heartbeatPakcet := newHVPacket()
		heartbeatPakcet.SetType(hvPacketFlagHeartbeat)
		checkPos := int64(c.Opt.Timeout.Seconds() / 2)

		var writeErr error
		var readErr error

		go func() {
			writeErr = socket.writeWork()
		}()
		recvchan := make(chan Packet, 100)
		go func() {
			readErr = socket.readWork(recvchan)
		}()

		defer func() {
			c.tcpSocket.Close()
			if c.Opt.OnSocketDisconn != nil {
				c.Opt.OnSocketDisconn(c, errors.Join(writeErr, readErr))
			}
			if c.Opt.ReconnectDelaySecond > 0 {
				c.reconnect()
			}
		}()

		if c.Opt.OnSocketConn != nil {
			c.Opt.OnSocketConn(c)
		}

		for {
			select {
			case <-socket.chClosed:
				return
			case now, ok := <-tk.C:
				if !ok {
					return
				}
				nowUnix := now.Unix()
				lastSendAt := atomic.LoadInt64(&socket.lastSendAt)
				if nowUnix-lastSendAt >= checkPos {
					socket.chWrite <- heartbeatPakcet
				}
			case p, ok := <-recvchan:
				if !ok {
					return
				}
				switch p.PacketType() {
				case hvPacketTypeInnerStartAt_:
				default:
					if c.Opt.OnSocketMessage != nil {
						c.Opt.OnSocketMessage(c, p)
					}
				}
			}
		}
	}()
	return nil
}

func (c *Client) reconnect() {
	time.AfterFunc(time.Duration(c.Opt.ReconnectDelaySecond)*time.Second, func() {
		fmt.Println("start to reconnect")
		if c.IsValid() {
			fmt.Println("already connected")
			return
		}
		err := c.doconnect()
		if err != nil {
			fmt.Println("connect error:", err)
			// go on reconnect
			c.reconnect()
		}
	})
}

func (c *Client) doHandShake(conn net.Conn) (*tcpSocket, error) {
	rwtimeout := c.Opt.Timeout

	p := newHVPacket()
	p.SetType(hvPacketFlagHandShake)
	if err := WritePacket(conn, rwtimeout, p); err != nil {
		return nil, err
	}

	actions := map[string][]byte{
		"auth": []byte(c.Opt.Token),
	}

	socketid := ""
	var err error

	for {
		var pp *HVPacket
		pp, err = ReadPacketT[*HVPacket](conn, rwtimeout)
		if err != nil {
			break
		}
		if pp.GetType() == hvPacketFlagActionRequire {
			name := string(pp.GetBody())
			if data, has := actions[name]; !has {
				err = fmt.Errorf("action %s not found", name)
				break
			} else {
				if err = doAckAction(conn, data, rwtimeout); err != nil {
					break
				}
			}
		} else if pp.GetType() == hvPacketFlagAckResult {
			body := string(pp.GetBody())
			if len(body) == 0 {
				err = fmt.Errorf("ack result failed")
				break
			}
			socketid = body
			break
		} else {
			err = fmt.Errorf("invalid packet type: %d", pp.GetType())
			break
		}
	}

	if err != nil {
		return nil, err
	}

	socket := NewTcpSocket(conn, tcpSocketOptions{
		ID:      socketid,
		Timeout: rwtimeout,
	})
	return socket, nil
}

func (c *Client) Connect() error {
	if c.Opt.RemoteAddress == "" {
		return fmt.Errorf("remote address is empty")
	}
	err := c.doconnect()
	if err != nil && c.Opt.ReconnectDelaySecond > 0 {
		c.reconnect()
	}
	return err
}

func (c *Client) Close() {
	if c.tcpSocket != nil {
		c.tcpSocket.Close()
	}
}
