package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
)

var (
	stuff []byte
	out   []byte
	out2  []byte
)

func handler1(w http.ResponseWriter, r *http.Request) {
	n, err := w.Write(out)
	if n != len(out) {
		fmt.Println("Short write!")
	}
	if err != nil {
		fmt.Println(err)
	}
}

func handler2(w http.ResponseWriter, r *http.Request) {
	n, err := w.Write(out2)
	if n != len(out2) {
		fmt.Println("Short write!")
	}
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	stuff = make([]byte, 32*1024)
	out = make([]byte, len(stuff)*2)
	out2 = []byte("hello world")
	for i := 0; i < len(stuff); i++ {
		stuff[i] = byte(i % 256)
	}
	hex.Encode(out, stuff)
	http.HandleFunc("/test1", handler1)
	http.HandleFunc("/test2", handler2)

	log.Println("Listening :20002")
	log.Fatal(http.ListenAndServe(":20002", nil))
}
