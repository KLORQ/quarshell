package slogx

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskingFunctions(t *testing.T) {
	t.Run("Email", func(t *testing.T) {
		assert.Equal(t, "an***@gmail.com", maskEmail("antonioh@gmail.com"))
		assert.Equal(t, "a***@ya.ru", maskEmail("a@ya.ru"))
	})

	t.Run("Phone", func(t *testing.T) {
		assert.Equal(t, "791*****456", maskPhone("+7 911 222 3456"))
	})

	t.Run("Card", func(t *testing.T) {
		assert.Equal(t, "4276 **** **** 0000", maskCard("4276123456780000"))
	})
}

func TestCorporateDetect(t *testing.T) {
	m := &CorporateMasker{Fingerprint: false}
	assert.Contains(t, fmt.Sprint(m.Mask("AKIAIOSFODNN7EXAMPLE", MaskDefault)), "TOKEN")
	assert.Contains(t, fmt.Sprint(m.Mask("eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.signature", MaskDefault)), "JWT")

	// Dotted op names must not auto-detect as JWT (handler only masks on detect hit).
	_, hit := detectSecret("cli.policy.check")
	assert.False(t, hit)
	_, hit = detectSecret("mysql.AnalyticsStorage.TeamSummaries")
	assert.False(t, hit)
	_, hit = detectSecret("gateway.sandboxes.upsert")
	assert.False(t, hit)
}

func TestLookupMaskSuffix(t *testing.T) {
	keys := CorporateMaskRules().Keys()
	mt, ok := lookupMask(keys, "user.password")
	assert.True(t, ok)
	assert.Equal(t, MaskSecret, mt)
	mt, ok = lookupMask(keys, "my_api_key")
	assert.True(t, ok)
	assert.Equal(t, MaskToken, mt)
}
