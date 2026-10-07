package toxiproxy

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Shopify/toxiproxy/v2/stream"
	"github.com/Shopify/toxiproxy/v2/toxics"
)

type hostsToxic struct {
	Hosts []string          `json:"hosts"`
	Tags  map[string]string `json:"tags"`
}

func (t *hostsToxic) Pipe(stub *toxics.ToxicStub) {
	new(toxics.NoopToxic).Pipe(stub)
}

func (t *hostsToxic) Validate() error {
	if len(t.Hosts) > 2 {
		return errors.New("at most 2 hosts are allowed")
	}
	return nil
}

func TestUpdateToxicJsonDoesNotShareStateWithLiveToxic(t *testing.T) {
	collection := NewToxicCollection(nil)
	original := &hostsToxic{
		Hosts: []string{"a", "b"},
		Tags:  map[string]string{"x": "1"},
	}
	collection.chainAddToxic(&toxics.ToxicWrapper{
		Toxic:     original,
		Name:      "hosts",
		Type:      "hosts",
		Direction: stream.Downstream,
		Toxicity:  1,
	})

	_, err := collection.UpdateToxicJson("hosts", strings.NewReader(
		`{"attributes":{"hosts":["evil","b","c"],"tags":{"y":"2"}}}`,
	))
	var apiErr *ApiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected 400 API error, got %v", err)
	}

	expectedHosts := []string{"a", "b"}
	expectedTags := map[string]string{"x": "1"}
	if !reflect.DeepEqual(original.Hosts, expectedHosts) ||
		!reflect.DeepEqual(original.Tags, expectedTags) {
		t.Fatalf("Rejected update modified live toxic: %+v", original)
	}

	_, err = collection.UpdateToxicJson("hosts", strings.NewReader(
		`{"attributes":{"hosts":["c"]},"toxicity":1}`,
	))
	if err != nil {
		t.Fatal("Expected valid update to succeed:", err)
	}

	if !reflect.DeepEqual(original.Hosts, expectedHosts) {
		t.Fatalf("Accepted update modified previous toxic: %+v", original)
	}
	updated := collection.GetToxic("hosts").Toxic.(*hostsToxic)
	if !reflect.DeepEqual(updated.Hosts, []string{"c"}) ||
		!reflect.DeepEqual(updated.Tags, expectedTags) {
		t.Fatalf("Unexpected attributes after update: %+v", updated)
	}
}
