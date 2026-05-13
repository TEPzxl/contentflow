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

type ArticleWriteResult struct {
	InsertedCount   int
	DuplicatedCount int
}
