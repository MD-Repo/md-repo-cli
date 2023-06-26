package test

import (
	"encoding/base64"
	"testing"

	"github.com/MD-Repo/md-repo-cli/commons"
	"github.com/stretchr/testify/assert"
)

func TestDecrypt(t *testing.T) {
	t.Run("test Decrypt", testDecrypt)
}

func testDecrypt(t *testing.T) {
	encrypted := "KgAAABIrA1QySmPzaRzuw9b+pbdBTlOuog74L//vVvGZkCuRJs54ehWZmFfmjHdVdtVmhg=="
	hashedPassword := "as1902398301sseevbn"
	payloadExpected := "payload_test_axvb2129043:slxxcive_39f9g9g3"

	rawTicket, err := base64.StdEncoding.DecodeString(encrypted)
	assert.NoError(t, err)

	payload, err := commons.AesDecrypt(hashedPassword, rawTicket)
	assert.NoError(t, err)

	assert.Equal(t, payloadExpected, string(payload))
}
