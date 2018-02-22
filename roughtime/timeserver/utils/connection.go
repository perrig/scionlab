package utils;

import (
    "log"
    "fmt"

    "github.com/scionproto/scion/go/lib/snet"
)

func getSciondAddr(scionAddr *snet.Addr)(string){
    return fmt.Sprintf("/run/shm/sciond/sd%d-%d.sock", scionAddr.IA.I, scionAddr.IA.A)
}

func getDispatcherAddr(scionAddr *snet.Addr)(string){
    return "/run/shm/dispatcher/default.sock"
}

func InitSCIONConnection(serverAddress string)(*snet.Addr, error){
    log.Println("Initializing SCION connection")

    serverCCAddr, err := snet.AddrFromString(serverAddress)
    if err != nil {
        return nil, err
    }

    err = snet.Init(serverCCAddr.IA, getSciondAddr(serverCCAddr), getDispatcherAddr(serverCCAddr))
    if err != nil {
        return serverCCAddr, err
    }

    return serverCCAddr, nil
}

