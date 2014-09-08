package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type server struct {
	collection *ProxyCollection
}

func NewServer() *server {
	return &server{
		collection: NewProxyCollection(),
	}
}

func (server *server) Listen() {
	r := mux.NewRouter()
	r.HandleFunc("/proxies", server.ProxyIndex).Methods("GET")
	r.HandleFunc("/proxies", server.ProxyCreate).Methods("POST")
	r.HandleFunc("/proxies/{name}", server.ProxyDelete).Methods("DELETE")
	http.Handle("/", r)

	err := http.ListenAndServe(":8474", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func (server *server) ProxyIndex(response http.ResponseWriter, request *http.Request) {
	data, err := json.Marshal(server.collection.Proxies())
	if err != nil {
		http.Error(response, fmt.Sprint(err), 500)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(200)
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ProxyIndex: Failed to write response to client", err)
	}
}

func (server *server) ProxyCreate(response http.ResponseWriter, request *http.Request) {
	proxy := NewProxy()
	err := json.NewDecoder(request.Body).Decode(&proxy)
	if err != nil {
		http.Error(response, server.apiError(err, 500), 400)
		return
	}

	err = server.collection.Add(proxy)
	if err != nil {
		http.Error(response, server.apiError(err, 409), 409)
		return
	}

	proxy.Start()
	<-proxy.started

	data, err := json.Marshal(&proxy)
	if err != nil {
		http.Error(response, server.apiError(err, 500), 500)
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(201)
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ProxyIndex: Failed to write response to client", err)
	}
}

func (server *server) ProxyDelete(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	err := server.collection.Remove(vars["name"])
	if err != nil {
		http.Error(response, server.apiError(err, 404), 404)
		return
	}

	response.WriteHeader(204)
	_, err = response.Write(nil)
	if err != nil {
		logrus.Warn("ProxyIndex: Failed to write headers to client", err)
	}
}

func (server *server) apiError(err error, code int) string {
	return fmt.Sprintf(`
{
	"title": "%s",
	"status": %d
}
	`, err.Error(), code)
}
