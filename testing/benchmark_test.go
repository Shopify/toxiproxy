package main

import (
	"io/ioutil"
	"net/http"
	"testing"
)

// Benchmark numbers:
//
// Toxiproxy 1.1
//
// 1x Toxic Types:
//     BenchmarkDirect             3000            588148 ns/op
//     BenchmarkProxy              2000            999949 ns/op
//     BenchmarkDirectSmall        5000            291324 ns/op
//     BenchmarkProxySmall         3000            504501 ns/op
//
// 10x Toxic Types:
//     BenchmarkDirect             3000            599519 ns/op
//     BenchmarkProxy              2000           1044746 ns/op
//     BenchmarkDirectSmall        5000            280713 ns/op
//     BenchmarkProxySmall         3000            574816 ns/op
//
// Toxiproxy 2.0
//
// No Enabled Toxics:
//     BenchmarkDirect             2000            597998 ns/op
//     BenchmarkProxy              2000            964510 ns/op
//     BenchmarkDirectSmall       10000            287448 ns/op
//     BenchmarkProxySmall         5000            560694 ns/op

// BenchmarkDirect tests the backend server directly, use 64k random endpoint
func BenchmarkDirect(b *testing.B) {
	client := http.Client{}
	for i := 0; i < b.N; i++ {
		resp, err := client.Get("http://localhost:20002/test1")
		if err != nil {
			b.Fatal(err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkProxy tests the backend through toxiproxy, use 64k random endpoint
func BenchmarkProxy(b *testing.B) {
	client := http.Client{}
	for i := 0; i < b.N; i++ {
		resp, err := client.Get("http://localhost:20000/test1")
		if err != nil {
			b.Fatal(err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkDirectSmall tests the backend server directly, use "hello world" endpoint
func BenchmarkDirectSmall(b *testing.B) {
	client := http.Client{}
	for i := 0; i < b.N; i++ {
		resp, err := client.Get("http://localhost:20002/test2")
		if err != nil {
			b.Fatal(err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}

// BenchmarkProxySmall tests the backend through toxiproxy, use "hello world" endpoint
func BenchmarkProxySmall(b *testing.B) {
	client := http.Client{}
	for i := 0; i < b.N; i++ {
		resp, err := client.Get("http://localhost:20000/test2")
		if err != nil {
			b.Fatal(err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			b.Fatal(err)
		}
		resp.Body.Close()
	}
}
