package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"os"
	"runtime"

	"route/auth"
	"route/handle"

	"github.com/ajenpan/surf/server"
	"github.com/ajenpan/surf/utils/rsagen"
	"github.com/ajenpan/surf/utils/signal"

	"github.com/urfave/cli/v2"
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
	svr, err := server.NewTcpServer(listenAt, h)
	if err != nil {
		panic(err)
	}

	defer svr.Stop()
	fmt.Println("server started,listening on: ", listenAt)
	go svr.Start()

	signal.WaitShutdown()
}

func main() {
	if err := Run(); err != nil {
		fmt.Println(err)
	}
}

var (
	Name       string = "battlefield"
	Version    string = "unknow"
	GitCommit  string = "unknow"
	BuildAt    string = "unknow"
	BuildBy    string = runtime.Version()
	RunnningOS string = runtime.GOOS + "/" + runtime.GOARCH
)

func longVersion() string {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintln(buf, "project:", Name)
	fmt.Fprintln(buf, "version:", Version)
	fmt.Fprintln(buf, "git commit:", GitCommit)
	fmt.Fprintln(buf, "build at:", BuildAt)
	fmt.Fprintln(buf, "build by:", BuildBy)
	fmt.Fprintln(buf, "running OS/Arch:", RunnningOS)
	return buf.String()
}

func Run() error {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Println(longVersion())
	}
	app := cli.NewApp()
	app.Version = Version
	app.Name = Name
	app.Action = RealMain
	err := app.Run(os.Args)
	return err
}

func RealMain(c *cli.Context) error {
	StartServer(":14321")
	signal.WaitShutdown()
	return nil
}
