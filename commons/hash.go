package commons

import (
	"crypto/hmac"
	"encoding/base64"
	"hash"

	"golang.org/x/crypto/sha3"
	"golang.org/x/xerrors"
)

func HMACStringSHA224(secret []byte, str string) (string, error) {
	return HMACStrings(secret, []string{str}, sha3.New224)
}

func HMACStrings(secret []byte, strs []string, hashAlg func() hash.Hash) (string, error) {
	hmac := hmac.New(hashAlg, secret)

	for _, str := range strs {
		_, err := hmac.Write([]byte(str))
		if err != nil {
			return "", xerrors.Errorf("failed to write: %w", err)
		}
	}

	sumBytes := hmac.Sum(nil)

	// base64
	sumString := base64.URLEncoding.EncodeToString(sumBytes)
	return sumString, nil
}

func Base64Decode(str string) ([]byte, error) {
	return base64.URLEncoding.DecodeString(str)
}
