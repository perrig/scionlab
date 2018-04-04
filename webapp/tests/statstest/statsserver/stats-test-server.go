// stats-test-server application
// Python output feed test app based on: https://github.com/perrig/scionlab/tree/master/sensorapp
package main

import (
    "bufio"
    "flag"
    "fmt"
    "github.com/scionproto/scion/go/lib/snet"
    "log"
    "os"
    "strconv"
    "strings"
    "sync"
)

const (
    TIMESTRING             string = "Time"
    TIMEFORMAT             string = "2006/01/02 15:04:05"
    SEPARATORSTRING        string = ": "
    TIMEANDSEPARATORSTRING string = TIMESTRING + SEPARATORSTRING
)

func check(e error) {
    if e != nil {
        log.Fatal(e)
    }
}

var sensorData map[string]string
var sensorDataLock sync.Mutex

func init() {
    sensorData = make(map[string]string)
}

// Obtains input from sensor observation application
func parseInput() {
    input := bufio.NewScanner(os.Stdin)
    for input.Scan() {
        line := input.Text()
        index := strings.Index(line, TIMEANDSEPARATORSTRING)
        if index == 0 {
            // We found a time string, format in case parsing is desired: 2017/11/16 21:29:49
            timestr := line[len(TIMEANDSEPARATORSTRING):]
            sensorDataLock.Lock()
            sensorData[TIMESTRING] = timestr
            sensorDataLock.Unlock()
            continue
        }
        index = strings.Index(line, SEPARATORSTRING)
        if index > 0 {
            sensorType := line[:index]
            sensorDataLock.Lock()
            sensorData[sensorType] = line
            sensorDataLock.Unlock()
        }
    }
}

func printUsage() {
    fmt.Println("stats-test-server -s ServerSCIONAddress")
    fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
    fmt.Println("Example SCION address 1-1,[127.0.0.1]:42002")
}

func main() {
    go parseInput()

    var (
        serverAddress string

        err    error
        server *snet.Addr

        udpConnection *snet.Conn
    )

    // Fetch arguments from command line
    flag.StringVar(&serverAddress, "s", "", "Server SCION Address")
    flag.Parse()

    // Create the SCION UDP socket
    if len(serverAddress) > 0 {
        server, err = snet.AddrFromString(serverAddress)
        check(err)
    } else {
        printUsage()
        check(fmt.Errorf("Error, server address needs to be specified with -s"))
    }

    sciondAddr := "/run/shm/sciond/sd" + strconv.Itoa(int(server.IA.I)) + "-" + strconv.Itoa(int(server.IA.A)) + ".sock"
    dispatcherAddr := "/run/shm/dispatcher/default.sock"
    snet.Init(server.IA, sciondAddr, dispatcherAddr)

    udpConnection, err = snet.ListenSCION("udp4", server)
    check(err)

    receivePacketBuffer := make([]byte, 2500)
    sendPacketBuffer := make([]byte, 2500)
    for {
        _, clientAddress, err := udpConnection.ReadFrom(receivePacketBuffer)
        check(err)

        // Packet received, send back response to same client
        var sensorValues string
        var timeString string
        sensorDataLock.Lock()
        for k, v := range sensorData {
            if strings.Index(k, TIMESTRING) == 0 {
                timeString = v
            } else {
                sensorValues = sensorValues + v + "\n"
            }
        }
        sensorDataLock.Unlock()
        sensorValues = timeString + "\n" + sensorValues
        copy(sendPacketBuffer, sensorValues)

        _, err = udpConnection.WriteTo(sendPacketBuffer[:len(sensorValues)], clientAddress)
        check(err)
    }
}
