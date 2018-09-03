// Copyright 2018 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/scionproto/scion/go/lib/sciond"
	"github.com/scionproto/scion/go/lib/snet"
	"github.com/scionproto/scion/go/lib/spath"
)

func Check(e error) {
	if e != nil {
		fmt.Println("Fatal error. Exiting.", "err", e)
		os.Exit(1)
	}
}

func main() {
	var (
		err             error
		clientCCAddrStr string
		clientCCAddr    *snet.Addr
	)
	dispatcherPath := "/run/shm/dispatcher/default.sock"
	serverCCAddrStr := "17-ffaa:0:1101,[127.0.0.1]:1122"

	flag.StringVar(&clientCCAddrStr, "local", "", "Client SCION Address")
	flag.Parse()
	if len(clientCCAddrStr) > 0 {
		clientCCAddr, err = snet.AddrFromString(clientCCAddrStr)
		Check(err)
	} else {
		Check(fmt.Errorf("Error, client address needs to be specified with -local"))
	}
	sciondPath := sciond.GetDefaultSCIONDPath(nil)
	err = snet.Init(clientCCAddr.IA, sciondPath, dispatcherPath)
	Check(err)
	serverCCAddr, err := snet.AddrFromString(serverCCAddrStr)
	Check(err)

	pathMgr := snet.DefNetwork.PathResolver()
	pathSet := pathMgr.Query(clientCCAddr.IA, serverCCAddr.IA)
	if len(pathSet) == 0 {
		Check(fmt.Errorf("No paths"))
	}
	i := 0
	minLength, argMinPath := 999, (*sciond.PathReplyEntry)(nil)
	for _, path := range pathSet {
		fmt.Printf("[%2d] %d %s\n", i, len(path.Entry.Path.Interfaces)/2, path.Entry.Path.String())
		if len(path.Entry.Path.Interfaces) < minLength {
			minLength = len(path.Entry.Path.Interfaces)
			argMinPath = path.Entry
		}
		i++
	}
	fmt.Println(argMinPath.Path.String())

	if serverCCAddr.IA.Eq(clientCCAddr.IA) {
		return
	}
	serverCCAddr.Path = spath.New(argMinPath.Path.FwdPath)
	serverCCAddr.Path.InitOffsets()
	serverCCAddr.NextHopHost = argMinPath.HostInfo.Host()
	serverCCAddr.NextHopPort = argMinPath.HostInfo.Port

	conn, err := snet.DialSCION("udp4", clientCCAddr, serverCCAddr)
	Check(err)
	defer conn.Close()
	err = conn.SetWriteDeadline(time.Now().Add(time.Second * 20))
	Check(err)
	nBytes, err := conn.Write([]byte("hello world"))
	Check(err)
	fmt.Println(nBytes)
	fmt.Println("Done.")
	// catch the packet with: sudo tcpdump -i tun0 -n -A -w - |grep -a 'hello'
}
