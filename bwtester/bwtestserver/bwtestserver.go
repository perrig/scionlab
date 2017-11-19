// bwtestserver application
// For more documentation on the application see:
// https://github.com/perrig/scionlab/blob/master/README.md
// https://github.com/perrig/scionlab/blob/master/bwtester/README.md
package main

import (
	// "bytes"
	// "crypto/aes"
	// "crypto/rand"
	// "encoding/binary"
	// "encoding/gob"
	// "flag"
	"fmt"
	"net"
	// "io/ioutil"
	"strconv"
	"time"

	// "github.com/netsec-ethz/scion/go/lib/snet"
	. "github.com/perrig/scionlab/bwtester/bwtestlib"
)

func main() {

	serverUDPaddress, err := net.ResolveUDPAddr( "udp", ":18007" )
	Check( err )
	CCConn, err := net.ListenUDP( "udp", serverUDPaddress )
	Check( err )

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	for {
		// Handle client requests
		
		n, clientUDPaddress, err := CCConn.ReadFromUDP( receivePacketBuffer )
		if err != nil {
			// Todo: check error in detail, but for now simply continue
			continue
		}
		
		clientBwp, n := DecodeBwtestParameters(receivePacketBuffer[:n])
		// fmt.Println(clientBwp)

		serverBwp, n := DecodeBwtestParameters(receivePacketBuffer[n:])
		// fmt.Println(serverBwp)

		// Data channel connection

		// Address of client data channel (DC)
		clientDCAddr, err := net.ResolveUDPAddr( "udp", "127.0.0.1:" + strconv.Itoa(int(clientBwp.Port)))
		Check( err )
		// Address of server data channel (DC)
		serverDCAddr, err := net.ResolveUDPAddr( "udp", "127.0.0.1:" + strconv.Itoa(int(serverBwp.Port)))
		Check( err )

		DCConn, err := net.DialUDP( "udp", serverDCAddr, clientDCAddr )
		Check( err )

		go HandleDCConnReceive(clientBwp, DCConn)
		go HandleDCConnSend(serverBwp, DCConn)

		sendPacketBuffer[0] = byte(1)
		n, err = CCConn.WriteTo(sendPacketBuffer[:1], clientUDPaddress)
		Check(err)

		// Wait for a very generous amount of time
		time.Sleep(clientBwp.BwtestDuration + serverBwp.BwtestDuration + GracePeriod)
		DCConn.Close()
		Check(err)
		fmt.Println("done sleeping, ready!")
	}
}
