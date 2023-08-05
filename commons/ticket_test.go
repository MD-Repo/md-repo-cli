package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicket(t *testing.T) {
	t.Run("test AES", testAES)
	t.Run("test SingleTicket", testSingleTicket)
	t.Run("test MultiTickets", testMultiTickets)
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
