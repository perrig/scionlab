// stats-test-client application
// Python output feed test app based on: https://github.com/perrig/scionlab/tree/master/sensorapp
package main

import (
    "flag"
    "fmt"
    "github.com/scionproto/scion/go/lib/snet"
    "log"
    "strconv"
)

func check(e error) {
    if e != nil {
        log.Fatal(e)
    }
}

func printUsage() {
    fmt.Println("stats-test-client -s ServerSCIONAddress -c ClientSCIONAddress")
    fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
    fmt.Println("Example SCION address 1-1,[127.0.0.1]:42002")
}

func main() {
    var (
        clientAddress string
        serverAddress string

        err    error
        local  *snet.Addr
        remote *snet.Addr

        udpConnection *snet.Conn
    )

    // Fetch arguments from command line
    flag.StringVar(&clientAddress, "c", "", "Client SCION Address")
    flag.StringVar(&serverAddress, "s", "", "Server SCION Address")
    flag.Parse()

    // Create the SCION UDP socket
    if len(clientAddress) > 0 {
        local, err = snet.AddrFromString(clientAddress)
        check(err)
    } else {
        printUsage()
        check(fmt.Errorf("Error, client address needs to be specified with -c"))
    }
    if len(serverAddress) > 0 {
        remote, err = snet.AddrFromString(serverAddress)
        check(err)
    } else {
        printUsage()
        check(fmt.Errorf("Error, server address needs to be specified with -s"))
    }

    sciondAddr := "/run/shm/sciond/sd" + strconv.Itoa(int(local.IA.I)) + "-" + strconv.Itoa(int(local.IA.A)) + ".sock"
    dispatcherAddr := "/run/shm/dispatcher/default.sock"
    snet.Init(local.IA, sciondAddr, dispatcherAddr)

    udpConnection, err = snet.DialSCION("udp4", local, remote)
    check(err)

    receivePacketBuffer := make([]byte, 2500)
    sendPacketBuffer := make([]byte, 0)

    n, err := udpConnection.Write(sendPacketBuffer)
    check(err)

    n, _, err = udpConnection.ReadFrom(receivePacketBuffer)
    check(err)

    fmt.Print(string(receivePacketBuffer[:n]))
}
