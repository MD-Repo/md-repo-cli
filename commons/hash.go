package commons

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"hash/adler32"
	"io"
	"os"
	"strings"

	"github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/crypto/sha3"
	"golang.org/x/xerrors"
)

func HashLocalFile(sourcePath string, hashAlg string) (string, error) {
	switch strings.ToLower(hashAlg) {
	case strings.ToLower(string(types.ChecksumAlgorithmMD5)):
		hash, err := hashLocalFile(sourcePath, md5.New())
		if err != nil {
			return "", xerrors.Errorf("failed to hash local file %s with alg %s: %w", sourcePath, hashAlg, err)
		}

		return hex.EncodeToString(hash), nil
	case strings.ToLower(string(types.ChecksumAlgorithmADLER32)):
		hash, err := hashLocalFile(sourcePath, adler32.New())
		if err != nil {
			return "", xerrors.Errorf("failed to hash local file %s with alg %s: %w", sourcePath, hashAlg, err)
		}

		return hex.EncodeToString(hash), nil
	case strings.ToLower(string(types.ChecksumAlgorithmSHA1)):
		hash, err := hashLocalFile(sourcePath, sha1.New())
		if err != nil {
			return "", xerrors.Errorf("failed to hash local file %s with alg %s: %w", sourcePath, hashAlg, err)
		}

		return base64.StdEncoding.EncodeToString(hash), nil
	case strings.ToLower(string(types.ChecksumAlgorithmSHA256)):
		hash, err := hashLocalFile(sourcePath, sha256.New())
		if err != nil {
			return "", xerrors.Errorf("failed to hash local file %s with alg %s: %w", sourcePath, hashAlg, err)
		}

		return base64.StdEncoding.EncodeToString(hash), nil
	case strings.ToLower(string(types.ChecksumAlgorithmSHA512)):
		hash, err := hashLocalFile(sourcePath, sha512.New())
		if err != nil {
			return "", xerrors.Errorf("failed to hash local file %s with alg %s: %w", sourcePath, hashAlg, err)
		}

		return base64.StdEncoding.EncodeToString(hash), nil
	default:
		return "", xerrors.Errorf("unknown hash algorithm %s", hashAlg)
	}
}

func hashLocalFile(sourcePath string, hashAlg hash.Hash) ([]byte, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return nil, xerrors.Errorf("failed to open file %s: %w", sourcePath, err)
	}

	defer f.Close()

	_, err = io.Copy(hashAlg, f)
	if err != nil {
		return nil, xerrors.Errorf("failed to write: %w", err)
	}

	sumBytes := hashAlg.Sum(nil)
	return sumBytes, nil
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
