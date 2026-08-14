package database

import (
	"database/sql"

	"github.com/agentregistry-dev/agentregistry/pkg/models"
)

func ownershipMetaFromColumns(subject, displayName, authMethod sql.NullString) *models.OwnershipMeta {
	if !subject.Valid {
		return nil
	}

	return &models.OwnershipMeta{
		Subject:     subject.String,
		DisplayName: displayName.String,
		AuthMethod:  authMethod.String,
	}
}

func ownershipMetaFromInput(input models.OwnershipInput) *models.OwnershipMeta {
	if input.Subject == "" {
		return nil
	}

	return &models.OwnershipMeta{
		Subject:     input.Subject,
		DisplayName: input.DisplayName,
		AuthMethod:  input.AuthMethod,
	}
}
