package commons

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

var (
	WrongPasswordError error = xerrors.Errorf("wrong password")
	InvalidTicketError error = xerrors.Errorf("invalid ticket string")
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

func EncodeMDRepoTickets(tickets []MDRepoTicket, password string) (string, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "EncodeMDRepoTickets",
	})

	hashedPassword, err := HashStringPBKDF2SHA256(password)
	if err != nil {
		return "", xerrors.Errorf("failed to MD5 hash password: %w", err)
	}

	logger.Debugf("password hash string: '%s'", hashedPassword)
	hashedPasswordParts := strings.Split(hashedPassword, "$")
	hash := hashedPassword
	if len(hashedPasswordParts) >= 4 {
		hash = hashedPasswordParts[3]
	}
	logger.Debugf("actual password hash string: '%s'", hash)

	sb := strings.Builder{}
	for _, ticket := range tickets {
		if sb.Len() != 0 {
			sb.WriteString(";")
		}
		sb.WriteString(fmt.Sprintf("%s:%s", ticket.IRODSTicket, ticket.IRODSDataPath))
	}

	payload := sb.String()
	rawTicket, err := AesEncrypt(hash, []byte(payload))
	if err != nil {
		return "", xerrors.Errorf("failed to AES encode ticket string: %w", err)
	}

	ticketString := base64.StdEncoding.EncodeToString(rawTicket)
	return ticketString, nil
}

func ValidateMDRepoTicket(ticketString string) error {
	tickets := strings.Split(ticketString, ";")
	if len(tickets) < 1 {
		return xerrors.Errorf("failed to parse tickets: %w", InvalidTicketError)
	}

	for _, ticket := range tickets {
		fmt.Printf("ticket: %s\n", ticket)

		ticketParts := strings.Split(string(ticket), ":")
		if len(ticketParts) != 2 {
			return xerrors.Errorf("failed to parse ticket parts. must have two parts: %w", InvalidTicketError)
		}

		irodsTicket := ticketParts[0]
		irodsDataPath := ticketParts[1]

		if !isTicketString(irodsTicket) {
			return xerrors.Errorf("failed to parse iRODS ticket. iRODS ticket string %s is invalid: %w", irodsTicket, InvalidTicketError)
		}

		if !isPathString(irodsDataPath) {
			return xerrors.Errorf("failed to parse iRODS data path. iRODS target path %s is invalid: %w", irodsDataPath, InvalidTicketError)
		}
	}

	return nil
}

func GetMDRepoTicketFromPlainText(ticket string) (MDRepoTicket, error) {
	ticketParts := strings.Split(string(ticket), ":")
	if len(ticketParts) != 2 {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse ticket parts. must have two parts: %w", InvalidTicketError)
	}

	irodsTicket := ticketParts[0]
	irodsDataPath := ticketParts[1]

	if !isTicketString(irodsTicket) {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse iRODS ticket. iRODS ticket string %s is invalid: %w", irodsTicket, InvalidTicketError)
	}

	if !isPathString(irodsDataPath) {
		return MDRepoTicket{}, xerrors.Errorf("failed to parse iRODS data path. iRODS target path %s is invalid: %w", irodsDataPath, InvalidTicketError)
	}

	return MDRepoTicket{
		IRODSTicket:   irodsTicket,
		IRODSDataPath: irodsDataPath,
	}, nil
}

func GetMDRepoTicketsFromPlainText(ticketString string) ([]MDRepoTicket, error) {
	tickets := strings.Split(ticketString, ";")
	if len(tickets) < 1 {
		return nil, xerrors.Errorf("failed to parse tickets: %w", InvalidTicketError)
	}

	mdRepoTickets := []MDRepoTicket{}
	for _, ticket := range tickets {
		mdRepoTicket, err := GetMDRepoTicketFromPlainText(ticket)
		if err != nil {
			return nil, err
		}

		mdRepoTickets = append(mdRepoTickets, mdRepoTicket)
	}

	return mdRepoTickets, nil
}

func isPathString(str string) bool {
	if len(str) == 0 {
		return false
	}

	if strings.HasPrefix(str, fmt.Sprintf("/%s/", mdRepoZone)) {
		return true
	}
	return false
}

func isTicketString(str string) bool {
	if len(str) == 0 {
		return false
	}

	for _, s := range str {
		sb := byte(s)
		if sb < '!' || sb > '~' {
			// non ascii
			return false
		}
	}
	return true
}

func DecodeMDRepoTickets(tickets string, password string) ([]MDRepoTicket, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "DecodeMDRepoTickets",
	})

	logger.Infof("decoding tickets '%s' with password '%s'", tickets, password)

	hashedPassword, err := HashStringPBKDF2SHA256(password)
	if err != nil {
		return nil, xerrors.Errorf("failed to MD5 hash password: %w", err)
	}

	logger.Debugf("password hash string: '%s'", hashedPassword)
	hashedPasswordParts := strings.Split(hashedPassword, "$")
	hash := hashedPassword
	if len(hashedPasswordParts) >= 4 {
		hash = hashedPasswordParts[3]
	}
	logger.Debugf("actual password hash string: '%s'", hash)

	rawTicket, err := base64.StdEncoding.DecodeString(tickets)
	if err != nil {
		return nil, xerrors.Errorf("failed to Base64 decode ticket string '%s': %w", tickets, err)
	}

	payload, err := AesDecrypt(hash, rawTicket)
	if err != nil {
		return nil, xerrors.Errorf("failed to AES decode ticket string: %w", err)
	}

	logger.Debugf("decoded ticket string: '%s'", payload)

	err = ValidateMDRepoTicket(string(payload))
	if err != nil {
		logger.Error(err)
		return nil, xerrors.Errorf("failed to decode ticket string '%s': %w", string(payload), WrongPasswordError)
	}

	return GetMDRepoTicketsFromPlainText(string(payload))
}
