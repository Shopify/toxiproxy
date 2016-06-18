package toxiproxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/Shopify/toxiproxy/toxics"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

type ApiServer struct {
	Collection *ProxyCollection
}

func NewServer() *ApiServer {
	return &ApiServer{
		Collection: NewProxyCollection(),
	}
}

func (server *ApiServer) Listen(host string, port string) {
	r := mux.NewRouter()
	r.HandleFunc("/reset", server.ResetState).Methods("POST")
	r.HandleFunc("/proxies", server.ProxyIndex).Methods("GET")
	r.HandleFunc("/proxies", server.ProxyCreate).Methods("POST")
	r.HandleFunc("/populate", server.Populate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}", server.ProxyShow).Methods("GET")
	r.HandleFunc("/proxies/{proxy}", server.ProxyUpdate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}", server.ProxyDelete).Methods("DELETE")
	r.HandleFunc("/proxies/{proxy}/toxics", server.ToxicIndex).Methods("GET")
	r.HandleFunc("/proxies/{proxy}/toxics", server.ToxicCreate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}/toxics/{toxic}", server.ToxicShow).Methods("GET")
	r.HandleFunc("/proxies/{proxy}/toxics/{toxic}", server.ToxicUpdate).Methods("POST")
	r.HandleFunc("/proxies/{proxy}/toxics/{toxic}", server.ToxicDelete).Methods("DELETE")

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

func (server *ApiServer) ProxyIndex(response http.ResponseWriter, request *http.Request) {
	proxies := server.Collection.Proxies()
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

func (server *ApiServer) ResetState(response http.ResponseWriter, request *http.Request) {
	proxies := server.Collection.Proxies()

	for _, proxy := range proxies {
		err := proxy.Start()
		if err != ErrProxyAlreadyStarted && apiError(response, err) {
			return
		}

		proxy.Toxics.ResetToxics()
	}

	response.WriteHeader(http.StatusNoContent)
	_, err := response.Write(nil)
	if err != nil {
		logrus.Warn("ResetState: Failed to write headers to client", err)
	}
}

func (server *ApiServer) ProxyCreate(response http.ResponseWriter, request *http.Request) {
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

	err = server.Collection.Add(proxy, input.Enabled)
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

func (server *ApiServer) Populate(response http.ResponseWriter, request *http.Request) {
	input := []Proxy{}

	err := json.NewDecoder(request.Body).Decode(&input)
	if apiError(response, joinError(err, ErrBadRequestBody)) {
		return
	}

	proxiesCreated := []*Proxy{}

	for _, p := range input {
		if len(p.Name) < 1 {
			apiError(response, joinError(fmt.Errorf("name"), ErrMissingField))
			return
		}
		if len(p.Upstream) < 1 {
			apiError(response, joinError(fmt.Errorf("upstream"), ErrMissingField))
			return
		}

		// TODO(jpittis) Defaulting to enabled is not standard but I dont see another way.
		p.Enabled = true

		proxy := NewProxy()
		proxy.Name = p.Name
		proxy.Listen = p.Listen
		proxy.Upstream = p.Upstream

		// TODO(jpittis) Tried to make this transactional. Kinda a mess of error handling code.
		err = server.Collection.Add(proxy, p.Enabled)
		if err != nil {
			for _, p := range proxiesCreated {
				err := server.Collection.Remove(p.Name)
				if apiError(response, err) {
					return
				}
			}
			if apiError(response, joinError(err, ErrBadRequestBody)) {
				return
			}
		}

		proxiesCreated = append(proxiesCreated, proxy)
	}

	data, err := json.Marshal(proxiesWithToxics(proxiesCreated))
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

func (server *ApiServer) ProxyShow(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
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

func (server *ApiServer) ProxyUpdate(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
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

func (server *ApiServer) ProxyDelete(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	err := server.Collection.Remove(vars["proxy"])
	if apiError(response, err) {
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		logrus.Warn("ProxyDelete: Failed to write headers to client", err)
	}
}

func (server *ApiServer) ToxicIndex(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	toxics := proxy.Toxics.GetToxicArray()
	data, err := json.Marshal(toxics)
	if apiError(response, err) {
		return
	}

	response.Header().Set("Content-Type", "application/json")
	_, err = response.Write(data)
	if err != nil {
		logrus.Warn("ToxicIndex: Failed to write response to client", err)
	}
}

func (server *ApiServer) ToxicCreate(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	toxic, err := proxy.Toxics.AddToxicJson(request.Body)
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

func (server *ApiServer) ToxicShow(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	toxic := proxy.Toxics.GetToxic(vars["toxic"])
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

func (server *ApiServer) ToxicUpdate(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	toxic, err := proxy.Toxics.UpdateToxicJson(vars["toxic"], request.Body)
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

func (server *ApiServer) ToxicDelete(response http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	proxy, err := server.Collection.Get(vars["proxy"])
	if apiError(response, err) {
		return
	}

	err = proxy.Toxics.RemoveToxic(vars["toxic"])
	if apiError(response, err) {
		return
	}

	response.WriteHeader(http.StatusNoContent)
	_, err = response.Write(nil)
	if err != nil {
		logrus.Warn("ToxicDelete: Failed to write headers to client", err)
	}
}

func (server *ApiServer) Version(response http.ResponseWriter, request *http.Request) {
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
	ErrInvalidStream      = newError("stream was invalid, can be either upstream or downstream", http.StatusBadRequest)
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

type proxyToxics struct {
	*Proxy
	Toxics []toxics.Toxic `json:"toxics"`
}

func proxyWithToxics(proxy *Proxy) (result proxyToxics) {
	result.Proxy = proxy
	result.Toxics = proxy.Toxics.GetToxicArray()
	return
}

func proxiesWithToxics(proxies []*Proxy) []proxyToxics {
	result := make([]proxyToxics, len(proxies))
	for i := range proxies {
		result[i] = proxyWithToxics(proxies[i])
	}
	return result
}
