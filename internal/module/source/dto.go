package source

import (
	"encoding/json"
	"time"
)

type CreateSourceRequest struct {
	UserID int64
	Name   string
	Type   string
	URL    *string
	Config json.RawMessage
}

type CreateSourceResponse struct {
	Source SourceDTO
}

type ListSourcesRequest struct {
	UserID int64
	Type   string
	Limit  int
	Offset int
}

type ListSourcesResponse struct {
	Sources []SourceDTO
	Total   int64
	Limit   int
	Offset  int
}

type GetSourceRequest struct {
	UserID   int64
	SourceID int64
}

type GetSourceResponse struct {
	Source SourceDTO
}

type UpdateSourceRequest struct {
	UserID   int64
	SourceID int64
	Name     *string
	URL      *string
	IsActive *bool
	Config   json.RawMessage
}

type UpdateSourceResponse struct {
	Source SourceDTO
}

type DeleteSourceRequest struct {
	UserID   int64
	SourceID int64
}

type SourceDTO struct {
	ID               int64
	Name             string
	Type             string
	URL              *string
	Config           json.RawMessage
	IsActive         bool
	LastFetchedAt    *time.Time
	LastFetchStatus  string
	LastFetchMessage string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
