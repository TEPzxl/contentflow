package collector

type CollectSourceRequest struct {
	UserID   int64
	SourceID int64
}

type CollectSourceResponse struct {
	RunID           int64
	SourceID        int64
	Status          string
	FetchedCount    int
	InsertedCount   int
	DuplicatedCount int
	ErrorMessage    string
}

type RequestCollectionResponse struct {
	TaskID   string
	SourceID int64
	Status   string
}

type ListCollectionRunsRequest struct {
	UserID   int64
	SourceID int64
	Status   string
	Limit    int
	Offset   int
}

type ListCollectionRunsResponse struct {
	Runs   []CollectionRunDTO
	Total  int64
	Limit  int
	Offset int
}

type GetCollectionRunRequest struct {
	UserID int64
	RunID  int64
}

type GetCollectionRunResponse struct {
	Run CollectionRunDTO
}

type CollectionRunDTO struct {
	ID              int64
	SourceID        int64
	Status          string
	StartedAt       string
	FinishedAt      *string
	FetchedCount    int
	InsertedCount   int
	DuplicatedCount int
	ErrorMessage    string
}

type ArticleWriteResult struct {
	InsertedCount   int
	DuplicatedCount int
}
