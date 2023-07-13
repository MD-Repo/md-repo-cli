package test

import (
	"testing"

	"github.com/alexandrevicenzi/unchained"
	"github.com/alexandrevicenzi/unchained/pbkdf2"
	"github.com/stretchr/testify/assert"
)

func TestCryptoDjango(t *testing.T) {
	t.Run("test Django Decrypt Password", testDjangoDecryptPass)
	t.Run("test Django Make Password", testDjangoMakePassword)
}

func testDjangoDecryptPass(t *testing.T) {
	ok, err := unchained.CheckPassword("thisisatest", "pbkdf2_sha256$260000$8675309$Tk8c+e1sGylS9u9FOvsa2VV47b/lpPJFGhOyXeEsjiE=")
	assert.NoError(t, err)
	assert.True(t, ok)
}

func testDjangoMakePassword(t *testing.T) {
	//hash, err := unchained.MakePassword("thisisatest", "8675309", "default")
	hash, err := pbkdf2.NewPBKDF2SHA256Hasher().Encode("thisisatest", "8675309", 260000)

	assert.NoError(t, err)
	assert.Equal(t, "pbkdf2_sha256$260000$8675309$Tk8c+e1sGylS9u9FOvsa2VV47b/lpPJFGhOyXeEsjiE=", hash)
}
