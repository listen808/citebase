package repository

import (
	adminmodel "github.com/listen808/citebase/internal/model/admin"
)

// OIDC 身份认证相关表。
func init() {
	RegisterModels(
		&adminmodel.OIDCProvider{},
		&adminmodel.OIDCUserMapping{},
	)
}
