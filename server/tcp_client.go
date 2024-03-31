package server

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type TcpClientOption func(*TcpClientOptions)

type TcpClientOptions struct {
	RemoteAddress        string
	Token                string
	Timeout              time.Duration
	ReconnectDelaySecond int32

	OnSessionPacket FuncOnSessionPacket
	OnSessionStatus FuncOnSessionStatus
}

func NewTcpClient(opts TcpClientOptions) *tcpClient {
	if opts.Timeout < time.Duration(DefaultMinTimeoutSec)*time.Second {
		opts.Timeout = time.Duration(DefaultTimeoutSec) * time.Second
	}
	if opts.ReconnectDelaySecond == 0 {
		opts.ReconnectDelaySecond = 5
	}
	ret := &tcpClient{
		Opt: opts,
	}
	return ret
}

type tcpClient struct {
	tcpSocket

	Opt   TcpClientOptions
	mutex sync.Mutex
}

func doAckAction(c net.Conn, body []byte) error {
	p := NewHVPacket()
	p.SetFlag(hvPacketFlagDoAction)
	p.SetBody(body)
	_, err := WritePacket(c, p)
	return err
}

func (c *tcpClient) doConnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	old := atomic.SwapInt32(&c.tcpSocket.status, Connectting)
	if old != Disconnected {
		return nil
	}

	conn, err := net.DialTimeout("tcp", c.Opt.RemoteAddress, c.Opt.Timeout)
	if err != nil {
		atomic.SwapInt32(&c.tcpSocket.status, Disconnected)
		return err
	}

	err = c.doHandShake(conn)
	if err != nil {
		conn.Close()
		// if hand shake fail, dont reconnect any more
		atomic.SwapInt32(&c.tcpSocket.status, Disconnected)
		c.Opt.ReconnectDelaySecond = -1
		return err
	}

	go func() {

		wg := &sync.WaitGroup{}
		wg.Add(2)

		var writeErr error
		var readErr error

		defer func() {
			wg.Wait()

			c.tcpSocket.Close()
			if c.Opt.OnSessionStatus != nil {

				fmt.Printf("socket closeat %v", errors.Join(readErr, writeErr))

				c.Opt.OnSessionStatus(c, false)
			}

			if c.Opt.ReconnectDelaySecond > 0 {
				c.reconnect()
			}
		}()

		defer conn.Close()

		socket := &c.tcpSocket
		tk := time.NewTicker(c.Opt.Timeout / 3)
		defer tk.Stop()

		go func() {
			defer func() {
				wg.Done()
				socket.Close()
			}()

			writeErr = socket.writeWork()
		}()

		go func() {
			defer func() {
				wg.Done()
				socket.Close()
			}()

			readErr = socket.readWork()
		}()

		atomic.StoreInt32(&c.tcpSocket.status, Connected)

		if c.Opt.OnSessionStatus != nil {
			c.Opt.OnSessionStatus(c, true)
		}

		heartbeatPakcet := NewHVPacket()
		heartbeatPakcet.SetFlag(HVPacketFlagHeartbeat)
		checkPos := int64(c.Opt.Timeout.Seconds() / 2)

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
					socket.Send(heartbeatPakcet)
				}
			case p, ok := <-socket.chRead:
				if !ok {
					return
				}

				dealed := false

				if p.PacketType() == HVPacketType {
					packet := p.(*HVPacket)
					switch packet.GetFlag() {
					case HVPacketFlagHeartbeat:
						dealed = true
					}
				}

				if !dealed && c.Opt.OnSessionPacket != nil {
					c.Opt.OnSessionPacket(socket, p)
				}
			}
		}
	}()
	return nil
}

func (c *tcpClient) reconnect() {
	time.AfterFunc(time.Duration(c.Opt.ReconnectDelaySecond)*time.Second, func() {
		fmt.Println("start to reconnect")
		if c.IsValid() {
			fmt.Println("already connected")
			return
		}
		err := c.doConnect()
		if err != nil {
			fmt.Println("connect error:", err)
			// go on reconnect
			c.reconnect()
		}
	})
}

func (c *tcpClient) doHandShake(conn net.Conn) error {
	deadline := time.Now().Add(c.Opt.Timeout)
	conn.SetReadDeadline(deadline)
	conn.SetWriteDeadline(deadline)

	p := NewHVPacket()
	p.SetFlag(hvPacketFlagHandShake)
	if _, err := WritePacket(conn, p); err != nil {
		return err
	}

	actions := map[string][]byte{
		"auth": []byte(c.Opt.Token),
	}

	socketid := ""

	var err error
	for {
		var pp *HVPacket
		pp, err = ReadPacketT[*HVPacket](conn)
		if err != nil {
			break
		}
		if pp.GetFlag() == hvPacketFlagActionRequire {
			name := string(pp.GetBody())
			if data, has := actions[name]; !has {
				err = fmt.Errorf("action %s not found", name)
				break
			} else {
				if err = doAckAction(conn, data); err != nil {
					break
				}
			}
		} else if pp.GetFlag() == hvPacketFlagAckResult {
			body := string(pp.GetBody())
			if len(body) == 0 {
				err = fmt.Errorf("ack result failed")
				break
			}
			socketid = body
			break
		} else {
			err = fmt.Errorf("invalid packet type: %d", pp.GetFlag())
			break
		}
	}

	if err != nil {
		return err
	}

	socket := &c.tcpSocket
	socket.id = socketid
	socket.conn = conn
	socket.timeOut = c.Opt.Timeout
	socket.chWrite = make(chan Packet, 100)
	socket.chRead = make(chan Packet, 100)
	socket.chClosed = make(chan struct{})
	socket.Meta = sync.Map{}
	socket.lastSendAt = time.Now().Unix()
	socket.lastRecvAt = time.Now().Unix()
	return nil
}

func (c *tcpClient) Connect() error {
	if c.Opt.RemoteAddress == "" {
		return fmt.Errorf("remote address is empty")
	}
	err := c.doConnect()
	if err != nil && c.Opt.ReconnectDelaySecond > 0 {
		c.reconnect()
	}
	return err
}

func (c *tcpClient) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Opt.ReconnectDelaySecond = -1
	return c.tcpSocket.Close()
}
