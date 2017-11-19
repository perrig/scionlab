// bwtestserver application
// For more documentation on the application see:
// https://github.com/perrig/scionlab/blob/master/README.md
// https://github.com/perrig/scionlab/blob/master/bwtester/README.md
package main

import (
	// "bytes"
	// "crypto/aes"
	"crypto/rand"
	// "encoding/binary"
	// "encoding/gob"
	// "flag"
	"fmt"
	"net"
	// "io/ioutil"
	"time"

	// "github.com/netsec-ethz/scion/go/lib/snet"
	. "github.com/perrig/scionlab/bwtester/bwtestlib"
)

func prepareAESKey() ([]byte) {
	key := make([]byte, 16)
	n, err := rand.Read(key)
	Check(err)
	if n != 16 {
		Check(fmt.Errorf("Did not obtain 16 bytes of random information, only received", n))
	}
	return key
}

func main() {
	// Address of client control channel (CC)
	// Todo: check if port already in use, then pick a different one
	clientCCAddr, err := net.ResolveUDPAddr( "udp", "127.0.0.1:18008" )
	Check( err )
	// Address of server control channel (CC)
	serverCCAddr, err := net.ResolveUDPAddr( "udp", "127.0.0.1:18007" )
	Check( err )
	// Address of client data channel (DC)
	clientDCAddr, err := net.ResolveUDPAddr( "udp", "127.0.0.1:18010" )
	Check( err )
	// Address of server data channel (DC)
	serverDCAddr, err := net.ResolveUDPAddr( "udp", "127.0.0.1:18009" )
	Check( err )

	// Control channel connection
	CCConn, err := net.DialUDP( "udp", clientCCAddr, serverCCAddr )
	Check( err )

	// Data channel connection
	DCConn, err := net.DialUDP( "udp", clientDCAddr, serverDCAddr )
	Check( err )

	// Prepare arguments
	clientKey := prepareAESKey()
	serverKey := prepareAESKey()

	clientBwp := BwtestParameters{time.Second * 3,
		1000,
		500000,
		clientKey,
		18010}

	serverBwp := BwtestParameters{time.Second * 3,
		1000,
		500000,
		serverKey,
		18009}

	go HandleDCConnReceive(&serverBwp, DCConn)

	pktbuf := make([]byte, 2000)
	n := EncodeBwtestParameters(&clientBwp, pktbuf)
	l := n
	n = EncodeBwtestParameters(&serverBwp, pktbuf[n:])
	l = l+n

	_, err = CCConn.Write(pktbuf[:l])
	Check( err )

	n, err = CCConn.Read(pktbuf)
	Check(err)

	go HandleDCConnSend(&clientBwp, DCConn)

	// Wait for a very generous amount of time
	time.Sleep(clientBwp.BwtestDuration + serverBwp.BwtestDuration + GracePeriod)
}
