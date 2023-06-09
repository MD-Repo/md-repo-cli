package commons

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/alexandrevicenzi/unchained/pbkdf2"
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
	hashAlg := md5.New()
	return HashStrings([]string{str}, hashAlg)
}

func HashStringsMD5(strs []string) (string, error) {
	hashAlg := md5.New()
	return HashStrings(strs, hashAlg)
}

func HashStringPBKDF2SHA256(str string) (string, error) {
	return pbkdf2.NewPBKDF2SHA256Hasher().Encode(str, pbkdf2SHA256HasherSalt, pbkdf2SHA256HasherIterations)
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
