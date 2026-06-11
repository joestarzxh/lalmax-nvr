package model

import "time"

type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleUser       Role = "user"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	DisplayName  string    `json:"display_name"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (r Role) IsValid() bool {
	return r == RoleSuperAdmin || r == RoleUser
}

func (r Role) CanOperate() bool {
	return r == RoleSuperAdmin
}
