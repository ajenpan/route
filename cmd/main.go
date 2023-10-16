package main

import (
	"crypto/rsa"
	"fmt"
	"os"

	"route/auth"
	"route/handle"
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

func StartServer(listenAt string) {
	var err error
	_, pk, err := LoadAuthKey()
	if err != nil {
		panic(err)
	}

	h, err := handle.NewRouter()
	if err != nil {
		panic(err)
	}
	h.Authc = &auth.LocalAuth{
		PK: pk,
	}

	svr, err := tcp.NewServer(tcp.ServerOptions{
		Address:   listenAt,
		OnMessage: h.OnMessage,
		OnConn:    h.OnConn,
		AuthTokenChecker: func(token string) (*tcp.UserInfo, error) {
			uid, uname, role, err := auth.VerifyToken(pk, token)
			if err != nil {
				return nil, err
			}
			return &tcp.UserInfo{
				UId:   uint64(uid),
				UName: uname,
				Role:  role,
			}, nil
		},
	})

	if err != nil {
		panic(err)
	}

	defer svr.Stop()
	fmt.Println("server started,listening on", svr.Address())
	go svr.Start()

	signal.WaitShutdown()
}

func main() {
	StartServer(":14321")
	signal.WaitShutdown()
}
