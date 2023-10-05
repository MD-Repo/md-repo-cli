package commons

import (
	"crypto/hmac"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"golang.org/x/crypto/sha3"
	"golang.org/x/xerrors"
)

func HashLocalFileMD5(sourcePath string) (string, error) {
	hashAlg := md5.New()
	return HashLocalFile(sourcePath, hashAlg)
}

func HashLocalFile(sourcePath string, hashAlg hash.Hash) (string, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return "", xerrors.Errorf("failed to open file %s: %w", sourcePath, err)
	}

	defer f.Close()

	_, err = io.Copy(hashAlg, f)
	if err != nil {
		return "", xerrors.Errorf("failed to write: %w", err)
	}

	sumBytes := hashAlg.Sum(nil)
	sumString := hex.EncodeToString(sumBytes)

	return sumString, nil
}

func HashStringMD5(str string) (string, error) {
	return HashStrings([]string{str}, md5.New())
}

func HashStringsMD5(strs []string) (string, error) {
	return HashStrings(strs, md5.New())
}

func HashStringSHA224(str string) (string, error) {
	return HashStrings([]string{str}, sha3.New224())
}

func HashStringsSHA224(strs []string) (string, error) {
	return HashStrings(strs, sha3.New224())
}

func HashStrings(strs []string, hashAlg hash.Hash) (string, error) {
	for _, str := range strs {
		_, err := hashAlg.Write([]byte(str))
		if err != nil {
			return "", xerrors.Errorf("failed to write: %w", err)
		}
	}

	sumBytes := hashAlg.Sum(nil)
	sumString := hex.EncodeToString(sumBytes)

	return sumString, nil
}

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
