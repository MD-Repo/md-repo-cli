package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicket(t *testing.T) {
	t.Run("test AES", testAES)
	t.Run("test Ticket", testTicket)
}

func testAES(t *testing.T) {
	data := "ticketstr123abc902#2134:/iplant/home/iychoi/data123"
	key := "aes_key1234"

	encrypted, err := AesEncrypt(key, []byte(data))
	assert.NoError(t, err)

	decrypted, err := AesDecrypt(key, encrypted)
	assert.NoError(t, err)

	assert.Equal(t, string(data), string(decrypted))
}

func testTicket(t *testing.T) {
	irodsTicket := "ticketstr123abc902#2134"
	irodsDataPath := "/iplant/home/iychoi/data123"
	key := "aes_key1234"

	ticket := MDRepoTicket{
		IRODSTicket:   irodsTicket,
		IRODSDataPath: irodsDataPath,
	}

	mdrepoTicket, err := EncodeMDRepoTicket(&ticket, key)
	assert.NoError(t, err)

	decryptedTicket, err := DecodeMDRepoTicket(mdrepoTicket, key)
	assert.NoError(t, err)

	assert.Equal(t, ticket.IRODSTicket, decryptedTicket.IRODSTicket)
	assert.Equal(t, ticket.IRODSDataPath, decryptedTicket.IRODSDataPath)
}
