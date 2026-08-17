package models

// CapabilitiesMeta records actions the current caller may perform on an artifact.
type CapabilitiesMeta struct {
	CanUpdate bool `json:"can_update"`
	CanDelete bool `json:"can_delete"`
	CanDeploy bool `json:"can_deploy"`
}
