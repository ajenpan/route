package main

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"route/auth"
	"route/client"
	msg "route/proto"
	"route/transport/tcp"
	"route/utils/rsagen"
	"route/utils/signal"
)

const PrivateKeyFile = "private.pem"
const PublicKeyFile = "public.pem"

func ReadRSAKey() ([]byte, []byte, error) {
	privateRaw, err := os.ReadFile(PrivateKeyFile)
	if err != nil {
		privateKey, publicKey, err := rsagen.GenerateRsaPem(2048)
		if err != nil {
			return nil, nil, err
		}
		privateRaw = []byte(privateKey)
		os.WriteFile(PrivateKeyFile, []byte(privateKey), 0644)
		os.WriteFile(PublicKeyFile, []byte(publicKey), 0644)
	}
	publicRaw, err := os.ReadFile(PublicKeyFile)
	if err != nil {
		return nil, nil, err
	}
	return privateRaw, publicRaw, nil
}

func LoadAuthKey() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	rawpr, rawpu, err := ReadRSAKey()
	if err != nil {
		return nil, nil, err
	}
	pr, err := rsagen.ParseRsaPrivateKeyFromPem(rawpr)
	if err != nil {
		return nil, nil, err
	}
	pu, err := rsagen.ParseRsaPublicKeyFromPem(rawpu)
	if err != nil {
		return nil, nil, err
	}
	return pr, pu, nil
}

func StartClient() {
	var err error
	pk, _, err := LoadAuthKey()
	if err != nil {
		panic(err)
	}
	jwt, err := auth.GenerateToken(pk, 111, "test-cli-111", "user")
	if err != nil {
		panic(err)
	}
	c := client.NewTcpClient("localhost:14321", jwt)
	c.OnMessageFunc = func(c *client.TcpClient, p *tcp.PackFrame) {
		ptype := p.GetType()
		if ptype == tcp.PacketTypRoute {
			head, err := tcp.CastRoutHead(p.GetHead())
			if err != nil {
				fmt.Println(err)
				return
			}
			msgtype := head.GetMsgTyp()
			msgid := head.GetMsgID()

			if msgtype == tcp.RouteTypRequest && msgid == uint32(msg.Echo_ID) {
				head.SetMsgTyp(tcp.RouteTypResponse)
				head.SetTargetUID(head.GetSrouceUID())
				c.SendPacket(p)
				return
			}
		}
	}
	c.OnLoginFunc = func(c *client.TcpClient, stat client.LoginStat) {
		fmt.Println("login stat:", stat)
		if stat == client.LoginStat_Success {
			go func() {
				tick := time.NewTicker(2 * time.Second)
				defer tick.Stop()
				for range tick.C {
					var err error
					sendAt := time.Now()
					b, err := sendAt.MarshalBinary()
					if err != nil {
						break
					}
					c.TargetEcho(386951685, []byte(b), func(err error, raw []byte) {
						fmt.Println("targetecho cost:", time.Since(sendAt))
					})
					if err != nil {
						fmt.Println("echo error:", err)
						break
					}
				}
				os.Exit(0)
			}()
		} else {
			os.Exit(0)
		}
	}
	err = c.Connect()
	if err != nil {
		c.AutoRecconect = false
		panic(err)
	} else {
		c.AutoRecconect = true
	}
	fmt.Println("client connect to ", c.RemoteAddr())
}

func main() {
	StartClient()
	signal.WaitShutdown()
}
