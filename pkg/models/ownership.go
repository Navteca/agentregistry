package models

// OwnershipMeta records who registered an artifact.
//
// Subject is the stable identity from the auth token and is what
// authorization compares against. DisplayName is a snapshot taken at
// creation for presentation only and must never be used for access
// decisions. Both are absent for artifacts registered before ownership
// tracking, which clients render as unowned.
type OwnershipMeta struct {
	Subject     string `json:"subject"`
	DisplayName string `json:"displayName,omitempty"`
	AuthMethod  string `json:"authMethod,omitempty"`
}

// OwnershipInput carries the authenticated creator identity into persistence.
type OwnershipInput struct {
	Subject     string
	DisplayName string
	AuthMethod  string
}
