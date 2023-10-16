package main

import (
	"crypto/rsa"
	"fmt"
	"os"

	"route/auth"
	"route/client"
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
	jwtstr, err := auth.GenerateToken(pk, 111, "test-cli-111", "user")
	if err != nil {
		panic(err)
	}
	c := client.NewTcpClient("localhost:14321", jwtstr)
	c.OnConnectFunc = func(c *client.TcpClient, enable bool) {
		if enable {
			fmt.Println("client connected")
		} else {
			fmt.Println("client disconnected")
		}
	}
	c.OnMessageFunc = func(c *client.TcpClient, p *tcp.THVPacket) {
		// ptype := p.GetType()
		// if ptype == PacketTypRoute {
		// 	head, err := tcp.CastRoutHead(p.GetBody())
		// 	if err != nil {
		// 		log.Println(err)
		// 		return
		// 	}
		// 	msgtype := head.GetMsgTyp()
		// 	msgid := head.GetMsgID()

		// 	if msgtype == hand.RouteTypRequest && msgid == uint32(msg.Echo_ID) {
		// 		head.SetMsgTyp(tcp.RouteTypResponse)
		// 		head.SetTargetUID(head.GetSrouceUID())
		// 		c.SendPacket(p)
		// 		return
		// 	}
		// }
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
