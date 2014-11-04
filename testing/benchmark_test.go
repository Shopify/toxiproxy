package main

import (
	"io/ioutil"
	"net/http"
	"testing"
)

// Benchmark numbers:
//
// 1x Toxics:
//     BenchmarkDirect             2000            694467 ns/op
//     BenchmarkProxy              2000           1136668 ns/op
//     BenchmarkDirectSmall        5000            423319 ns/op
//     BenchmarkProxySmall         2000            769262 ns/op
//
// 5x Toxics:
//     BenchmarkDirect             5000            695102 ns/op
//     BenchmarkProxy              2000           1232454 ns/op
//     BenchmarkDirectSmall        5000            424712 ns/op
//     BenchmarkProxySmall         2000            798016 ns/op

// Test the backend server directly, use 64k random endpoint
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

// Test the backend through toxiproxy, use 64k random endpoint
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

// Test the backend server directly, use "hello world" endpoint
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

// Test the backend through toxiproxy, use "hello world" endpoint
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
