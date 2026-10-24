package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify_UkrainianTitle(t *testing.T) {
	assert.Equal(t,
		"uriad-skhvalyv-novyi-biudzhet-na-2026-rik",
		slugify("Уряд схвалив новий бюджет на 2026 рік"),
	)
	assert.Equal(t,
		"nbu-zminyv-oblikovu-stavku",
		slugify("НБУ змінив облікову ставку"),
	)
}

func TestSlugify_PunctuationCollapses(t *testing.T) {
	assert.Equal(t, "pryvit-svit-hello", slugify("Привіт,  світ — hello!"))
	assert.Equal(t, "a-b", slugify("a---b"))
	assert.Equal(t, "ab", slugify("--ab--"))
}

func TestSlugify_OnlyEmojiGivesEmpty(t *testing.T) {
	assert.Equal(t, "", slugify("🔥🔥🔥"))
}
