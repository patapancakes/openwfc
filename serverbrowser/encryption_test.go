package serverbrowser

import (
	"fmt"
	"testing"
)

func TestEncryption(t *testing.T) {
	encrypted := EncryptTypeX([]byte("key"), []byte("challenge"), []byte("data"))
	for i := range encrypted {
		fmt.Printf("%02x ", encrypted[i])
	}
	fmt.Printf("\n")
}
