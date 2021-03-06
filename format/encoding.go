package format

import (
	"encoding/base64"
	"fmt"

	"code.cloudfoundry.org/bbs/encryption"
)

type Encoding [EncodingOffset]byte

var (
	LEGACY_UNENCODED Encoding = [2]byte{}
	UNENCODED        Encoding = [2]byte{'0', '0'}
	BASE64           Encoding = [2]byte{'0', '1'}
	BASE64_ENCRYPTED Encoding = [2]byte{'0', '2'}
)

const EncodingOffset int = 2

type encoder struct {
	cryptor encryption.Cryptor
}

type Encoder interface {
	Encode(encoding Encoding, payload []byte) ([]byte, error)
	Decode(payload []byte) ([]byte, error)
}

func NewEncoder(cryptor encryption.Cryptor) Encoder {
	return &encoder{cryptor: cryptor}
}

func (e *encoder) Encode(encoding Encoding, payload []byte) ([]byte, error) {
	switch encoding {
	case LEGACY_UNENCODED:
		return payload, nil
	case UNENCODED:
		return append(encoding[:], payload...), nil
	case BASE64:
		encoded := encodeBase64(payload)
		return append(encoding[:], encoded...), nil
	case BASE64_ENCRYPTED:
		encrypted, err := e.encrypt(payload)
		if err != nil {
			return nil, err
		}
		encoded := encodeBase64(encrypted)
		return append(encoding[:], encoded...), nil
	default:
		return nil, fmt.Errorf("Unknown encoding: %v", encoding)
	}
}

func (e *encoder) Decode(payload []byte) ([]byte, error) {
	encoding := encodingFromPayload(payload)
	switch encoding {
	case LEGACY_UNENCODED:
		return payload, nil
	case UNENCODED:
		return payload[EncodingOffset:], nil
	case BASE64:
		return decodeBase64(payload[EncodingOffset:])
	case BASE64_ENCRYPTED:
		encrypted, err := decodeBase64(payload[EncodingOffset:])
		if err != nil {
			return nil, err
		}
		return e.decrypt(encrypted)
	default:
		return nil, fmt.Errorf("Unknown encoding: %v", encoding)
	}
}

func (e *encoder) encrypt(cleartext []byte) ([]byte, error) {
	encrypted, err := e.cryptor.Encrypt(cleartext)
	if err != nil {
		return nil, err
	}

	payload := []byte{}
	payload = append(payload, byte(len(encrypted.KeyLabel)))
	payload = append(payload, []byte(encrypted.KeyLabel)...)
	payload = append(payload, encrypted.Nonce...)
	payload = append(payload, encrypted.CipherText...)

	return payload, nil
}

func (e *encoder) decrypt(encryptedData []byte) ([]byte, error) {
	labelLength := encryptedData[0]
	encryptedData = encryptedData[1:]

	label := string(encryptedData[:labelLength])
	encryptedData = encryptedData[labelLength:]

	nonce := encryptedData[:encryption.NonceSize]
	ciphertext := encryptedData[encryption.NonceSize:]

	return e.cryptor.Decrypt(encryption.Encrypted{
		KeyLabel:   label,
		Nonce:      nonce,
		CipherText: ciphertext,
	})
}

func encodeBase64(unencodedPayload []byte) []byte {
	encodedLen := base64.StdEncoding.EncodedLen(len(unencodedPayload))
	encodedPayload := make([]byte, encodedLen)
	base64.StdEncoding.Encode(encodedPayload, unencodedPayload)
	return encodedPayload
}

func decodeBase64(encodedPayload []byte) ([]byte, error) {
	decodedLen := base64.StdEncoding.DecodedLen(len(encodedPayload))
	decodedPayload := make([]byte, decodedLen)
	n, err := base64.StdEncoding.Decode(decodedPayload, encodedPayload)
	return decodedPayload[:n], err
}

func encodingFromPayload(payload []byte) Encoding {
	if !isEncoded(payload) {
		return LEGACY_UNENCODED
	}
	return Encoding{payload[0], payload[1]}
}

func isEncoded(payload []byte) bool {
	if len(payload) < EncodingOffset {
		return false
	}

	if payload[0] < '0' || payload[0] > '9' {
		return false
	}

	return true
}
