package commons

import (
	"bytes"
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

func padPkcs7(data []byte, blocksize int) []byte {
	n := blocksize - (len(data) % blocksize)
	pb := make([]byte, len(data)+n)
	copy(pb, data)
	copy(pb[len(data):], bytes.Repeat([]byte{byte(n)}, n))
	return pb
}

func AesDecrypt(key string, data []byte) ([]byte, error) {
	key = padAesKey(key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	decrypter := cipher.NewCBCDecrypter(block, []byte(aesIV))
	contentLength := binary.LittleEndian.Uint32(data[:4])

	dest := make([]byte, len(data[4:]))
	decrypter.CryptBlocks(dest, data[4:])

	return dest[:contentLength], nil
}

func AesEncrypt(key string, data []byte) ([]byte, error) {
	key = padAesKey(key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	encrypter := cipher.NewCBCEncrypter(block, []byte(aesIV))

	contentLength := uint32(len(data))
	padData := padPkcs7(data, block.BlockSize())

	dest := make([]byte, len(padData)+4)

	// add size header
	binary.LittleEndian.PutUint32(dest, contentLength)
	encrypter.CryptBlocks(dest[4:], padData)

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

	ticketString := base64.StdEncoding.EncodeToString(rawTicket)
	return ticketString, nil
}

func GetMDRepoTicketFromPlainText(ticket string) (*MDRepoTicket, error) {
	ticketParts := strings.Split(string(ticket), ":")
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

func DecodeMDRepoTicket(ticket string, password string) (*MDRepoTicket, error) {
	hashedPassword, err := HashStringMD5(password)
	if err != nil {
		return nil, xerrors.Errorf("failed to MD5 hash password: %w", err)
	}

	rawTicket, err := base64.StdEncoding.DecodeString(ticket)
	if err != nil {
		return nil, xerrors.Errorf("failed to Base64 decode ticket string: %w", err)
	}

	payload, err := AesDecrypt(hashedPassword, rawTicket)
	if err != nil {
		return nil, xerrors.Errorf("failed to AES decode ticket string: %w", err)
	}

	return GetMDRepoTicketFromPlainText(string(payload))
}
