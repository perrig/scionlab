// imageserver application. This simple image server sends images via a series of UDP requests.
// For more documentation on the application see:
// https://github.com/perrig/scionlab/blob/master/README.md
// https://github.com/perrig/scionlab/camerapp/blob/master/README.md
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/netsec-ethz/scion/go/lib/snet"
)

const (
	MaxFileNameLength int = 255

	// After an image was stored for this amount of time, it will be deleted
	MaxFileAge time.Duration = time.Minute * 10

	// Duration after which an image is still available for download, but it will not be listed any more in new requests
	MaxFileAgeGracePeriod time.Duration = time.Minute * 1

	// Interval after which the file system is read to check for new images
	imageReadInterval time.Duration = time.Second * 59
)

type ImageFileType struct {
	name     string
	size     uint32
	content  []byte
	readTime time.Time
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

var currentFiles []ImageFileType
var currentFilesLock sync.Mutex

func HandleImageFiles() {
	for {
		// Read the directory and look for new .jpg images
		direntries, err := ioutil.ReadDir(".")
		check(err)

		for _, entry := range direntries {
			if entry.IsDir() {
				continue
			}
			if !strings.HasSuffix(entry.Name(), ".jpg") {
				continue
			}
			if len(entry.Name()) > MaxFileNameLength {
				continue
			}
			// Check if we've already read in the image
			foundImage := false
			currentFilesLock.Lock()
			for _, ift := range currentFiles {
				if strings.Compare(entry.Name(), ift.name) == 0 {
					foundImage = true
					break
				}
			}
			if !foundImage {
				fileContents, err := ioutil.ReadFile(entry.Name())
				check(err)
				newFile := ImageFileType{entry.Name(), uint32(entry.Size()), fileContents, time.Now()}
				currentFiles = append(currentFiles, newFile)
			}
			currentFilesLock.Unlock()
		}
		// Check if an image should be deleted
		now := time.Now()
		deleteEntry := -1
		// For simplicity, only one entry can be deleted in each iteration
		currentFilesLock.Lock()
		for i, ift := range currentFiles {
			age := now.Sub(ift.readTime)
			if age > MaxFileAge+MaxFileAgeGracePeriod {
				deleteEntry = i
			}
		}
		currentFilesLock.Unlock()
		if deleteEntry >= 0 {
			currentFilesLock.Lock()
			err = os.Remove(currentFiles[deleteEntry].name)
			check(err)
			copy(currentFiles[deleteEntry:], currentFiles[deleteEntry+1:])
			currentFiles = currentFiles[:len(currentFiles)-1]
			currentFilesLock.Unock()
		}
		time.Sleep(imageReadInterval)
	}
}

func printUsage() {
	fmt.Println("imageserver -s ServerSCIONAddress")
	fmt.Println("The SCION address is specified as ISD-AS,[IP Address]:Port")
	fmt.Println("Example SCION address 1-1,[127.0.0.1]:42002")
}

func main() {
	currentFiles = make([]ImageFileType, 0)

	go HandleImageFiles()

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

	sciondAddr := "/run/shm/sciond/sd" + strconv.Itoa(server.IA.I) + "-" + strconv.Itoa(server.IA.A) + ".sock"
	dispatcherAddr := "/run/shm/dispatcher/default.sock"
	snet.Init(server.IA, sciondAddr, dispatcherAddr)

	udpConnection, err = snet.ListenSCION("udp4", server)
	check(err)

	receivePacketBuffer := make([]byte, 2500)
	sendPacketBuffer := make([]byte, 2500)
	for {
		// Handle client requests
		n, remoteUDPaddress, err := udpConnection.ReadFrom(receivePacketBuffer)
		if err != nil {
			if operr, ok := err.(*snet.OpError); ok {
				// This is an OpError, could be SCMP, in which case continue
				if operr.SCMP() != nil {
					continue
				}
			}
			// If it's not an snet SCMP error, then it's something more serious and fail
			check(err)
		}
		if n > 0 {
			if receivePacketBuffer[0] == 'L' && len(currentFiles) > 0 {
				mostRecentImage := currentFiles[len(currentFiles)-1]
				sendLen := len(mostRecentImage.name)
				if sendLen > MaxFileNameLength {
					fmt.Println("Error, file size too long, should never happen")
					continue
				}
				sendPacketBuffer[0] = 'L'
				sendPacketBuffer[1] = byte(sendLen)
				copy(sendPacketBuffer[2:], []byte(mostRecentImage.name))
				sendLen = sendLen + 2
				binary.LittleEndian.PutUint32(sendPacketBuffer[sendLen:], mostRecentImage.size)
				sendLen = sendLen + 4
				n, err = udpConnection.WriteTo(sendPacketBuffer[:sendLen], remoteUDPaddress)
				check(err)
			} else if receivePacketBuffer[0] == 'G' && n > 1 {
				filenameLen := int(receivePacketBuffer[1])
				if n >= (2 + filenameLen + 8) {
					filename := string(receivePacketBuffer[2 : filenameLen+2])
					startByte := binary.LittleEndian.Uint32(receivePacketBuffer[filenameLen+2:])
					endByte := binary.LittleEndian.Uint32(receivePacketBuffer[filenameLen+6:])
					currentFilesLock.Lock()
					for _, ift := range currentFiles {
						if strings.Compare(filename, ift.name) == 0 {
							if endByte > startByte && endByte <= (ift.size+1) {
								sendPacketBuffer[0] = 'G'
								// Copy startByte and endByte from request packet
								copy(sendPacketBuffer[1:], receivePacketBuffer[filenameLen+2:filenameLen+10])
								// Copy image contents
								copy(sendPacketBuffer[9:], ift.content[startByte:endByte])
								sendLen := 9 + endByte - startByte
								n, err = udpConnection.WriteTo(sendPacketBuffer[:sendLen], remoteUDPaddress)
								check(err)
							}
							break
						}
					}
					currentFilesLock.Unlock()
				}
			}
		}
	}
}
