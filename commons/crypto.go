package commons

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"

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

func HashStringSHA256(str string) (string, error) {
	hashAlg := sha256.New()
	return HashStrings([]string{str}, hashAlg)
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

	contentLength := binary.LittleEndian.Uint32(data[:4])

	decrypter := cipher.NewCBCDecrypter(block, []byte(aesIV))

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

	encrypter.CryptBlocks(dest[4:], padData)

	// add size header
	binary.LittleEndian.PutUint32(dest, contentLength)

	return dest, nil
}
