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

func setupContactRepo(t *testing.T) *newsrepo.ContactRepo {
	t.Helper()

	pool := setupPool(t)
	_, err := pool.Exec(context.Background(), "TRUNCATE contacts")
	require.NoError(t, err)

	return newsrepo.NewContactRepo(pool)
}

func contactSeed(contactType models.ContactType, value string) *models.Contact {
	return &models.Contact{
		ID:    uuid.New(),
		Type:  contactType,
		Value: value,
	}
}

func contactPositions(contacts []*models.Contact) []int {
	positions := make([]int, 0, len(contacts))
	for _, c := range contacts {
		positions = append(positions, c.Position)
	}

	return positions
}

func contactIDs(contacts []*models.Contact) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(contacts))
	for _, c := range contacts {
		ids = append(ids, c.ID)
	}

	return ids
}

func TestContactRepo_CreateAppendsPositions(t *testing.T) {
	repo := setupContactRepo(t)
	ctx := context.Background()

	seeds := []*models.Contact{
		contactSeed(models.ContactEmail, "info@mandat.ua"),
		contactSeed(models.ContactTelegram, "mandat"),
		contactSeed(models.ContactPhone, "+380441234567"),
	}
	for i, seed := range seeds {
		require.NoError(t, repo.Create(ctx, seed))
		assert.Equal(t, i+1, seed.Position, "create must append position %d", i+1)
	}
}

func TestContactRepo_ListOrderedByPosition(t *testing.T) {
	repo := setupContactRepo(t)
	ctx := context.Background()

	for _, seed := range []*models.Contact{
		contactSeed(models.ContactEmail, "info@mandat.ua"),
		contactSeed(models.ContactTelegram, "mandat"),
		contactSeed(models.ContactPhone, "+380441234567"),
	} {
		require.NoError(t, repo.Create(ctx, seed))
	}

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, []int{1, 2, 3}, contactPositions(items))
	assert.Equal(t, models.ContactEmail, items[0].Type)
	assert.Equal(t, "info@mandat.ua", items[0].Value)
	assert.Equal(t, models.ContactTelegram, items[1].Type)
	assert.Equal(t, models.ContactPhone, items[2].Type)
}

func TestContactRepo_UpdatePersistsFields(t *testing.T) {
	repo := setupContactRepo(t)
	ctx := context.Background()

	seed := contactSeed(models.ContactEmail, "info@mandat.ua")
	require.NoError(t, repo.Create(ctx, seed))

	before, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, before, 1)

	updated := &models.Contact{
		ID:    seed.ID,
		Type:  models.ContactTelegram,
		Value: "mandat",
		Label: "Редакція",
	}
	require.NoError(t, repo.Update(ctx, updated))
	assert.Equal(t, 1, updated.Position, "update must fill Position back into the struct")
	assert.False(t, updated.CreatedAt.IsZero(), "update must fill CreatedAt back into the struct")

	got, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, models.ContactTelegram, got[0].Type)
	assert.Equal(t, "mandat", got[0].Value)
	assert.Equal(t, "Редакція", got[0].Label)
	assert.Equal(t, 1, got[0].Position, "update must not move the contact")
	assert.True(t, got[0].UpdatedAt.After(before[0].UpdatedAt) || got[0].UpdatedAt.Equal(before[0].UpdatedAt),
		"update must bump updated_at")
	assert.Equal(t, before[0].CreatedAt, got[0].CreatedAt, "update must keep created_at")
}

func TestContactRepo_UpdateNotFound(t *testing.T) {
	repo := setupContactRepo(t)

	err := repo.Update(context.Background(), contactSeed(models.ContactEmail, "ghost@mandat.ua"))

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrContactNotFound), "expected ErrContactNotFound, got %v", err)
}

func TestContactRepo_Delete(t *testing.T) {
	repo := setupContactRepo(t)
	ctx := context.Background()

	seed := contactSeed(models.ContactEmail, "info@mandat.ua")
	require.NoError(t, repo.Create(ctx, seed))
	require.NoError(t, repo.Delete(ctx, seed.ID))

	items, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestContactRepo_DeleteNotFound(t *testing.T) {
	repo := setupContactRepo(t)

	err := repo.Delete(context.Background(), uuid.New())

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrContactNotFound), "expected ErrContactNotFound, got %v", err)
}

func TestContactRepo_MoveUpSwapsWithPrevious(t *testing.T) {
	repo := setupContactRepo(t)
	ctx := context.Background()

	var ids []uuid.UUID
	for _, seed := range []*models.Contact{
		contactSeed(models.ContactEmail, "first@mandat.ua"),
		contactSeed(models.ContactTelegram, "second"),
		contactSeed(models.ContactPhone, "+380441234567"),
	} {
		require.NoError(t, repo.Create(ctx, seed))
		ids = append(ids, seed.ID)
	}

	require.NoError(t, repo.Move(ctx, ids[1], true))

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, ids[1], items[0].ID)
	assert.Equal(t, ids[0], items[1].ID)
	assert.Equal(t, ids[2], items[2].ID)
	assert.Equal(t, []int{1, 2, 3}, contactPositions(items))
}

func TestContactRepo_MoveDownSwapsWithNext(t *testing.T) {
	repo := setupContactRepo(t)
	ctx := context.Background()

	var ids []uuid.UUID
	for _, seed := range []*models.Contact{
		contactSeed(models.ContactEmail, "first@mandat.ua"),
		contactSeed(models.ContactTelegram, "second"),
		contactSeed(models.ContactPhone, "+380441234567"),
	} {
		require.NoError(t, repo.Create(ctx, seed))
		ids = append(ids, seed.ID)
	}

	require.NoError(t, repo.Move(ctx, ids[1], false))

	items, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Equal(t, ids[0], items[0].ID)
	assert.Equal(t, ids[2], items[1].ID)
	assert.Equal(t, ids[1], items[2].ID)
	assert.Equal(t, []int{1, 2, 3}, contactPositions(items))
}

func TestContactRepo_MoveEdgesAreNoOps(t *testing.T) {
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
			repo := setupContactRepo(t)
			ctx := context.Background()

			var ids []uuid.UUID
			for _, seed := range []*models.Contact{
				contactSeed(models.ContactEmail, "first@mandat.ua"),
				contactSeed(models.ContactTelegram, "second"),
				contactSeed(models.ContactPhone, "+380441234567"),
			} {
				require.NoError(t, repo.Create(ctx, seed))
				ids = append(ids, seed.ID)
			}

			require.NoError(t, repo.Move(ctx, ids[tt.index], tt.up))

			items, err := repo.List(ctx)
			require.NoError(t, err)
			require.Len(t, items, 3)
			assert.Equal(t, ids, contactIDs(items),
				"moving past the edge must leave the order unchanged")
		})
	}
}

func TestContactRepo_MoveUnknownNotFound(t *testing.T) {
	repo := setupContactRepo(t)

	err := repo.Move(context.Background(), uuid.New(), true)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errs.ErrContactNotFound), "expected ErrContactNotFound, got %v", err)
}
