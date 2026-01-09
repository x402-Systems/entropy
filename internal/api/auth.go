package api

import (
	"crypto/sha256"
	"fmt"
	evmsigners "github.com/coinbase/x402/go/signers/evm"
)

// DeriveAddress takes a hex private key and returns the 0x address
func DeriveAddress(hexPrivKey string) (string, error) {
	signer, err := evmsigners.NewClientSignerFromPrivateKey(hexPrivKey)
	if err != nil {
		return "", err
	}
	return signer.Address(), nil
}

// DeriveMoneroID creates a stable identity string from a Monero address.
// We hash it so the primary address isn't leaked in plaintext headers.
func DeriveMoneroID(address string) string {
	h := sha256.New()
	h.Write([]byte("entropy-v1-" + address))
	return fmt.Sprintf("xmr-%x", h.Sum(nil))
}
