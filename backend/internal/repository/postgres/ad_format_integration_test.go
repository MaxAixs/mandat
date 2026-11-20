package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/maxaixs/mandat/backend/internal/errs"
	"github.com/maxaixs/mandat/backend/internal/models"
	newsrepo "github.com/maxaixs/mandat/backend/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdFormatRepo(t *testing.T) *newsrepo.AdFormatRepo {
	t.Helper()

	pool := setupPool(t)
	_, err := pool.Exec(context.Background(), "TRUNCATE ad_formats")
	require.NoError(t, err)

	return newsrepo.NewAdFormatRepo(pool)
}

func adFormatSeed(title string) *models.AdFormat {
	return &models.AdFormat{
		ID:          uuid.New(),
		Title:       title,
		Description: "Опис " + title,
	}
}

func adFormatPositions(formats []*models.AdFormat) []int {
	positions := make([]int, 0, len(formats))
	for _, f := range formats {
		positions = append(positions, f.Position)
	}

	return positions
}

func TestAdFormatRepo_CreateAppendsPositions(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	for i, title := range []string{"Перший", "Другий", "Третій"} {
		format := adFormatSeed(title)
		require.NoError(t, repo.Create(ctx, format))
		assert.Equal(t, i+1, format.Position, "create must append position %d", i+1)
	}
}

func TestAdFormatRepo_ListOrderedByPosition(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	for _, title := range []string{"Перший", "Другий", "Третій"} {
		require.NoError(t, repo.Create(ctx, adFormatSeed(title)))
	}

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, []int{1, 2, 3}, adFormatPositions(items))
	assert.Equal(t, "Перший", items[0].Title)
	assert.Equal(t, "Другий", items[1].Title)
	assert.Equal(t, "Третій", items[2].Title)
}

func TestAdFormatRepo_GetByIDNotFound(t *testing.T) {
	repo := setupAdFormatRepo(t)

	_, err := repo.GetByID(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}

func TestAdFormatRepo_UpdatePersistsFields(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	format := adFormatSeed("Початкова")
	require.NoError(t, repo.Create(ctx, format))
	before, err := repo.GetByID(ctx, format.ID)
	require.NoError(t, err)

	format.Title = "Оновлена"
	format.Description = "Новий опис"
	format.ImageURL = "https://cdn.example.com/ad-formats/img.png"
	require.NoError(t, repo.Update(ctx, format))

	got, err := repo.GetByID(ctx, format.ID)
	require.NoError(t, err)
	assert.Equal(t, "Оновлена", got.Title)
	assert.Equal(t, "Новий опис", got.Description)
	assert.Equal(t, "https://cdn.example.com/ad-formats/img.png", got.ImageURL)
	assert.Equal(t, before.Position, got.Position, "update must not move the card")
	assert.True(t, got.UpdatedAt.After(before.UpdatedAt) || got.UpdatedAt.Equal(before.UpdatedAt),
		"update must bump updated_at")
	assert.Equal(t, before.CreatedAt, got.CreatedAt, "update must keep created_at")
}

func TestAdFormatRepo_UpdateNotFound(t *testing.T) {
	repo := setupAdFormatRepo(t)

	err := repo.Update(context.Background(), adFormatSeed("Привид"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}

func TestAdFormatRepo_Delete(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	format := adFormatSeed("До видалення")
	require.NoError(t, repo.Create(ctx, format))
	require.NoError(t, repo.Delete(ctx, format.ID))

	_, err := repo.GetByID(ctx, format.ID)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound))
}

func TestAdFormatRepo_DeleteNotFound(t *testing.T) {
	repo := setupAdFormatRepo(t)

	err := repo.Delete(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}

func TestAdFormatRepo_MoveUpSwapsWithPrevious(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	var ids []uuid.UUID
	for _, title := range []string{"Перший", "Другий", "Третій"} {
		format := adFormatSeed(title)
		require.NoError(t, repo.Create(ctx, format))
		ids = append(ids, format.ID)
	}

	require.NoError(t, repo.Move(ctx, ids[1], true))

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, ids[1], items[0].ID)
	assert.Equal(t, ids[0], items[1].ID)
	assert.Equal(t, ids[2], items[2].ID)
	assert.Equal(t, []int{1, 2, 3}, adFormatPositions(items))
}

func TestAdFormatRepo_MoveDownSwapsWithNext(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	var ids []uuid.UUID
	for _, title := range []string{"Перший", "Другий", "Третій"} {
		format := adFormatSeed(title)
		require.NoError(t, repo.Create(ctx, format))
		ids = append(ids, format.ID)
	}

	require.NoError(t, repo.Move(ctx, ids[1], false))

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, ids[0], items[0].ID)
	assert.Equal(t, ids[2], items[1].ID)
	assert.Equal(t, ids[1], items[2].ID)
	assert.Equal(t, []int{1, 2, 3}, adFormatPositions(items))
}

func TestAdFormatRepo_MoveEdgesAreNoOps(t *testing.T) {
	tests := []struct {
		name  string
		index int
		up    bool
	}{
		{"first up", 0, true},
		{"last down", 2, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := setupAdFormatRepo(t)
			ctx := context.Background()

			var ids []uuid.UUID
			for _, title := range []string{"Перший", "Другий", "Третій"} {
				format := adFormatSeed(title)
				require.NoError(t, repo.Create(ctx, format))
				ids = append(ids, format.ID)
			}

			require.NoError(t, repo.Move(ctx, ids[tt.index], tt.up))

			items, err := repo.List(ctx)
			require.NoError(t, err)
			require.Len(t, items, 3)
			assert.Equal(t, ids, []uuid.UUID{items[0].ID, items[1].ID, items[2].ID},
				"moving past the edge must leave the order unchanged")
		})
	}
}

func TestAdFormatRepo_MoveUnknownNotFound(t *testing.T) {
	repo := setupAdFormatRepo(t)

	err := repo.Move(context.Background(), uuid.New(), true)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrAdFormatNotFound), "expected ErrAdFormatNotFound, got %v", err)
}

func TestAdFormatRepo_DeleteThenCreateGetsMaxPlusOne(t *testing.T) {
	repo := setupAdFormatRepo(t)
	ctx := context.Background()

	var ids []uuid.UUID
	for _, title := range []string{"Перший", "Другий", "Третій"} {
		format := adFormatSeed(title)
		require.NoError(t, repo.Create(ctx, format))
		ids = append(ids, format.ID)
	}
	require.NoError(t, repo.Delete(ctx, ids[2]))

	next := adFormatSeed("Четвертий")
	require.NoError(t, repo.Create(ctx, next))
	assert.Equal(t, 3, next.Position, "create after delete must reuse max+1")

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, []int{1, 2, 3}, adFormatPositions(items))
}
