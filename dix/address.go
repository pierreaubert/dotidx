// copy pasted from substrate-api-rpc Apache 2

package dix

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"

	"golang.org/x/crypto/blake2b"
)

const (
	// alphabet is the modified base58 alphabet used by Bitcoin.
	alphabet     = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	alphabetIdx0 = '1'
)

var b58 = [256]byte{
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 0, 1, 2, 3, 4, 5, 6,
	7, 8, 255, 255, 255, 255, 255, 255,
	255, 9, 10, 11, 12, 13, 14, 15,
	16, 255, 17, 18, 19, 20, 21, 255,
	22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 255, 255, 255, 255, 255,
	255, 33, 34, 35, 36, 37, 38, 39,
	40, 41, 42, 43, 255, 44, 45, 46,
	47, 48, 49, 50, 51, 52, 53, 54,
	55, 56, 57, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
	255, 255, 255, 255, 255, 255, 255, 255,
}

func IntInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func AddHex(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if strings.HasPrefix(s, "0x") {
		return s
	}
	return strings.ToLower("0x" + s)
}

func TrimHex(s string) string {
	return strings.TrimPrefix(s, "0x")
}

func IntToHex(i interface{}) string {
	return fmt.Sprintf("%x", i)
}

func HexToBytes(s string) []byte {
	s = strings.TrimPrefix(s, "0x")
	c := make([]byte, hex.DecodedLen(len(s)))
	_, _ = hex.Decode(c, []byte(s))
	return c
}

func BytesToHex(b []byte) string {
	c := make([]byte, hex.EncodedLen(len(b)))
	hex.Encode(c, b)
	return string(c)
}

var bigRadix = big.NewInt(58)
var bigZero = big.NewInt(0)

// Decode decodes a modified base58 string to a byte slice.
func Base58Decode(b string) []byte {
	answer := big.NewInt(0)
	j := big.NewInt(1)

	scratch := new(big.Int)
	for i := len(b) - 1; i >= 0; i-- {
		tmp := b58[b[i]]
		if tmp == 255 {
			return []byte("")
		}
		scratch.SetInt64(int64(tmp))
		scratch.Mul(j, scratch)
		answer.Add(answer, scratch)
		j.Mul(j, bigRadix)
	}

	tmpval := answer.Bytes()

	var numZeros int
	for numZeros = 0; numZeros < len(b); numZeros++ {
		if b[numZeros] != alphabetIdx0 {
			break
		}
	}
	flen := numZeros + len(tmpval)
	val := make([]byte, flen)
	copy(val[numZeros:], tmpval)

	return val
}

// Encode encodes a byte slice to a modified base58 string.
func Base58Encode(b []byte) string {
	x := new(big.Int)
	x.SetBytes(b)

	answer := make([]byte, 0, len(b)*136/100)
	for x.Cmp(bigZero) > 0 {
		mod := new(big.Int)
		x.DivMod(x, bigRadix, mod)
		answer = append(answer, alphabet[mod.Int64()])
	}

	// leading zero bytes
	for _, i := range b {
		if i != 0 {
			break
		}
		answer = append(answer, alphabetIdx0)
	}

	// reverse
	alen := len(answer)
	for i := 0; i < alen/2; i++ {
		answer[i], answer[alen-1-i] = answer[alen-1-i], answer[i]
	}

	return string(answer)
}

func SS58Decode(address string, addressType int) string {
	checksumPrefix := []byte("SS58PRE")
	ss58Format := Base58Decode(address)
	if len(ss58Format) == 0 || ss58Format[0] != byte(addressType) {
		log.Printf("first failed %d", ss58Format[0])
		return ""
	}
	var checksumLength int
	if IntInSlice(len(ss58Format), []int{3, 4, 6, 10}) {
		checksumLength = 1
	} else if IntInSlice(len(ss58Format), []int{5, 7, 11, 35}) {
		checksumLength = 2
	} else if IntInSlice(len(ss58Format), []int{8, 12}) {
		checksumLength = 3
	} else if IntInSlice(len(ss58Format), []int{9, 13}) {
		checksumLength = 4
	} else if IntInSlice(len(ss58Format), []int{14}) {
		checksumLength = 5
	} else if IntInSlice(len(ss58Format), []int{15}) {
		checksumLength = 6
	} else if IntInSlice(len(ss58Format), []int{16}) {
		checksumLength = 7
	} else if IntInSlice(len(ss58Format), []int{17}) {
		checksumLength = 8
	} else {
		log.Printf("checksum failed")
		return ""
	}
	bss := ss58Format[0 : len(ss58Format)-checksumLength]
	checksum, _ := blake2b.New(64, []byte{})
	w := append(checksumPrefix[:], bss[:]...)
	_, err := checksum.Write(w)
	if err != nil {
		log.Printf("checksum2 failed")
		return ""
	}

	h := checksum.Sum(nil)
	if BytesToHex(h[0:checksumLength]) != BytesToHex(ss58Format[len(ss58Format)-checksumLength:]) {
		log.Printf("byte2hex failed")
		return ""
	}
	return BytesToHex(ss58Format[1 : len(ss58Format)-checksumLength])
}

func SS58Encode(address string, addressType int) string {
	checksumPrefix := []byte("SS58PRE")
	addressBytes := HexToBytes(address)
	var checksumLength int
	if len(addressBytes) == 32 {
		checksumLength = 2
	} else if IntInSlice(len(addressBytes), []int{1, 2, 4, 8}) {
		checksumLength = 1
	} else {
		return ""
	}
	addressFormat := append([]byte{byte(addressType)}[:], addressBytes[:]...)
	checksum, _ := blake2b.New(64, []byte{})
	w := append(checksumPrefix[:], addressFormat[:]...)
	_, err := checksum.Write(w)
	if err != nil {
		return ""
	}

	h := checksum.Sum(nil)
	b := append(addressFormat[:], h[:checksumLength][:]...)
	return Base58Encode(b)
}
