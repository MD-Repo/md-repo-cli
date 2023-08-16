package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicket(t *testing.T) {
	//t.Run("test PasswordHash", testPasswordHash)
	//t.Run("test AES", testAES)
	//t.Run("test DecodeTicket1", testDecodeTicket1)
	t.Run("test DecodeTicket2", testDecodeTicket2)
	//t.Run("test SingleTicket", testSingleTicket)
	//t.Run("test MultiTickets", testMultiTickets)
}

func testPasswordHash(t *testing.T) {
	key := "catgutBiascowing"
	hash := "pbkdf2_sha256$260000$8675309$sGB5GVBEPcSE4lVyVbAL60ls+brsct2bT8hKPI1d8Fo="

	hashedPassword, err := HashStringPBKDF2SHA256(key)
	assert.NoError(t, err)

	assert.Equal(t, hash, hashedPassword)
}

func testAES(t *testing.T) {
	data := "ticketstr123abc902#2134:/iplant/home/iychoi/data123;ticketstr345efv932#2424:/iplant/home/iychoi/data345"
	key := "aes_key1234"

	encrypted, err := AesEncrypt(key, []byte(data))
	assert.NoError(t, err)

	decrypted, err := AesDecrypt(key, encrypted)
	assert.NoError(t, err)

	assert.Equal(t, string(data), string(decrypted))
}

func testDecodeTicket1(t *testing.T) {
	mdrepoTicketString := "bAAAABlJ/tQ16tgo3e2dvsnV/USMDg5tmSrtxIiaiWJb5mt1TkDadJ+E3JYBArgW47uK3Dwtwlf+JujrmsT2cGAaPXOc/IiGHvMDPMe7M5+voWO040QEQhtQM40j1XrhAES7uv2U6mZAwyl30ZPJvl77XZU="
	key := "duetbobakthem"

	decryptedMDRepoTicket, err := DecodeMDRepoTickets(mdrepoTicketString, key)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(decryptedMDRepoTicket), 1)
}

func testDecodeTicket2(t *testing.T) {
	mdrepoTicketString := "bwAAAPqbphAPISAHe6Zfj0JT8bMDvPI4ElRQy1OpkWQS8IHWZFzNOrqkBWEG1mJz2K0UJhk3SxHlBKLMRdoyplWsNCuP39zRM+eM86S773497LG0z6nyRgNCU5loxrraSHmoiuRKG62tPRUHDWqJtDX6BD4="
	key := "catgutBiascowing"

	decryptedMDRepoTicket, err := DecodeMDRepoTickets(mdrepoTicketString, key)
	assert.NoError(t, err)

	assert.GreaterOrEqual(t, len(decryptedMDRepoTicket), 1)
}

func testSingleTicket(t *testing.T) {
	irodsTicket := "ticketstr123abc902#2134"
	irodsDataPath := "/iplant/home/iychoi/data123"
	key := "aes_key1234"

	ticket := MDRepoTicket{
		IRODSTicket:   irodsTicket,
		IRODSDataPath: irodsDataPath,
	}

	mdrepoTicketString, err := EncodeMDRepoTickets([]MDRepoTicket{ticket}, key)
	assert.NoError(t, err)

	decryptedMDRepoTicket, err := DecodeMDRepoTickets(mdrepoTicketString, key)
	assert.NoError(t, err)

	assert.Len(t, decryptedMDRepoTicket, 1)
	assert.Equal(t, ticket, decryptedMDRepoTicket[0])
}

func testMultiTickets(t *testing.T) {
	irodsTicket1 := "ticketstr123abc902#2134"
	irodsDataPath1 := "/iplant/home/iychoi/data123"
	irodsTicket2 := "ticketstr345efv932#2424"
	irodsDataPath2 := "/iplant/home/iychoi/data345"
	key := "aes_key1234"

	tickets := []MDRepoTicket{
		{
			IRODSTicket:   irodsTicket1,
			IRODSDataPath: irodsDataPath1,
		},
		{
			IRODSTicket:   irodsTicket2,
			IRODSDataPath: irodsDataPath2,
		},
	}

	mdrepoTicketString, err := EncodeMDRepoTickets(tickets, key)
	assert.NoError(t, err)

	decryptedMDRepoTicket, err := DecodeMDRepoTickets(mdrepoTicketString, key)
	assert.NoError(t, err)

	assert.Len(t, decryptedMDRepoTicket, 2)
	assert.Equal(t, tickets, decryptedMDRepoTicket)
}
