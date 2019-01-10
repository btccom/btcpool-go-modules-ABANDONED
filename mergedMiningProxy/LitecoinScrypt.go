package main

import (
	"golang.org/x/crypto/scrypt"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func HashSHA256(input []byte) []byte {
	hash := sha256.New()
	hash.Write(input)
	return hash.Sum(nil)
}

func DoubleSHA256(input []byte) []byte {
	return HashSHA256(HashSHA256(input))
}

func DecodeHexString(input string) ([]byte, error) {
	return hex.DecodeString(input)
}

func HexToString(input []byte) string {
	return hex.EncodeToString(input)
}

func ArrayReverse(input []byte) []byte {
	for i, j := 0, len(input)-1; i < j; i, j = i+1, j-1 {
		input[i], input[j] = input[j], input[i]
	}
	return input
}

func Scrypt(input []byte) ([]byte, error) {
	key, err := scrypt.Key(input, input, 1024, 1, 1, 32)
	if err != nil {
		err = fmt.Errorf("Unable to generate scrypt key: %s", err)
		return key, err
	}
	return key, err
}
/*
func main() {
	header := "01000000f615f7ce3b4fc6b8f61e8f89aedb1d0852507650533a9e3b10b9bbcc30639f279fcaa86746e1ef52d3edb3c4ad8259920d509bd073605c9bf1d59983752a6b06b817bb4ea78e011d012d59d4"
	data, err := DecodeHexString(header)

	//if err != nil {
	//	fmt.Errorf("Unable to decode header hex string: %s", err)
	//	return
	//}

	//result := DoubleSHA256(data)
	//fmt.Println(HexToString(result))
	//fmt.Println(HexToString(ArrayReverse(result)))

	dk, err := Scrypt(data)
	if err != nil {
		fmt.Errorf("Unable to generate scrypt key: %s", err)
		return
	}

	fmt.Println(hex.EncodeToString(dk))
	fmt.Println(hex.EncodeToString(ArrayReverse(dk)))
}
*/