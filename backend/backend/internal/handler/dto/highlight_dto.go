package dto

import "time"

type ImportHighlightItem struct {
	ASIN          string     `json:"asin"`
	BookTitle     string     `json:"book_title"`
	BookAuthor    string     `json:"book_author"`
	Content       string     `json:"content"`
	Location      string     `json:"location"`
	HighlightedAt *time.Time `json:"highlighted_at"`
}

type ImportHighlightsRequest struct {
	Highlights []ImportHighlightItem `json:"highlights"`
}

type CheckHighlightHashesRequest struct {
	Hashes []string `json:"hashes"`
}

type CheckHighlightHashesResponse struct {
	ExistingHashes []string `json:"existing_hashes"`
}

type ImportSharedHighlightRequest struct {
	BookTitle  string     `json:"book_title"`
	BookAuthor string     `json:"book_author"`
	Content    string     `json:"content"`
	SourceApp  string     `json:"source_app"`
	SourceURL  string     `json:"source_url"`
	SharedAt   *time.Time `json:"shared_at"`
}

type ImportPastedHighlightRequest struct {
	BookTitle  string `json:"book_title"`
	BookAuthor string `json:"book_author"`
	Content    string `json:"content"`
	SourceApp  string `json:"source_app"`
	SourceURL  string `json:"source_url"`
}

type UpdateHighlightExplanationRequest struct {
	Explanation string `json:"explanation"`
}

type HighlightResponse struct {
	ID             string     `json:"id"`
	BookID         *string    `json:"book_id,omitempty"`
	BookTitle      *string    `json:"book_title,omitempty"`
	BookAuthor     *string    `json:"book_author,omitempty"`
	ASIN           *string    `json:"asin,omitempty"`
	Content        string     `json:"content"`
	Explanation    *string    `json:"explanation,omitempty"`
	Location       *string    `json:"location,omitempty"`
	HighlightedAt  *time.Time `json:"highlighted_at,omitempty"`
	Source         string     `json:"source"`
	SourceApp      *string    `json:"source_app,omitempty"`
	SourceURL      *string    `json:"source_url,omitempty"`
	BookOrderIndex *int       `json:"book_order_index,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type ImportHighlightsResponse struct {
	// キューモード (Queued=true の時に設定される)
	Queued      bool   `json:"queued"`
	QueueID     string `json:"queue_id,omitempty"`
	QueuedCount int    `json:"queued_count,omitempty"`
	// 同期モード (Queued=false の時に設定される)
	SavedCount     int                  `json:"saved_count,omitempty"`
	DuplicateCount int                  `json:"duplicate_count,omitempty"`
	Highlights     []*HighlightResponse `json:"highlights,omitempty"`
	ResolvedASIN   string               `json:"resolved_asin,omitempty"`
	// 共通
	CopyProtectedCount int     `json:"copy_protected_count,omitempty"`
	Warning            *string `json:"warning,omitempty"`
}

type ImportSharedHighlightResponse struct {
	Saved     bool               `json:"saved"`
	Duplicate bool               `json:"duplicate"`
	Highlight *HighlightResponse `json:"highlight,omitempty"`
}

type ImportPastedHighlightResponse struct {
	ID        string `json:"id"`
	Duplicate bool   `json:"duplicated"`
}

type ListBookHighlightsResponse struct {
	Highlights []*HighlightResponse `json:"highlights"`
}

type KindleBookResponse struct {
	ASIN           string `json:"asin"`
	BookTitle      string `json:"book_title"`
	BookAuthor     string `json:"book_author"`
	HighlightCount int    `json:"highlight_count"`
	Source         string `json:"source"`
}

type ListKindleBooksResponse struct {
	Books []*KindleBookResponse `json:"books"`
}
