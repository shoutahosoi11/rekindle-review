package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shout/ai-study-tool/backend/internal/domain"
)

type HighlightUsecase struct {
	repo        domain.HighlightImportRepository
	importQueue domain.HighlightImportQueueRepository
	jobTrigger  domain.HighlightImportJobTrigger
}

func NewHighlightUsecase(repo domain.HighlightImportRepository) *HighlightUsecase {
	return &HighlightUsecase{repo: repo}
}

func NewHighlightUsecaseWithQueue(
	repo domain.HighlightImportRepository,
	importQueue domain.HighlightImportQueueRepository,
	jobTrigger domain.HighlightImportJobTrigger,
) *HighlightUsecase {
	return &HighlightUsecase{
		repo:        repo,
		importQueue: importQueue,
		jobTrigger:  jobTrigger,
	}
}

type ImportHighlightItem struct {
	ASIN          string
	BookTitle     string
	BookAuthor    string
	Content       string
	Location      string
	HighlightedAt *time.Time
}

type ImportSharedHighlightInput struct {
	BookTitle  string
	BookAuthor string
	Content    string
	SourceApp  string
	SourceURL  string
	SharedAt   *time.Time
}

type ImportPastedHighlightInput struct {
	BookTitle  string
	BookAuthor string
	Content    string
	SourceApp  string
	SourceURL  string
}

type ImportKindleResult struct {
	// キューモード: QueueID と QueuedCount が設定される
	QueueID     uuid.UUID
	QueuedCount int
	// 同期モード: Saved 以下が設定される
	Saved          int
	DuplicateCount int
	// 共通
	CopyProtectedCount int
	ResolvedASIN       string
	Highlights         []*domain.Highlight
	Warning            *string
	Queued             bool
}

type ImportSharedHighlightResult struct {
	Saved     bool
	Duplicate bool
	Highlight *domain.Highlight
}

type ImportPastedHighlightResult struct {
	ID        uuid.UUID
	Duplicate bool
	Highlight *domain.Highlight
}

func (u *HighlightUsecase) ImportKindleHighlights(ctx context.Context, userID uuid.UUID, items []ImportHighlightItem) (*ImportKindleResult, error) {
	if len(items) == 0 {
		return nil, domain.ErrAllCopyProtected
	}

	// キューが設定されている場合は非同期処理に委譲する
	if u.importQueue != nil {
		return u.enqueueKindleImport(ctx, userID, items)
	}

	// キューなし（後方互換: NewHighlightUsecase 経由）
	return u.importKindleHighlightsDirect(ctx, userID, items)
}

func (u *HighlightUsecase) enqueueKindleImport(ctx context.Context, userID uuid.UUID, items []ImportHighlightItem) (*ImportKindleResult, error) {
	// 既存の検証・正規化ロジックを使いコピープロテクトを除外する
	highlights, copyProtectedCount, err := buildImportHighlights(userID, items)
	if err != nil {
		return nil, err
	}
	if len(highlights) == 0 {
		return nil, domain.ErrAllCopyProtected
	}

	// 検証済み highlights を JSON にシリアライズしてキューに積む
	payload, err := json.Marshal(highlights)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: marshal kindle items: %w", err)
	}

	queueID, err := u.importQueue.Enqueue(ctx, userID, domain.ImportQueueSourceKindle, payload)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: enqueue kindle import: %w", err)
	}

	if u.jobTrigger != nil {
		if triggerErr := u.jobTrigger.TriggerHighlightImportJob(ctx); triggerErr != nil {
			log.Printf("highlight import job trigger failed (queue_id=%s): %v", queueID, triggerErr)
		}
	}

	result := &ImportKindleResult{
		QueueID:            queueID,
		QueuedCount:        len(highlights),
		CopyProtectedCount: copyProtectedCount,
	}
	if copyProtectedCount > 0 {
		warning := "コピー制限により一部のハイライトが読み込めませんでした"
		result.Warning = &warning
	}
	return result, nil
}

func (u *HighlightUsecase) importKindleHighlightsDirect(ctx context.Context, userID uuid.UUID, items []ImportHighlightItem) (*ImportKindleResult, error) {
	highlights, copyProtectedCount, err := buildImportHighlights(userID, items)
	if err != nil {
		return nil, err
	}
	if len(highlights) == 0 {
		return nil, domain.ErrAllCopyProtected
	}

	saved, err := u.repo.BulkUpsert(ctx, highlights)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: import kindle highlights: %w", err)
	}
	if saved > len(highlights) {
		return nil, fmt.Errorf("highlight usecase: import kindle highlights: invalid saved count %d", saved)
	}

	result := &ImportKindleResult{
		Saved:              saved,
		DuplicateCount:     len(highlights) - saved,
		CopyProtectedCount: copyProtectedCount,
		ResolvedASIN:       resolveImportASIN(highlights),
		Highlights:         collectPersistedHighlights(highlights),
	}
	if copyProtectedCount > 0 {
		warning := "コピー制限により一部のハイライトが読み込めませんでした"
		result.Warning = &warning
	}
	return result, nil
}

func (u *HighlightUsecase) ImportPastedHighlight(ctx context.Context, userID uuid.UUID, input ImportPastedHighlightInput) (*ImportPastedHighlightResult, error) {
	highlight, err := newPastedHighlight(userID, input)
	if err != nil {
		return nil, err
	}

	saved, err := u.repo.BulkUpsert(ctx, []*domain.Highlight{highlight})
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: import pasted highlight: %w", err)
	}
	if saved < 0 || saved > 1 {
		return nil, fmt.Errorf("highlight usecase: import pasted highlight: invalid saved count %d", saved)
	}
	if saved == 1 {
		return &ImportPastedHighlightResult{
			ID:        highlight.ID,
			Duplicate: false,
			Highlight: highlight,
		}, nil
	}

	existing, err := u.repo.FindByUserIDAndContentHash(ctx, userID, *highlight.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: find pasted duplicate: %w", err)
	}

	return &ImportPastedHighlightResult{
		ID:        existing.ID,
		Duplicate: true,
		Highlight: existing,
	}, nil
}

func (u *HighlightUsecase) ImportSharedHighlight(ctx context.Context, userID uuid.UUID, input ImportSharedHighlightInput) (*ImportSharedHighlightResult, error) {
	highlight, err := newSharedHighlight(userID, input)
	if err != nil {
		return nil, err
	}

	saved, err := u.repo.BulkUpsert(ctx, []*domain.Highlight{highlight})
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: import shared highlight: %w", err)
	}
	if saved < 0 || saved > 1 {
		return nil, fmt.Errorf("highlight usecase: import shared highlight: invalid saved count %d", saved)
	}

	result := &ImportSharedHighlightResult{
		Saved:     saved == 1,
		Duplicate: saved == 0,
	}
	if result.Saved {
		result.Highlight = highlight
	}

	return result, nil
}

func (u *HighlightUsecase) ListExistingContentHashes(ctx context.Context, userID uuid.UUID, hashes []string) ([]string, error) {
	if len(hashes) == 0 {
		return make([]string, 0), nil
	}

	normalized := normalizeHashList(hashes)
	if len(normalized) == 0 {
		return make([]string, 0), nil
	}

	existing, err := u.repo.ListExistingContentHashesByUserID(ctx, userID, normalized)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: list existing content hashes: %w", err)
	}
	if existing == nil {
		return make([]string, 0), nil
	}

	return existing, nil
}

func (u *HighlightUsecase) ListKindleBooks(ctx context.Context, userID uuid.UUID) ([]*domain.KindleBook, error) {
	books, err := u.repo.ListBooksWithHighlightsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: list kindle books: %w", err)
	}
	if books == nil {
		return make([]*domain.KindleBook, 0), nil
	}

	return books, nil
}

func (u *HighlightUsecase) ListByASIN(ctx context.Context, userID uuid.UUID, asin string) ([]*domain.Highlight, error) {
	highlights, err := u.repo.ListByUserIDAndASIN(ctx, userID, asin)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: list by asin: %w", err)
	}
	if highlights == nil {
		return make([]*domain.Highlight, 0), nil
	}

	return highlights, nil
}

func (u *HighlightUsecase) ListByBookMetadata(ctx context.Context, userID uuid.UUID, bookTitle, bookAuthor string) ([]*domain.Highlight, error) {
	highlights, err := u.repo.ListByUserIDAndBookMetadata(ctx, userID, bookTitle, bookAuthor)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: list by book metadata: %w", err)
	}
	if highlights == nil {
		return make([]*domain.Highlight, 0), nil
	}

	return highlights, nil
}

func (u *HighlightUsecase) UpdateExplanation(ctx context.Context, id, userID uuid.UUID, explanation string) (*domain.Highlight, error) {
	normalizedExplanation := optionalString(explanation)

	highlight, err := u.repo.UpdateExplanation(ctx, id, userID, normalizedExplanation)
	if err != nil {
		return nil, fmt.Errorf("highlight usecase: update explanation: %w", err)
	}

	return highlight, nil
}

func buildImportHighlights(userID uuid.UUID, items []ImportHighlightItem) ([]*domain.Highlight, int, error) {
	highlights := make([]*domain.Highlight, 0, len(items))
	copyProtectedCount := 0

	for _, item := range items {
		highlight, ok, err := newImportHighlight(userID, item)
		if err != nil {
			return nil, copyProtectedCount, err
		}
		if !ok {
			copyProtectedCount++
			continue
		}

		highlights = append(highlights, highlight)
	}

	return highlights, copyProtectedCount, nil
}

func newImportHighlight(userID uuid.UUID, item ImportHighlightItem) (*domain.Highlight, bool, error) {
	content := strings.TrimSpace(item.Content)
	if content == "" {
		return nil, false, nil
	}

	normalizedContent, err := normalizeAndValidateHighlightContent(content)
	if err != nil {
		return nil, false, err
	}

	bookTitle, err := normalizeAndValidateOptionalMetaText(item.BookTitle, 200)
	if err != nil {
		return nil, false, err
	}
	bookAuthor, err := normalizeAndValidateOptionalMetaText(item.BookAuthor, 100)
	if err != nil {
		return nil, false, err
	}

	asin := strings.TrimSpace(item.ASIN)
	location := strings.TrimSpace(item.Location)
	contentHash := computeContentHash(normalizedContent)
	highlightedAt := sanitizeHighlightedAt(item.HighlightedAt)

	return &domain.Highlight{
		UserID:        userID,
		BookTitle:     bookTitle,
		BookAuthor:    bookAuthor,
		ASIN:          optionalString(asin),
		Content:       normalizedContent,
		ContentHash:   &contentHash,
		Location:      optionalString(location),
		HighlightedAt: highlightedAt,
		Source:        domain.HighlightSourceExtension,
		Status:        domain.HighlightStatusPending,
		RequestedAt:   time.Now().UTC(),
	}, true, nil
}

func newSharedHighlight(userID uuid.UUID, input ImportSharedHighlightInput) (*domain.Highlight, error) {
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, domain.ErrInvalidInput
	}

	normalizedContent, err := normalizeAndValidateHighlightContent(content)
	if err != nil {
		return nil, err
	}

	sourceApp := strings.TrimSpace(input.SourceApp)
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL != "" {
		if err := domain.ValidateSourceURL(sourceURL); err != nil {
			return nil, err
		}
	}

	bookTitle, err := normalizeAndValidateOptionalMetaText(input.BookTitle, 200)
	if err != nil {
		return nil, err
	}
	bookAuthor, err := normalizeAndValidateOptionalMetaText(input.BookAuthor, 100)
	if err != nil {
		return nil, err
	}
	contentHash := computeContentHash(normalizedContent)

	return &domain.Highlight{
		UserID:        userID,
		BookTitle:     bookTitle,
		BookAuthor:    bookAuthor,
		Content:       normalizedContent,
		ContentHash:   &contentHash,
		HighlightedAt: sanitizeHighlightedAt(input.SharedAt),
		Source:        domain.HighlightSourceShare,
		SourceApp:     optionalString(sourceApp),
		SourceURL:     optionalString(sourceURL),
		Status:        domain.HighlightStatusPending,
		RequestedAt:   time.Now().UTC(),
	}, nil
}

func newPastedHighlight(userID uuid.UUID, input ImportPastedHighlightInput) (*domain.Highlight, error) {
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, domain.ErrInvalidInput
	}

	normalizedContent, err := normalizeAndValidateHighlightContent(content)
	if err != nil {
		return nil, err
	}

	sourceApp := strings.TrimSpace(input.SourceApp)
	if sourceApp != "" && !isAllowedPasteSourceApp(sourceApp) {
		return nil, domain.ErrInvalidInput
	}
	sourceApp = strings.ToLower(sourceApp)

	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL != "" {
		if err := domain.ValidateSourceURL(sourceURL); err != nil {
			return nil, err
		}
	}

	bookTitle, err := normalizeAndValidateOptionalMetaText(input.BookTitle, 200)
	if err != nil {
		return nil, err
	}
	bookAuthor, err := normalizeAndValidateOptionalMetaText(input.BookAuthor, 100)
	if err != nil {
		return nil, err
	}
	contentHash := computeContentHash(normalizedContent)

	return &domain.Highlight{
		UserID:      userID,
		BookTitle:   bookTitle,
		BookAuthor:  bookAuthor,
		Content:     normalizedContent,
		ContentHash: &contentHash,
		Source:      domain.HighlightSourcePaste,
		SourceApp:   optionalString(sourceApp),
		SourceURL:   optionalString(sourceURL),
		Status:      domain.HighlightStatusPending,
		RequestedAt: time.Now().UTC(),
	}, nil
}

func normalizeAndValidateHighlightContent(content string) (string, error) {
	if err := validateHighlightContent(content); err != nil {
		return "", err
	}

	normalized := domain.NormalizeText(content)
	if err := validateHighlightContent(normalized); err != nil {
		return "", err
	}

	return normalized, nil
}

func validateHighlightContent(content string) error {
	if err := domain.ValidateTextLength(content, 300); err != nil {
		return err
	}
	if err := domain.ValidateLineCount(content, 20); err != nil {
		return err
	}
	if err := domain.ValidateMaxLineLength(content, 300); err != nil {
		return err
	}

	return nil
}

func normalizeAndValidateOptionalMetaText(value string, max int) (*string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	if err := domain.ValidateTextLength(trimmed, max); err != nil {
		return nil, err
	}

	normalized := domain.NormalizeMetaText(trimmed)
	if err := domain.ValidateTextLength(normalized, max); err != nil {
		return nil, err
	}

	return &normalized, nil
}

func isAllowedPasteSourceApp(sourceApp string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceApp)) {
	case "kindle", "x", "web":
		return true
	default:
		return false
	}
}

func computeContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func normalizeHashList(hashes []string) []string {
	seen := make(map[string]struct{}, len(hashes))
	items := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		normalized := strings.TrimSpace(hash)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		items = append(items, normalized)
	}
	return items
}

func sanitizeHighlightedAt(highlightedAt *time.Time) *time.Time {
	if highlightedAt == nil {
		return nil
	}
	if highlightedAt.After(time.Now()) {
		return nil
	}

	sanitized := *highlightedAt
	return &sanitized
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func collectPersistedHighlights(highlights []*domain.Highlight) []*domain.Highlight {
	persisted := make([]*domain.Highlight, 0, len(highlights))
	for _, highlight := range highlights {
		if highlight == nil || highlight.ID == uuid.Nil {
			continue
		}

		persisted = append(persisted, highlight)
	}

	return persisted
}

func resolveImportASIN(highlights []*domain.Highlight) string {
	for _, highlight := range highlights {
		if highlight == nil || highlight.ASIN == nil {
			continue
		}

		asin := strings.TrimSpace(*highlight.ASIN)
		if asin != "" {
			return asin
		}
	}

	return ""
}
