package main

import (
	"crypto/rsa"
	"fmt"
	"os"
	"os/signal"
	"route/auth"
	"route/server"
	"route/utils"
	"syscall"
	"time"
)

const PrivateKeyFile = "private.pem"
const PublicKeyFile = "public.pem"

func ReadRSAKey() ([]byte, []byte, error) {
	privateRaw, err := os.ReadFile(PrivateKeyFile)
	if err != nil {
		privateKey, publicKey, err := utils.GenerateRsaPem(512)
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
	pr, err := utils.ParseRsaPrivateKeyFromPem(rawpr)
	if err != nil {
		return nil, nil, err
	}
	pu, err := utils.ParseRsaPublicKeyFromPem(rawpu)
	if err != nil {
		return nil, nil, err
	}
	return pr, pu, nil
}

func main() {
	var err error
	privateKey, publicKey, err := LoadAuthKey()
	if err != nil {
		panic(err)
	}

	addr := "192.168.2.14:8080"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	echodata := "helloworld"
	if len(os.Args) > 2 {
		echodata = os.Args[2]
	}

	jwt, _ := auth.GenerateToken(privateKey, &auth.UserInfo{
		UId:   10001,
		UName: "gdclient",
		URole: "user",
	}, 24*time.Hour)

	fmt.Println(jwt)
	uinfo, err := auth.VerifyToken(publicKey, jwt)
	if err != nil {
		panic(err)
	}

	fmt.Println(uinfo)
	total := 0
	datalen := len(echodata)

	var startAt time.Time
	var endAt time.Time

	opts := server.TcpClientOptions{
		RemoteAddress: addr,
		Token:         jwt,
		OnSessionPacket: func(s server.Session, p server.Packet) {
			total += datalen
			if p.PacketType() == server.HVPacketType {
				packet := p.(*server.HVPacket)
				switch packet.GetFlag() {
				case server.HVPacketFlagEcho:
					err := s.Send(packet)
					if err != nil {
						fmt.Println(err)
					}
				}
			}
		},
		OnSessionStatus: func(s server.Session, enable bool) {
			fmt.Printf("onconn sid:%v, enable:%v \n", s.SessionID(), enable)
			if enable {
				startAt = time.Now()
				pk := server.NewHVPacket()
				pk.SetFlag(server.HVPacketFlagEcho)
				pk.SetBody([]uint8(echodata))
				s.Send(pk)
			} else {
				endAt = time.Now()
				costtime := endAt.Sub(startAt)
				fmt.Printf("total size: %v ,time:%v \n", total, costtime.Seconds())
			}
		},
		ReconnectDelaySecond: -1,
	}
	cli := server.NewTcpClient(opts)

	err = cli.Connect()
	if err != nil {
		fmt.Println(err)
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	<-signals
}
