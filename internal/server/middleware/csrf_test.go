package middleware

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSRFToken(t *testing.T) {
	a, b := newCSRFToken(), newCSRFToken()
	assert.Len(t, a, csrfTokenLen)
	assert.NotEqual(t, a, b)
	assert.True(t, validCSRFToken(a))

	assert.False(t, validCSRFToken(""))
	assert.False(t, validCSRFToken(a[:63]))
	assert.False(t, validCSRFToken(strings.Repeat("z", 64)), "не hex")
	assert.False(t, validCSRFToken(a+"0"))
}

func TestSameOrigin(t *testing.T) {
	assert.True(t, sameOrigin("http://localhost:8080", "localhost:8080"))
	assert.True(t, sameOrigin("https://kt.example", "kt.example"))
	assert.False(t, sameOrigin("http://evil.example", "localhost:8080"))
	assert.False(t, sameOrigin("http://localhost:8081", "localhost:8080"))
	assert.False(t, sameOrigin("null", "localhost:8080"))
	assert.False(t, sameOrigin("::::", "localhost:8080"))
}
