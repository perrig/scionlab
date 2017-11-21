// bwtestserver application
// For more documentation on the application see:
// https://github.com/perrig/scionlab/blob/master/README.md
// https://github.com/perrig/scionlab/blob/master/bwtester/README.md
package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/netsec-ethz/scion/go/lib/snet"
	// . "github.com/perrig/scionlab/bwtester/bwtestlib"
	. "adrian/bwtestlib"
)

func prepareAESKey() []byte {
	key := make([]byte, 16)
	n, err := rand.Read(key)
	Check(err)
	if n != 16 {
		Check(fmt.Errorf("Did not obtain 16 bytes of random information, only received", n))
	}
	return key
}

func printUsage() {
	fmt.Println("imagefetcher -c ClientSCIONAddress -s ServerSCIONAddress")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1011,[192.33.93.166]:42002")
}

func main() {
	var (
		clientCCAddrStr string
		serverCCAddrStr string
		clientISDASIP   string
		serverISDASIP   string
		clientPort      int
		serverPort      int
		// Address of client control channel (CC)
		clientCCAddr *snet.Addr
		// Address of server control channel (CC)
		serverCCAddr *snet.Addr
		// Control channel connection
		CCConn *snet.Conn

		// clientDCAddrStr string
		// serverDCAddrStr string
		// Address of client data channel (DC)
		clientDCAddr *snet.Addr
		// Address of server data channel (DC)
		serverDCAddr *snet.Addr
		// Data channel connection
		DCConn *snet.Conn

		err error
	)

	flag.StringVar(&clientCCAddrStr, "c", "", "Client SCION Address")
	flag.StringVar(&serverCCAddrStr, "s", "", "Server SCION Address")

	flag.Parse()

	// Create SCION UDP socket
	if len(clientCCAddrStr) > 0 {
		clientCCAddr, err = snet.AddrFromString(clientCCAddrStr)
		Check(err)
	} else {
		printUsage()
		Check(fmt.Errorf("Error, client address needs to be specified with -c"))
	}
	if len(serverCCAddrStr) > 0 {
		serverCCAddr, err = snet.AddrFromString(serverCCAddrStr)
		Check(err)
	} else {
		printUsage()
		Check(fmt.Errorf("Error, server address needs to be specified with -s"))
	}

	sciondAddr := "/run/shm/sciond/sd" + strconv.Itoa(clientCCAddr.IA.I) + "-" + strconv.Itoa(clientCCAddr.IA.A) + ".sock"
	dispatcherAddr := "/run/shm/dispatcher/default.sock"
	snet.Init(clientCCAddr.IA, sciondAddr, dispatcherAddr)
	CCConn, err = snet.DialSCION("udp4", clientCCAddr, serverCCAddr)
	Check(err)
	fmt.Println("clientCCAddr -> serverCCAddr", clientCCAddr, "->", serverCCAddr)

	ci := strings.LastIndex(serverCCAddrStr, ":")
	if ci < 0 {
		// This should never happen, an error would have been much earlier detected
		Check(fmt.Errorf("Malformed server address"))
	}
	serverISDASIP = serverCCAddrStr[:ci]
	serverPort, err = strconv.Atoi(serverCCAddrStr[ci+1:])
	Check(err)
	fmt.Println("serverISDASIP:", serverISDASIP)
	fmt.Println("serverPort:", serverPort)

	ci = strings.LastIndex(clientCCAddrStr, ":")
	if ci < 0 {
		// This should never happen, an error would have been much earlier detected
		Check(fmt.Errorf("Malformed client address"))
	}
	clientISDASIP = clientCCAddrStr[:ci]
	clientPort, err = strconv.Atoi(clientCCAddrStr[ci+1:])
	Check(err)
	fmt.Println("clientISDASIP:", clientISDASIP)
	fmt.Println("clientPort:", clientPort)

	// Address of client data channel (DC)
	clientDCAddr, err = snet.AddrFromString(clientISDASIP + ":" + strconv.Itoa(clientPort+1))
	Check(err)
	// Address of server data channel (DC)
	serverDCAddr, err = snet.AddrFromString(serverISDASIP + ":" + strconv.Itoa(serverPort+1))
	Check(err)

	// Data channel connection
	DCConn, err = snet.DialSCION("udp4", clientDCAddr, serverDCAddr)
	Check(err)
	fmt.Println("clientDCAddr -> serverDCAddr", clientDCAddr, "->", serverDCAddr)

	// Prepare arguments
	clientKey := prepareAESKey()
	serverKey := prepareAESKey()

	clientBwp := BwtestParameters{time.Second * 3,
		100,
		5,
		clientKey,
		uint16(clientPort + 1)}

	serverBwp := BwtestParameters{time.Second * 3,
		100,
		5,
		serverKey,
		uint16(serverPort + 1)}

	go HandleDCConnReceive(&serverBwp, DCConn)

	pktbuf := make([]byte, 2000)
	n := EncodeBwtestParameters(&clientBwp, pktbuf)
	l := n
	n = EncodeBwtestParameters(&serverBwp, pktbuf[n:])
	l = l + n

	_, err = CCConn.Write(pktbuf[:l])
	Check(err)

	n, err = CCConn.Read(pktbuf)
	Check(err)

	go HandleDCConnSend(&clientBwp, DCConn)

	// Wait for a very generous amount of time
	time.Sleep(clientBwp.BwtestDuration + serverBwp.BwtestDuration + GracePeriod)
}
