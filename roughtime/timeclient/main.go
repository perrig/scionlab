package main

import (
    "os"
    "log"
    "time"
    "fmt"


    "gopkg.in/alecthomas/kingpin.v2"
    "github.com/perrig/scionlab/roughtime/utils"
    "github.com/perrig/scionlab/roughtime/timeclient/lib"
    "roughtime.googlesource.com/go/client/monotime"
)

var (
    app = kingpin.New("timeclient", "SCION roughtime client")

    clientAddress = app.Flag("address", "Client's SCION address").Required().String()
    chainFile = app.Flag("chain-file", "Name of a file in which the query chain will be maintained").Default("query-chain.json").String()
    maxChainSize = app.Flag("max-chain-size", "Maximum number of entries in chain file").Default("128").Int()
    serversFile = app.Flag("servers", "Name of the file with server configuration").Default("servers.json").String()
)

const (
    defaultServerQuorum = 3
)

func checkErr(action string, err error){
    if err!=nil {
        log.Panicf("%s caused an error: %v", action, err)
    }
}

func main(){
    app.Parse(os.Args[1:])

    //TODO: Check arguments, if everything is in place

    log.Printf("Client address: %s", *clientAddress)
    cAddr, err := utils.InitSCIONConnection(*clientAddress)
    checkErr("Init SCION", err)

    log.Printf("Scion address is %s", cAddr.String())

    servers, err := utils.LoadServersConfigurationList(*serversFile)
    checkErr("Loading server file", err)

    chain, err := utils.LoadChain(*chainFile)
    checkErr("Loading chain file", err)

    //TODO: Check if number of servers is enough for quorum
    var client lib.Client
    result, err := client.EstablishTime(chain, len(servers), servers, cAddr)
    checkErr("Establishing time", err)

    for serverName, err := range result.ServerErrors {
        log.Printf("Failed to query server %q: %s", serverName, err)
    }

    if result.MonoUTCDelta == nil {
        fmt.Fprintf(os.Stderr, "Failed to get %d servers to agree on the time.\n", len(servers))
    } else {
        nowUTC := time.Unix(0, int64(monotime.Now()+*result.MonoUTCDelta))
        nowRealTime := time.Now()

        fmt.Printf("real-time delta: %s\n", nowRealTime.Sub(nowUTC))
    }
}