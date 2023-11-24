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
	RemoteAddress        string
	OnMessage            func(*Client, *THVPacket)
	OnConnStat           func(*Client, bool)
	Token                string
	Timeout              time.Duration
	ReconnectDelaySecond int32
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
	*Socket
	Opt   *ClientOptions
	mutex sync.Mutex
}

func doAckAction(c net.Conn, name string, body []byte, timeout time.Duration) error {
	p := newEmptyTHVPacket()
	p.SetType(PacketTypeDoAction)
	p.SetHead([]byte(name))
	p.SetBody(body)
	return writePacket(c, p, timeout)
}

func (c *Client) doconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Socket != nil && c.Socket.Valid() {
		c.Socket.Close()
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
	c.Socket = socket

	go func() {

		tk := time.NewTicker(c.Opt.Timeout / 3)
		defer tk.Stop()
		heartbeatPakcet := newEmptyTHVPacket()
		heartbeatPakcet.SetType(PacketTypeHeartbeat)
		checkPos := int64(c.Opt.Timeout.Seconds() / 2)

		var writeErr error
		var readErr error

		go func() {
			writeErr = socket.writeWork()
			fmt.Println("writeErr:", writeErr)
		}()
		recvchan := make(chan *THVPacket, 100)
		go func() {
			readErr = socket.readWork(recvchan)
			fmt.Println("readErr:", readErr)
		}()

		defer func() {
			c.Socket.Close()
			if c.Opt.OnConnStat != nil {
				c.Opt.OnConnStat(c, false)
			}
			if c.Opt.ReconnectDelaySecond > 0 {
				c.reconnect()
			}
		}()

		if c.Opt.OnConnStat != nil {
			c.Opt.OnConnStat(c, true)
		}

		for {
			select {
			case <-socket.chClosed:
				return
			case t, ok := <-tk.C:
				if !ok {
					return
				}
				nowUnix := t.Unix()
				lastSendAt := atomic.LoadInt64(&socket.lastSendAt)
				if nowUnix-lastSendAt >= checkPos {
					socket.chSend <- heartbeatPakcet
				}
			case p, ok := <-recvchan:
				if !ok {
					return
				}
				typ := p.GetType()

				if typ >= PacketTypeInnerStartAt_ && typ <= PacketTypeInnerEndAt_ {
					switch typ {
					case PacketTypeHeartbeat:
						fallthrough
					case PacketTypeEcho:
					default:
						//DO NOTHING
					}

				} else {
					if c.Opt.OnMessage != nil {
						c.Opt.OnMessage(c, p)
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
		if c.Valid() {
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

func (c *Client) doHandShake(conn net.Conn) (*Socket, error) {
	rwtimeout := c.Opt.Timeout

	p := newEmptyTHVPacket()
	p.SetType(PacketTypeHandShake)
	if err := writePacket(conn, p, rwtimeout); err != nil {
		return nil, err
	}

	actions := map[string][]byte{
		"auth": []byte(c.Opt.Token),
	}

	socketid := ""

	var err error
	for {
		p.Reset()
		if err = readPacket(conn, p, rwtimeout); err != nil {
			break
		}
		if p.GetType() == PacketTypeActionRequire {
			name := string(p.GetHead())
			if data, has := actions[name]; !has {
				err = fmt.Errorf("action %s not found", name)
				break
			} else {
				if err = doAckAction(conn, name, data, rwtimeout); err != nil {
					break
				}
			}
		} else if p.GetType() == PacketTypeAckResult {
			head := string(p.GetHead())
			body := string(p.GetBody())
			if head != "ok" {
				err = fmt.Errorf("ack result failed, head: %s, body: %s", head, body)
				break
			}
			if len(body) > 0 {
				socketid = body
			}
			break
		} else {
			err = fmt.Errorf("invalid packet type: %d", p.GetType())
			break
		}
	}
	if err != nil {
		return nil, err
	}
	socket := NewSocket(conn, SocketOptions{
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
