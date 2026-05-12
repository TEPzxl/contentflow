package user

import "time"

type User struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	Email        string    `gorm:"column:email;type:varchar(255);not null;unique"`
	PasswordHash string    `gorm:"column:password_hash;type:text;not null"`
	DisplayName  string    `gorm:"column:display_name;type:varchar(100);not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (User) TableName() string {
	return "users"
}
