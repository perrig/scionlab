// bwtestserver application
// For more documentation on the application see:
// https://github.com/perrig/scionlab/blob/master/README.md
// https://github.com/perrig/scionlab/blob/master/bwtester/README.md
package main

import (
	"bytes"
	"crypto/rand"
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/netsec-ethz/scion/go/lib/snet"
	. "github.com/perrig/scionlab/bwtester/bwtestlib"
)

const (
	DefaultBwtestParameters = "5,1000,5"
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
	fmt.Println("imagefetcher -c ClientSCIONAddress -s ServerSCIONAddress -cs t,size,num -sc t,size,num")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1011,[192.33.93.166]:42002")
	fmt.Println("cs specifies time duration (seconds), packet size (bytes), number of packets of client->server test")
	fmt.Println("sc specifies time duration, packet size, number of packets of server->client test")
	fmt.Println("Default test parameters", DefaultBwtestParameters)
}

// Input format (time duration, packet size, number of packets), no spaces
func parseBwtestParameters(s string) BwtestParameters {
	// Since "-" is not part of the parse string, all numbers read are positive
	re := regexp.MustCompile("[0-9]+")
	a := re.FindAllString(s, -1)
	if len(a) != 3 {
		Check(fmt.Errorf("Incorrect number of arguments, need 3 values for bwtestparameters"))
	}

	a1, err := strconv.Atoi(a[0])
	Check(err)
	d := time.Second * time.Duration(a1)
	if d > MaxDuration {
		Check(fmt.Errorf("Duration is exceeding MaxDuration:", a1, ">", MaxDuration/time.Second))
	}
	a2, err := strconv.Atoi(a[1])
	Check(err)
	if a2 < MinPacketSize {
		a2 = MinPacketSize
	}
	if a2 > MaxPacketSize {
		a2 = MaxPacketSize
	}
	a3, err := strconv.Atoi(a[2])
	Check(err)
	key := prepareAESKey()
	return BwtestParameters{d, a2, a3, key, 0}
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

		// Address of client data channel (DC)
		clientDCAddr *snet.Addr
		// Address of server data channel (DC)
		serverDCAddr *snet.Addr
		// Data channel connection
		DCConn *snet.Conn

		clientBwpStr string
		clientBwp    BwtestParameters
		serverBwpStr string
		serverBwp    BwtestParameters

		err   error
		tzero time.Time // initialized to "zero" time
	)

	flag.StringVar(&clientCCAddrStr, "c", "", "Client SCION Address")
	flag.StringVar(&serverCCAddrStr, "s", "", "Server SCION Address")
	flag.StringVar(&serverBwpStr, "sc", DefaultBwtestParameters, "Server->Client test parameter")
	flag.StringVar(&clientBwpStr, "cs", DefaultBwtestParameters, "Client->Server test parameter")

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
	// fmt.Println("clientCCAddr -> serverCCAddr", clientCCAddr, "->", serverCCAddr)

	ci := strings.LastIndex(serverCCAddrStr, ":")
	if ci < 0 {
		// This should never happen, an error would have been much earlier detected
		Check(fmt.Errorf("Malformed server address"))
	}
	serverISDASIP = serverCCAddrStr[:ci]
	serverPort, err = strconv.Atoi(serverCCAddrStr[ci+1:])
	Check(err)
	// fmt.Println("serverISDASIP:", serverISDASIP)
	// fmt.Println("serverPort:", serverPort)

	ci = strings.LastIndex(clientCCAddrStr, ":")
	if ci < 0 {
		// This should never happen, an error would have been much earlier detected
		Check(fmt.Errorf("Malformed client address"))
	}
	clientISDASIP = clientCCAddrStr[:ci]
	clientPort, err = strconv.Atoi(clientCCAddrStr[ci+1:])
	Check(err)
	// fmt.Println("clientISDASIP:", clientISDASIP)
	// fmt.Println("clientPort:", clientPort)

	// Address of client data channel (DC)
	clientDCAddr, err = snet.AddrFromString(clientISDASIP + ":" + strconv.Itoa(clientPort+1))
	Check(err)
	// Address of server data channel (DC)
	serverDCAddr, err = snet.AddrFromString(serverISDASIP + ":" + strconv.Itoa(serverPort+1))
	Check(err)

	// Data channel connection
	DCConn, err = snet.DialSCION("udp4", clientDCAddr, serverDCAddr)
	Check(err)

	clientBwp = parseBwtestParameters(clientBwpStr)
	clientBwp.Port = uint16(clientPort + 1)
	serverBwp = parseBwtestParameters(serverBwpStr)
	serverBwp.Port = uint16(serverPort + 1)
	fmt.Println("\nTest parameters:")
	fmt.Println("clientDCAddr -> serverDCAddr", clientDCAddr, "->", serverDCAddr)
	fmt.Printf("client->server: %d seconds, %d bytes, %d packets\n",
		int(clientBwp.BwtestDuration/time.Second), clientBwp.PacketSize, clientBwp.NumPackets)
	fmt.Printf("server->client: %d seconds, %d bytes, %d packets\n",
		int(serverBwp.BwtestDuration/time.Second), serverBwp.PacketSize, serverBwp.NumPackets)

	t := time.Now()
	expFinishTimeSend := t.Add(serverBwp.BwtestDuration + MaxRTT + GracePeriodSend)
	expFinishTimeReceive := t.Add(clientBwp.BwtestDuration + MaxRTT + StragglerWaitPeriod)
	res := BwtestResult{-1, -1, clientBwp.PrgKey, expFinishTimeReceive}
	var resLock sync.Mutex
	if expFinishTimeReceive.Before(expFinishTimeSend) {
		// The receiver will close the DC connection, so it will wait long enough until the
		// sender is also done
		res.ExpectedFinishTime = expFinishTimeSend
	}

	go HandleDCConnReceive(&serverBwp, DCConn, &res, &resLock)

	pktbuf := make([]byte, 2000)
	pktbuf[0] = 'N' // Request for new bwtest
	n := EncodeBwtestParameters(&clientBwp, pktbuf[1:])
	l := n + 1
	n = EncodeBwtestParameters(&serverBwp, pktbuf[l:])
	l = l + n

	numtries := 0
	for numtries < MaxTries {
		_, err = CCConn.Write(pktbuf[:l])
		Check(err)

		err = CCConn.SetReadDeadline(time.Now().Add(MaxRTT))
		Check(err)
		n, err = CCConn.Read(pktbuf)
		if err != nil {
			// A timeout likely happened, see if we should adjust the expected finishing time
			expFinishTimeReceive = time.Now().Add(clientBwp.BwtestDuration + MaxRTT + StragglerWaitPeriod)
			resLock.Lock()
			if res.ExpectedFinishTime.Before(expFinishTimeReceive) {
				res.ExpectedFinishTime = expFinishTimeReceive
			}
			resLock.Unlock()

			numtries++
			continue
		}
		// Remove read deadline
		err = CCConn.SetReadDeadline(tzero)
		Check(err)

		if n != 2 {
			fmt.Println("Incorrect server response, trying again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if pktbuf[0] != 'N' {
			fmt.Println("Incorrect server response, trying again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if pktbuf[1] != 0 {
			// The server asks us to wait for some amount of time
			time.Sleep(time.Second * time.Duration(int(pktbuf[1])))
			// Don't increase numtries in this case
			continue
		}

		// Everything was successful, exit the loop
		break
	}

	if numtries == MaxTries {
		Check(fmt.Errorf("Error, could not receive a server response, MaxTries attempted without success."))
	}

	go HandleDCConnSend(&clientBwp, DCConn)

	// Wait until sender and receiver completed
	// The reason we're not simply using a channel for synchronization is because the server
	// needs to estimate for how long the test is still running when responding to new client
	// requests. So we use the same mechanism here on the client side, although it is a bit
	// more complex, see: https://github.com/perrig/scionlab/blob/master/bwtester/README.md
	t = time.Now()
	resLock.Lock()
	eft := res.ExpectedFinishTime
	resLock.Unlock()
	for t.Before(eft) {
		time.Sleep(eft.Sub(t))
		// The reason that this is a loop is because res.ExpectedFinishTime is updated in the
		// receive function when the first packet arrived. Thus, the ExpectedFinishTime may
		// have gotten longer while it was sleeping
		resLock.Lock()
		eft = res.ExpectedFinishTime
		resLock.Unlock()
		t = time.Now()
	}

	fmt.Println("\nS->C results")
	att := 8 * serverBwp.PacketSize * serverBwp.NumPackets / int(serverBwp.BwtestDuration/time.Second)
	ach := 8 * serverBwp.PacketSize * res.CorrectlyReceived / int(serverBwp.BwtestDuration/time.Second)
	fmt.Println("Attempted bandwidth:", att, "bps /", att/1000000, "Mbps")
	fmt.Println("Achieved bandwidth:", ach, "bps / ", ach/1000000, "Mbps")
	fmt.Println("Loss rate:", (serverBwp.NumPackets-res.CorrectlyReceived)/serverBwp.NumPackets, "%")

	// Fetch results from server
	numtries = 0
	for numtries < MaxTries {
		pktbuf[0] = 'R'
		copy(pktbuf[1:], clientBwp.PrgKey)
		_, err = CCConn.Write(pktbuf[:1+len(clientBwp.PrgKey)])
		Check(err)

		err = CCConn.SetReadDeadline(time.Now().Add(MaxRTT))
		Check(err)
		n, err = CCConn.Read(pktbuf)
		if err != nil {
			numtries++
			continue
		}
		// Remove read deadline
		err = CCConn.SetReadDeadline(tzero)
		Check(err)

		if n < 2 {
			numtries++
			continue
		}
		if pktbuf[0] != 'R' {
			numtries++
			continue
		}
		if pktbuf[1] != byte(0) {
			// Error case
			if pktbuf[1] == byte(127) {
				Check(fmt.Errorf("Results could not be found or PRG key was incorrect, abort"))
			}
			// pktbuf[1] contains number of seconds to wait for results
			fmt.Println("We need to sleep for", pktbuf[1], "seconds before we can get the results")
			time.Sleep(time.Duration(pktbuf[1]) * time.Second)
			// We don't increment numtries as this was not a lost packet or other communication error
			continue
		}

		sres, n1, err := DecodeBwtestResult(pktbuf[2:])
		if err != nil {
			fmt.Println("Decoding error, try again")
			numtries++
			continue
		}
		if n1+2 < n {
			fmt.Println("Insufficient number of bytes received, try again")
			time.Sleep(Timeout)
			numtries++
			continue
		}
		if !bytes.Equal(clientBwp.PrgKey, sres.PrgKey) {
			fmt.Println("PRG Key returned from server incorrect, this should never happen")
			numtries++
			continue
		}
		fmt.Println("\nC->S results")
		att = 8 * clientBwp.PacketSize * clientBwp.NumPackets / int(clientBwp.BwtestDuration/time.Second)
		ach = 8 * clientBwp.PacketSize * sres.CorrectlyReceived / int(clientBwp.BwtestDuration/time.Second)
		fmt.Println("Attempted bandwidth:", att, "bps /", att/1000000, "Mbps")
		fmt.Println("Achieved bandwidth:", ach, "bps /", ach/1000000, "Mbps")
		fmt.Println("Loss rate:", (clientBwp.NumPackets-sres.CorrectlyReceived)/clientBwp.NumPackets, "%")
		return
	}

	fmt.Println("Error, could not fetch server results, MaxTries attempted without success.")
}
