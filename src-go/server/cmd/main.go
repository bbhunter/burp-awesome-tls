package main

import "C"

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"server"
	"server/internal"
)

func main() {
	spoofAddr := flag.String("spoof", server.DefaultSpoofProxyAddress, "Spoof proxy address to listen on ([ip:]port)")
	flag.Parse()

	defaultConfig, err := json.Marshal(internal.TransportConfig{
		InterceptProxyAddr: server.DefaultInterceptProxyAddress,
		BurpAddr:           server.DefaultBurpProxyAddress,
		HttpTimeout:        int(internal.DefaultHttpTimeout.Seconds()),
		Fingerprint:        "default",
	})
	if err != nil {
		log.Fatalln(err)
	}

	if err := server.SaveSettings(string(defaultConfig)); err != nil {
		log.Fatalln(err)
	}

	log.Fatalln(server.StartServer(*spoofAddr))
}

//export StartServer
func StartServer(spoofAddr *C.char) *C.char {
	if err := server.StartServer(C.GoString(spoofAddr)); err != nil {
		return C.CString(err.Error())
	}
	return C.CString("")
}

//export StopServer
func StopServer() *C.char {
	if err := server.StopServer(); err != nil {
		return C.CString(err.Error())
	}
	return C.CString("")
}

//export SaveSettings
func SaveSettings(configJson *C.char) *C.char {
	if err := server.SaveSettings(C.GoString(configJson)); err != nil {
		return C.CString(err.Error())
	}

	return C.CString("")
}

//export SmokeTest
func SmokeTest() {
	fmt.Println("smoke test success")
}
