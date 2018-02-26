package main

import (
	"log"
	"os"
	"fmt"
    "crypto/rand"
    "time"

    "golang.org/x/crypto/ed25519"
	"github.com/perrig/scionlab/roughtime/utils"
	"github.com/scionproto/scion/go/lib/snet"
	"roughtime.googlesource.com/go/protocol"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("timeserver", "SCION roughtime server")

	configCommand           = app.Command("configure", "Generate server configuration files")
	serverAddress    = configCommand.Arg("address", "Server's SCION address").Required().String()
	outputKeyFile    = configCommand.Flag("private_key", "Name of a file where private key will be stored").Default("private.key").String()
	outputConfigFile = configCommand.Flag("config_file", "Name of configuration file where server details will be stored").Default("config.json").String()
	serverName       = configCommand.Flag("name", "Server name").String()
	
	runCommand             = app.Command("run", "Run roughtime server")
	inputKeyFile    = runCommand.Arg("private_key", "Name of a file containing private key").Default("private.key").String()
	inputConfigFile = runCommand.Arg("config_file", "Name of configuration file with server config").Default("config.json").String()
)

func checkErr(action string, err error){
	if err!=nil {
		log.Panicf("%s caused an error: %v", action, err)
	}
}

func main() {
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	// Configuring roughtime server
	case configCommand.FullCommand():
		err:=utils.GenerateServerConfiguration(*serverAddress, *outputKeyFile, *outputConfigFile, *serverName)
		checkErr("Configuring server", err)

	// Running server
	case runCommand.FullCommand():
		runServers(*inputConfigFile, *inputKeyFile)
	}
}

func runServers(configurationFile, privateKeyFile string){
	serverConfig, err := utils.LoadServerConfiguration(configurationFile)
	checkErr("Loading server configuration", err)

	privateKey, err := utils.ReadPrivateKey(privateKeyFile)
	checkErr("Loading private key", err)

	for _, addr := range serverConfig.Addresses{
		//TODO: run in goroutine
		serveRequests(addr.Address, addr.Protocol, privateKey)
	}
}

func serveRequests(bindAddress, connectionProtocol string, privateKey []byte){
	sAddr, err := utils.InitSCIONConnection(bindAddress)
	checkErr("Initializing SCION connection", err)

	conn, err := snet.ListenSCION(connectionProtocol, sAddr)
	checkErr("Starting to listen", err)

	onlinePublicKey, onlinePrivateKey, err := ed25519.GenerateKey(rand.Reader)
	checkErr("Generate temp private key", err)
	
	// As this is just an example, the certificate is created covering the
	// maximum possible range.
	cert, err := protocol.CreateCertificate(0, ^uint64(0), onlinePublicKey, privateKey)
	checkErr("Generating certificate", err)
	
	var packetBuf [protocol.MinRequestSize]byte

	for {
		n, sourceAddr, err := conn.ReadFrom(packetBuf[:])
		if err != nil {
			log.Print(err)
		}

		if n < protocol.MinRequestSize {
			continue
		}

		packet, err := protocol.Decode(packetBuf[:n])
		if err != nil {
			continue
		}

		nonce, ok := packet[protocol.TagNonce]
		if !ok || len(nonce) != protocol.NonceSize {
			continue
		}

		midpoint := uint64(time.Now().UnixNano() / 1000)
		radius := uint32(1000000)
		fmt.Printf("Timestamp: %d ", midpoint)

		replies, err := protocol.CreateReplies([][]byte{nonce}, midpoint, radius, cert, onlinePrivateKey)
		if err != nil {
			log.Print(err)
			continue
		}

		if len(replies) != 1 {
			continue
		}

		conn.WriteTo(replies[0], sourceAddr)
	}
}