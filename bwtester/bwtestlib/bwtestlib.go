package bwtestlib

import (
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"log"
	// "net"
	"time"

	"github.com/netsec-ethz/scion/go/lib/snet"
)

const (
	// Maximum duration of a bandwidth test
	MaxDuration time.Duration = time.Second * 10
	// Maximum amount of time to wait for packet reception
	GracePeriod time.Duration = time.Second * 3
)

type BwtestParameters struct {
	BwtestDuration time.Duration
	PacketSize     int
	NumPackets     int
	PrgKey         []byte
	Port           uint16
}

func Check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

// Fill buffer with AES PRG in counter mode
// The value of the ith 16-byte block is simply an encryption of i under the key
func PrgFill(key []byte, iv int, data []byte) {
	i := uint32(iv)
	aesCipher, err := aes.NewCipher(key)
	Check(err)
	s := aesCipher.BlockSize()
	pt := make([]byte, s)
	j := 0
	for j <= len(data)-s {
		binary.LittleEndian.PutUint32(pt, i)
		aesCipher.Encrypt(data, pt)
		j = j + s
		i = i + uint32(s)
	}
	// Check if fewer than BlockSize bytes are required for the final block
	if j < len(data) {
		binary.LittleEndian.PutUint32(pt, i)
		aesCipher.Encrypt(pt, pt)
		copy(data[j:], pt[:len(data)-j])
	}
}

// Encode BwtestParameters into a sufficiently large byte buffer that is passed in, return the number of bytes written
func EncodeBwtestParameters(bwtp *BwtestParameters, buf []byte) int {
	var bb bytes.Buffer
	enc := gob.NewEncoder(&bb)
	err := enc.Encode(*bwtp)
	Check(err)
	copy(buf, bb.Bytes())
	return bb.Len()
}

// Decode BwtestParameters from byte buffer that is passed in, returns BwtestParameters structure and number of bytes consumed
func DecodeBwtestParameters(buf []byte) (*BwtestParameters, int) {
	bb := bytes.NewBuffer(buf)
	is := bb.Len()
	dec := gob.NewDecoder(bb)
	var v BwtestParameters
	err := dec.Decode(&v)
	Check(err)
	return &v, is - bb.Len()
}

func HandleDCConnSend(bwp *BwtestParameters, udpConnection *snet.Conn) {
	sb := make([]byte, bwp.PacketSize)
	i := 0
	t0 := time.Now()
	interPktInterval := bwp.BwtestDuration / time.Duration(bwp.NumPackets)
	for i < bwp.NumPackets {
		// Compute how long to wait
		t1 := time.Now()
		t2 := t0.Add(interPktInterval * time.Duration(i))
		if t1.Before(t2) {
			time.Sleep(t2.Sub(t1))
		} else {
			// We're running a bit behind, sending bandwidth may be insufficient
			// fmt.Println("\nBehind:", t2.Sub(t1))
		}
		// Send packet now
		PrgFill(bwp.PrgKey, i*bwp.PacketSize, sb)
		// Place packet number at the beginning of the packet, overwriting some PRG data
		binary.LittleEndian.PutUint32(sb, uint32(i*bwp.PacketSize))
		n, err := udpConnection.Write(sb)
		Check(err)
		if n < bwp.PacketSize {
			Check(fmt.Errorf("Insufficient number of bytes written:", n, "instead of:", bwp.PacketSize))
		}
		i++
	}
}

func HandleDCConnReceive(bwp *BwtestParameters, udpConnection *snet.Conn) {
	t0 := time.Now()
	finish := t0.Add(bwp.BwtestDuration + GracePeriod)
	numPacketsReceived := 0
	correctlyReceived := 0
	udpConnection.SetReadDeadline(finish)
	// Make the receive buffer a bit larger to enable detection of packets that are too large
	recBuf := make([]byte, bwp.PacketSize+1000)
	cmpBuf := make([]byte, bwp.PacketSize)
	for time.Now().Before(finish) && numPacketsReceived < bwp.NumPackets {
		n, _, err := udpConnection.ReadFrom(recBuf)
		// Ignore errors, todo: detect type of error
		if err != nil {
			continue
		}
		numPacketsReceived++
		// fmt.Print(".")
		if n != bwp.PacketSize {
			// The packet has incorrect size, do not count as a correct packet
			fmt.Println("Incorrect size.", n, "bytes instead of", bwp.PacketSize)
			continue
		}
		// Could consider pre-computing all the packets in a separate goroutine
		// but since computation is usually much higher than bandwidth, this is
		// not necessary
		// Todo: create separate verif function which only compares the packet
		// so that a discrepancy is noticed immediately without generating the
		// entire packet
		iv := int(binary.LittleEndian.Uint32(recBuf))
		PrgFill(bwp.PrgKey, iv, cmpBuf)
		binary.LittleEndian.PutUint32(cmpBuf, uint32(iv))
		if bytes.Equal(recBuf[:bwp.PacketSize], cmpBuf) {
			correctlyReceived++
			// fmt.Print("C")
		}
	}
	fmt.Println("\nnumPacketsReceived:", numPacketsReceived)
	fmt.Println("correctlyReceived:", correctlyReceived)
	fmt.Println("Duration:", time.Now().Sub(t0))
}
