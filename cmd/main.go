package main

import (
	"crypto/rsa"
	"fmt"
	"os"

	"route/handle"
	"route/transport/tcp"
	"route/utils/rsagen"
	"route/utils/signal"
)

func LoadAuthPublicKey() (*rsa.PublicKey, error) {
	publicRaw, err := os.ReadFile("public.pem")
	if err != nil {
		return nil, err
	}
	pk, err := rsagen.ParseRsaPublicKeyFromPem(publicRaw)
	return pk, err
}

func main() {
	var err error
	pk, err := LoadAuthPublicKey()
	if err != nil {
		panic(err)
	}
	h, err := handle.NewRouter()
	if err != nil {
		panic(err)
	}
	h.PublicKey = pk

	svr := tcp.NewServer(tcp.ServerOptions{
		Address:   ":14321",
		OnMessage: h.OnMessage,
		OnConn:    h.OnConn,
	})

	err = svr.Start()
	if err != nil {
		panic(err)
	}

	fmt.Println("server started,listening on", svr.Address())
	signal.WaitShutdown()
}
