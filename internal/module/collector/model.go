package collector

import "time"

type CollectionRun struct {
	ID              int64      `gorm:"column:id;primaryKey"`
	SourceID        int64      `gorm:"column:source_id;not null;index"`
	Status          string     `gorm:"column:status;type:varchar(50);not null;index"`
	StartedAt       time.Time  `gorm:"column:started_at;not null"`
	FinishedAt      *time.Time `gorm:"column:finished_at"`
	FetchedCount    int        `gorm:"column:fetched_count;not null;default:0"`
	InsertedCount   int        `gorm:"column:inserted_count;not null;default:0"`
	DuplicatedCount int        `gorm:"column:duplicated_count;not null;default:0"`
	ErrorMessage    string     `gorm:"column:error_message;type:text;not null;default:''"`
	CreatedAt       time.Time  `gorm:"column:created_at;not null"`
}

func (CollectionRun) TableName() string {
	return "collection_runs"
}
