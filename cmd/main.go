package main

import (
	"bytes"
	"crypto/rsa"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"

	"route/auth"
	"route/handle"
	"route/server"

	"os/signal"

	"github.com/urfave/cli/v2"
)

const PrivateKeyFile = "private.pem"
const PublicKeyFile = "public.pem"

func ReadRSAKey() ([]byte, []byte, error) {
	privateRaw, err := os.ReadFile(PrivateKeyFile)
	if err != nil {
		privateKey, publicKey, err := GenerateRsaPem(512)
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
	pr, err := ParseRsaPrivateKeyFromPem(rawpr)
	if err != nil {
		return nil, nil, err
	}
	pu, err := ParseRsaPublicKeyFromPem(rawpu)
	if err != nil {
		return nil, nil, err
	}
	return pr, pu, nil
}

func WaitShutdown() os.Signal {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	return <-signals
}

func StartServer(listenAt string) {
	var err error
	ppk, pk, err := LoadAuthKey()
	if err != nil {
		panic(err)
	}
	jwt, _ := auth.GenerateToken(ppk, &auth.UserInfo{
		UId:   10001,
		UName: "gdclient",
		URole: "user",
	}, 24*time.Hour)

	fmt.Println(jwt)

	h, err := handle.NewRouter()
	if err != nil {
		panic(err)
	}
	svropt := &server.TcpServerOptions{
		AuthPublicKey:    pk,
		ListenAddr:       listenAt,
		OnSessionMessage: h.OnSessionMessage,
		OnSessionStatus:  h.OnSessionStatus,
	}
	svr, err := server.NewTcpServer(svropt)
	if err != nil {
		panic(err)
	}

	defer svr.Stop()
	fmt.Println("server started,listening on ", listenAt)
	go svr.Start()

	WaitShutdown()
}

func main() {
	if err := Run(); err != nil {
		fmt.Println(err)
	}
}

var (
	Name       string = "route"
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

	listenAt := ":8080"
	if c.Args().Len() == 2 {
		listenAt = c.Args().Get(1)
	}
	StartServer(listenAt)
	return nil
}
