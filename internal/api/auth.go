package api

import (
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
