package commons

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"golang.org/x/xerrors"
)

type MDRepoTicket struct {
	IRODSTicket   string
	IRODSDataPath string
}

// AES key must be 16bytes len
func padAesKey(key string) string {
	paddedKey := fmt.Sprintf("%s%s", key, aesPadding)
	return paddedKey[:16]
}

func AesDecrypt(key string, data []byte) ([]byte, error) {
	key = padAesKey(key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	decrypter := cipher.NewCBCDecrypter(block, []byte(aesIV))

	dest := make([]byte, len(data))
	decrypter.CryptBlocks(dest, data)

	dataLen := binary.LittleEndian.Uint32(dest[:4])
	return dest[4 : 4+dataLen], nil
}

func AesEncrypt(key string, data []byte) ([]byte, error) {
	key = padAesKey(key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	encrypter := cipher.NewCBCEncrypter(block, []byte(aesIV))

	blockSize := block.BlockSize()

	dataToWrite := len(data) + 4

	numBlocks := dataToWrite / blockSize
	if dataToWrite%blockSize != 0 {
		numBlocks += 1
	}

	paddedSize := numBlocks * blockSize
	paddedData := make([]byte, paddedSize)

	// add size header
	binary.LittleEndian.PutUint32(paddedData, uint32(len(data)))
	// add data
	copy(paddedData[4:], data)

	dest := make([]byte, paddedSize)

	encrypter.CryptBlocks(dest, paddedData)
	return dest, nil
}

func EncodeMDRepoTicket(ticket *MDRepoTicket, password string) (string, error) {
	hashedPassword, err := HashStringMD5(password)
	if err != nil {
		return "", xerrors.Errorf("failed to MD5 hash password: %w", err)
	}

	payload := fmt.Sprintf("%s:%s", ticket.IRODSTicket, ticket.IRODSDataPath)
	rawTicket, err := AesEncrypt(hashedPassword, []byte(payload))
	if err != nil {
		return "", xerrors.Errorf("failed to AES encode ticket string: %w", err)
	}

	ticketString := base64.RawStdEncoding.EncodeToString(rawTicket)
	return ticketString, nil
}

func DecodeMDRepoTicket(ticket string, password string) (*MDRepoTicket, error) {
	hashedPassword, err := HashStringMD5(password)
	if err != nil {
		return nil, xerrors.Errorf("failed to MD5 hash password: %w", err)
	}

	rawTicket, err := base64.RawStdEncoding.DecodeString(ticket)
	if err != nil {
		return nil, xerrors.Errorf("failed to Base64 decode ticket string: %w", err)
	}

	payload, err := AesDecrypt(hashedPassword, rawTicket)
	if err != nil {
		return nil, xerrors.Errorf("failed to AES decode ticket string: %w", err)
	}

	ticketParts := strings.Split(string(payload), ":")
	if len(ticketParts) == 0 {
		return nil, xerrors.Errorf("failed to parse ticket parts")
	}

	irodsTicket := ticketParts[0]
	irodsDataPath := ""
	if len(ticketParts) >= 2 {
		irodsDataPath = ticketParts[1]
	}

	if len(irodsTicket) == 0 {
		return nil, xerrors.Errorf("failed to parse iRODS ticket")
	}

	return &MDRepoTicket{
		IRODSTicket:   irodsTicket,
		IRODSDataPath: irodsDataPath,
	}, nil
}
