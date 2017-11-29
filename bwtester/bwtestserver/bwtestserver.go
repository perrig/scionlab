// bwtestserver application
// For more documentation on the application see:
// https://github.com/perrig/scionlab/blob/master/README.md
// https://github.com/perrig/scionlab/blob/master/bwtester/README.md
package main

import (
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/netsec-ethz/scion/go/lib/snet"
	. "github.com/perrig/scionlab/bwtester/bwtestlib"
)

func printUsage() {
	fmt.Println("bwtestserver -s ServerSCIONAddress")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1,[127.0.0.1]:42002")
}

func main() {
	var (
		serverCCAddrStr string
		serverCCAddr    *snet.Addr
		err             error
		CCConn          *snet.Conn
	)

	// Fetch arguments from command line
	flag.StringVar(&serverCCAddrStr, "s", "", "Server SCION Address")
	flag.Parse()

	// Create the SCION UDP socket
	if len(serverCCAddrStr) > 0 {
		serverCCAddr, err = snet.AddrFromString(serverCCAddrStr)
		if err != nil {
			printUsage()
			Check(err)
		}
	} else {
		printUsage()
		Check(fmt.Errorf("Error, server address needs to be specified with -s"))
	}

	sciondAddr := "/run/shm/sciond/sd" + strconv.Itoa(serverCCAddr.IA.I) + "-" + strconv.Itoa(serverCCAddr.IA.A) + ".sock"
	dispatcherAddr := "/run/shm/dispatcher/default.sock"
	snet.Init(serverCCAddr.IA, sciondAddr, dispatcherAddr)

	ci := strings.LastIndex(serverCCAddrStr, ":")
	if ci < 0 {
		// This should never happen, an error would have been much earlier detected
		Check(fmt.Errorf("Malformed server address"))
	}
	serverISDASIP := serverCCAddrStr[:ci]
	// fmt.Println("serverISDASIP:", serverISDASIP)

	CCConn, err = snet.ListenSCION("udp4", serverCCAddr)
	Check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	for {
		// Handle client requests

		n, clientCCAddr, err := CCConn.ReadFrom(receivePacketBuffer)
		if err != nil {
			// Todo: check error in detail, but for now simply continue
			continue
		}
		// fmt.Println("clientCCAddr:", clientCCAddr.String())

		clientBwp, n := DecodeBwtestParameters(receivePacketBuffer[:n])
		// fmt.Println(clientBwp)

		serverBwp, n := DecodeBwtestParameters(receivePacketBuffer[n:])
		// fmt.Println(serverBwp)

		clientCCAddrStr := clientCCAddr.String()
		ci := strings.LastIndex(clientCCAddrStr, ":")
		if ci < 0 {
			// This should never happen
			Check(fmt.Errorf("Malformed client address"))
		}
		clientISDASIP := clientCCAddrStr[:ci]

		// Address of client data channel (DC)
		ca := clientISDASIP + ":" + strconv.Itoa(int(clientBwp.Port))
		clientDCAddr, err := snet.AddrFromString(ca)
		Check(err)
		// Address of server data channel (DC)
		serverDCAddr, err := snet.AddrFromString(serverISDASIP + ":" + strconv.Itoa(int(serverBwp.Port)))
		Check(err)

		// Data channel connection
		DCConn, err := snet.DialSCION("udp4", serverDCAddr, clientDCAddr)
		Check(err)
		fmt.Println("serverDCAddr -> clientDCAddr", serverDCAddr, "->", clientDCAddr)

		go HandleDCConnReceive(clientBwp, DCConn)
		go HandleDCConnSend(serverBwp, DCConn)

		sendPacketBuffer[0] = byte(1)
		n, err = CCConn.WriteTo(sendPacketBuffer[:1], clientCCAddr)
		Check(err)

		// Wait a generous amount of time
		if clientBwp.BwtestDuration > serverBwp.BwtestDuration {
			time.Sleep(clientBwp.BwtestDuration + GracePeriod)
		} else {
			time.Sleep(serverBwp.BwtestDuration + GracePeriod)
		}
		DCConn.Close()
		Check(err)
	}
}
