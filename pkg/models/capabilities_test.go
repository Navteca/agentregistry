package models

import (
	"encoding/json"
	"testing"

	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

func TestCapabilitiesMetaJSON(t *testing.T) {
	data, err := json.Marshal(struct {
		Meta ServerResponseMeta `json:"_meta"`
	}{
		Meta: ServerResponseMeta{
			Capabilities: &CapabilitiesMeta{
				CanUpdate: true,
				CanDelete: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal capabilities: %v", err)
	}

	got := string(data)
	want := `{"_meta":{"aregistry.ai/capabilities":{"can_update":true,"can_delete":true}}}`
	if got != want {
		t.Fatalf("unexpected capabilities JSON: got %s, want %s", got, want)
	}
}

func TestResponseWithoutCapabilitiesDoesNotAdvertiseActions(t *testing.T) {
	data, err := json.Marshal(ServerResponse{
		Server: apiv0.ServerJSON{},
		Meta:   ServerResponseMeta{},
	})
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	meta, ok := response["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("response metadata has unexpected shape: %#v", response["_meta"])
	}
	if _, ok := meta["aregistry.ai/capabilities"]; ok {
		t.Fatalf("response unexpectedly advertised capabilities: %#v", meta)
	}
}
