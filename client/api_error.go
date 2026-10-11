// Package Toxiproxy provides a client wrapper around the Toxiproxy HTTP API for
// testing the resiliency of Go applications.
//
// For use with Toxiproxy 2.x

package toxiproxy

import (
	"fmt"
)

type ApiError struct {
	Message string `json:"error"`
	Status  int    `json:"status"`
}

func (err *ApiError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", err.Status, err.Message)
}
