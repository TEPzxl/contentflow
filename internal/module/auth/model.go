package auth

import "time"

type RefreshToken struct {
	ID        int64      `gorm:"column:id;primaryKey"`
	UserID    int64      `gorm:"column:user_id;not null;index"`
	TokenHash string     `gorm:"column:token_hash;type:text;not null;uniqueIndex"`
	ExpiresAt time.Time  `gorm:"column:expires_at;not null"`
	RevokedAt *time.Time `gorm:"column:revoked_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
