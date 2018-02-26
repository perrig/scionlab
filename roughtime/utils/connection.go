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

func InitSCIONConnection(scionAddressString string)(*snet.Addr, error){
    log.Println("Initializing SCION connection")

    scionAddress, err := snet.AddrFromString(scionAddressString)
    if err != nil {
        return nil, err
    }

    err = snet.Init(scionAddress.IA, getSciondAddr(scionAddress), getDispatcherAddr(scionAddress))
    if err != nil {
        return scionAddress, err
    }

    return scionAddress, nil
}

