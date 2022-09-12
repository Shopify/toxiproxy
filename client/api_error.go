// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x

package toxiproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type ApiError struct {
	Message string `json:"error"`
	Status  int    `json:"status"`
}

func (err *ApiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", err.Status, err.Message)
}

func checkError(resp *http.Response, expectedCode int, caller string) error {
	if resp.StatusCode != expectedCode {
		apiError := new(ApiError)
		err := json.NewDecoder(resp.Body).Decode(apiError)
		if err != nil {
			apiError.Message = fmt.Sprintf("Unexpected response code, expected %d", expectedCode)
			apiError.Status = resp.StatusCode
		}
		return fmt.Errorf("%s: %v", caller, apiError)
	}
	return nil
}
