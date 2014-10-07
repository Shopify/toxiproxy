package main

import (
	"io/ioutil"
	"net/http"
	"testing"
)

// Benchmark numbers:
//
// 1x NoopToxic:
//     BenchmarkDirect             5000            686886 ns/op
//     BenchmarkProxy              1000           2520665 ns/op
//     BenchmarkDirectSmall        5000            442163 ns/op
//     BenchmarkProxySmall         2000            836634 ns/op
//
// 5x NoopToxic:
//     BenchmarkDirect             5000            698202 ns/op
//     BenchmarkProxy               500           3058915 ns/op
//     BenchmarkDirectSmall        5000            454142 ns/op
//     BenchmarkProxySmall         2000            816412 ns/op

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
