package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
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

func (server *server) Listen(host string, port string) {
	r := mux.NewRouter()
	r.HandleFunc("/reset", server.ResetState).Methods("GET")
	r.HandleFunc("/proxies", server.ProxyIndex).Methods("GET")
	r.HandleFunc("/proxies", server.ProxyCreate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}", server.ProxyShow).Methods("GET")
	r.HandleFunc("/proxies/{proxy}", server.ProxyUpdate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}", server.ProxyDelete).Methods("DELETE")
	r.HandleFunc("/proxies/{proxy}/{stream}/toxics", server.ToxicIndex).Methods("GET")
	r.HandleFunc("/proxies/{proxy}/{stream}/toxics", server.ToxicCreate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}/{stream}/toxics/{toxic}", server.ToxicShow).Methods("GET")
	r.HandleFunc("/proxies/{proxy}/{stream}/toxics/{toxic}", server.ToxicUpdate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}/{stream}/toxics/{toxic}", server.ToxicDelete).Methods("DELETE")

	r.HandleFunc("/version", server.Version).Methods("GET")
	http.Handle("/", r)

	logrus.WithFields(logrus.Fields{
		"host":    host,
		"port":    port,
		"version": Version,
	}).Info("API HTTP server starting")

	err := http.ListenAndServe(net.JoinHostPort(host, port), nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func (server *server) ProxyIndex(response http.ResponseWriter, request *http.Request) {
	proxies := server.collection.Proxies()
	marshalData := make(map[string]interface{}, len(proxies))

	for name, proxy := range proxies {
		marshalData[name] = proxyWithToxics(proxy)
	}

	data, err := json.Marshal(marshalData)
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ProxyIndex: Failed to write response to client", err)
	}
}

func (server *server) ResetState(response http.ResponseWriter, request *http.Request) {
	proxies := server.collection.Proxies()

	for _, proxy := range proxies {
		err := proxy.Start()
		if err != ErrProxyAlreadyStarted && apiError(response, err) {
			return
		}

		proxy.upToxics.ResetToxics()
		proxy.downToxics.ResetToxics()
	}

	response.WriteHeader(http.StatusNoContent)
	_, err := response.Write(nil)
	if err != nil {
		logrus.Warn("ResetState: Failed to write headers to client", err)
	}
}

func (server *server) ProxyCreate(response http.ResponseWriter, request *http.Request) {
	// Default fields to enable the proxy right away
	input := Proxy{Enabled: true}
	err := json.NewDecoder(request.Body).Decode(&input)
	if apiError(response, joinError(err, ErrBadRequestBody)) {
		return
	}

	if len(input.Name) < 1 {
		apiError(response, joinError(fmt.Errorf("name"), ErrMissingField))
		return
	}
	if len(input.Upstream) < 1 {
		apiError(response, joinError(fmt.Errorf("upstream"), ErrMissingField))
		return
	}

	proxy := NewProxy()
	proxy.Name = input.Name
	proxy.Listen = input.Listen
	proxy.Upstream = input.Upstream

	err = server.collection.Add(proxy, input.Enabled)
	if apiError(response, err) {
		return
	}

	data, err := json.Marshal(proxyWithToxics(proxy))
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ProxyCreate: Failed to write response to client", err)
	}
}

func (server *server) ProxyUpdate(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	// Default fields are the same as existing proxy
	input := Proxy{Listen: proxy.Listen, Upstream: proxy.Upstream, Enabled: proxy.Enabled}
	err = json.NewDecoder(request.Body).Decode(&input)
	if apiError(response, joinError(err, ErrBadRequestBody)) {
		return
	}

	err = proxy.Update(&input)
	if apiError(response, err) {
		return
	}

	data, err := json.Marshal(proxyWithToxics(proxy))
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ProxyUpdate: Failed to write response to client", err)
	}
}

func (server *server) ProxyDelete(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	err := server.collection.Remove(vars["proxy"])
	if apiError(response, err) {
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		logrus.Warn("ProxyDelete: Failed to write headers to client", err)
	}
}

func (server *server) ProxyShow(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	data, err := json.Marshal(proxyWithToxics(proxy))
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ProxyShow: Failed to write response to client", err)
	}
}

func (server *server) ToxicIndex(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	var data []byte
	switch vars["stream"] {
	case "upstream":
		data, err = json.Marshal(proxy.upToxics.GetToxicMap())
	case "downstream":
		data, err = json.Marshal(proxy.downToxics.GetToxicMap())
	default:
		apiError(response, ErrInvalidStream)
		return
	}
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ToxicIndex: Failed to write response to client", err)
	}
}

func (server *server) ToxicCreate(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	var toxic Toxic
	switch vars["stream"] {
	case "upstream":
		toxic, err = proxy.upToxics.AddToxicJson(request.Body)
	case "downstream":
		toxic, err = proxy.downToxics.AddToxicJson(request.Body)
	default:
		apiError(response, ErrInvalidStream)
		return
	}
	if apiError(response, err) {
		return
	}

	data, err := json.Marshal(toxic)
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ToxicCreate: Failed to write response to client", err)
	}
}

func (server *server) ToxicShow(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	var toxic Toxic
	switch vars["stream"] {
	case "upstream":
		toxic = proxy.upToxics.GetToxic(vars["toxic"])
	case "downstream":
		toxic = proxy.downToxics.GetToxic(vars["toxic"])
	default:
		apiError(response, ErrInvalidStream)
		return
	}
	if toxic == nil {
		apiError(response, ErrToxicNotFound)
		return
	}

	data, err := json.Marshal(toxic)
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ToxicShow: Failed to write response to client", err)
	}
}

func (server *server) ToxicUpdate(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	var toxic Toxic
	switch vars["stream"] {
	case "upstream":
		toxic, err = proxy.upToxics.UpdateToxicJson(vars["toxic"], request.Body)
	case "downstream":
		toxic, err = proxy.downToxics.UpdateToxicJson(vars["toxic"], request.Body)
	default:
		apiError(response, ErrInvalidStream)
		return
	}
	if apiError(response, err) {
		return
	}

	data, err := json.Marshal(toxic)
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ToxicUpdate: Failed to write response to client", err)
	}
}

func (server *server) ToxicDelete(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	switch vars["stream"] {
	case "upstream":
		err = proxy.upToxics.RemoveToxic(vars["toxic"])
	case "downstream":
		err = proxy.downToxics.RemoveToxic(vars["toxic"])
	default:
		apiError(response, ErrInvalidStream)
		return
	}
	if apiError(response, err) {
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		logrus.Warn("ToxicDelete: Failed to write headers to client", err)
	}
}

func (server *server) Version(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "text/plain")
	_, err := response.Write([]byte(Version))
	if err != nil {
		logrus.Warn("Version: Failed to write response to client", err)
	}
}

type ApiError struct {
	Message    string `json:"title"`
	StatusCode int    `json:"status"`
}

func (e *ApiError) Error() string {
	return e.Message
}

func newError(msg string, status int) *ApiError {
	return &ApiError{msg, status}
}

func joinError(err error, wrapper *ApiError) *ApiError {
	if err != nil {
		return &ApiError{wrapper.Message + ": " + err.Error(), wrapper.StatusCode}
	}
	return nil
}

var (
	ErrBadRequestBody     = newError("bad request body", http.StatusBadRequest)
	ErrMissingField       = newError("missing required field", http.StatusBadRequest)
	ErrProxyNotFound      = newError("proxy not found", http.StatusNotFound)
	ErrProxyAlreadyExists = newError("proxy already exists", http.StatusConflict)
	ErrInvalidStream      = newError("stream not found", http.StatusNotFound)
	ErrInvalidToxicType   = newError("invalid toxic type", http.StatusBadRequest)
	ErrToxicAlreadyExists = newError("toxic already exists", http.StatusConflict)
	ErrToxicNotFound      = newError("toxic not found", http.StatusNotFound)
)

func apiError(resp http.ResponseWriter, err error) bool {
	obj, ok := err.(*ApiError)
	if !ok && err != nil {
		logrus.Warn("Error did not include status code:", err)
		obj = &ApiError{err.Error(), http.StatusInternalServerError}
	}

	if obj == nil {
		return false
	}

	data, err2 := json.Marshal(obj)
	if err2 != nil {
		logrus.Warn("Error json encoding error (╯°□°）╯︵ ┻━┻", err2)
	}
	resp.Header().Set("Content-Type", "application/json")
	http.Error(resp, string(data), obj.StatusCode)

	return true
}

func proxyWithToxics(proxy *Proxy) (result struct {
	*Proxy
	UpstreamToxics   map[string]Toxic `json:"upstream_toxics"`
	DownstreamToxics map[string]Toxic `json:"downstream_toxics"`
}) {
	result.Proxy = proxy
	result.UpstreamToxics = proxy.upToxics.GetToxicMap()
	result.DownstreamToxics = proxy.downToxics.GetToxicMap()
	return
}
