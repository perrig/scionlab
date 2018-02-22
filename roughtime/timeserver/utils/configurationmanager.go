package utils;

import (
    "fmt"
    "os"
    "encoding/json"
    "crypto/rand"
    "io/ioutil"
    "encoding/hex"
    "bufio"
    "bytes"

    "golang.org/x/crypto/ed25519"

    "github.com/scionproto/scion/go/lib/snet"

    "roughtime.googlesource.com/go/config"
)

func generateKeypair(privateKeyFile string)(rootPrivate ed25519.PrivateKey, rootPublic ed25519.PublicKey, err error){
    rootPublic, rootPrivate, err = ed25519.GenerateKey(rand.Reader)
    if err != nil {
        err=fmt.Errorf("Error creating server keypair %v", err)
        return
    }

    pkFile, err := os.Create(privateKeyFile)
    if err != nil {
        err=fmt.Errorf("Error creating private key file %v", err)
        return
    }
    defer pkFile.Close()

    w := bufio.NewWriter(pkFile)
    _, err = fmt.Fprintf(w, "%x", rootPrivate)
    if err != nil{
        err=fmt.Errorf("Error writing private key to file %v", err)
        return
    }
    w.Flush()

    return
}

func createConfigFile(pubKey ed25519.PublicKey, address *snet.Addr, serverName, configFile string)(error){
    configInformation := config.Server{
        Name:          serverName,
        PublicKeyType: "ed25519",   // For now this is fixed
        PublicKey:     pubKey,
        Addresses: []config.ServerAddress{
            config.ServerAddress{
                Protocol: "udp4",   // For now this is fixed
                Address:  address.String(),
            },
        },
    }

    jsonBytes, err := json.MarshalIndent(configInformation, "", "  ")
    if err != nil {
        return err
    }

    file, err := os.Create(configFile)
    if err != nil {
        return fmt.Errorf("Error creating config file %v", err)
    }
    defer file.Close()

    _, err = file.Write(jsonBytes)
    if err != nil {
        return fmt.Errorf("Error writing configuration to file %v", err)
    }

    file.Sync()

    return nil
}

func GenerateServerConfiguration(address, privateKeyFile, configFile, serverName string)(error){
    serverAddr, err := snet.AddrFromString(address)
    if err!= nil{
        return fmt.Errorf("Invalid scion address! %v", err)
    }

    _, public, err:=generateKeypair(privateKeyFile)
    if err!=nil{
        return err
    }

    err = createConfigFile(public, serverAddr, serverName, configFile)
    if err!=nil{
        return err
    }

    return nil 
}

func LoadServerConfiguration(configurationPath string)(*config.Server, error){
    fileData, err := ioutil.ReadFile(configurationPath)
    if err != nil {
        return nil, fmt.Errorf("Error opening configuration file %v",err)
    }

    var serverConfig config.Server
    if err := json.Unmarshal(fileData, &serverConfig); err != nil {
        return nil, fmt.Errorf("Error parsing configuration file %v", err)
    }

    return &serverConfig, nil
}

func ReadPrivateKey(privateKeyFile string)(ed25519.PrivateKey, error){
    privateKeyHex, err := ioutil.ReadFile(privateKeyFile)
    if err != nil {
        return nil, fmt.Errorf("Cannot open private key file %v", err)
    }

    privateKey, err := hex.DecodeString(string(bytes.TrimSpace(privateKeyHex)))
    if err != nil {
        return nil, fmt.Errorf("Cannot parse private key %v", err)
    }

    return privateKey, nil
}