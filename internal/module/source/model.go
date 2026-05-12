package source

import (
	"time"

	"gorm.io/datatypes"
)

type Source struct {
	ID               int64          `gorm:"column:id;primaryKey"`
	UserID           int64          `gorm:"column:user_id;not null;index"`
	Name             string         `gorm:"column:name;type:varchar(200);not null"`
	Type             string         `gorm:"column:type;type:varchar(50);not null;index"`
	URL              *string        `gorm:"column:url;type:text"`
	ConfigJSON       datatypes.JSON `gorm:"column:config_json;type:jsonb;not null"`
	IsActive         bool           `gorm:"column:is_active;not null;default:true"`
	LastFetchedAt    *time.Time     `gorm:"column:last_fetched_at"`
	LastFetchStatus  string         `gorm:"column:last_fetch_status;type:varchar(50);not null;default:''"`
	LastFetchMessage string         `gorm:"column:last_fetch_message;type:text;not null;default:''"`
	CreatedAt        time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;not null"`
	DeletedAt        *time.Time     `gorm:"column:deleted_at;index"`
}

func (Source) TableName() string {
	return "sources"
}
