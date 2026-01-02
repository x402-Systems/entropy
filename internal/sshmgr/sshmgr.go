package sshmgr

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func GetDefaultKey() (string, error) {
	home, _ := os.UserHomeDir()
	keyDir := filepath.Join(home, ".config", "entropy", "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return "", err
	}

	privPath := filepath.Join(keyDir, "id_ed25519")
	pubPath := privPath + ".pub"

	if _, err := os.Stat(privPath); err == nil {
		return pubPath, nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}

	privBytes, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return "", err
	}
	privPem := pem.EncodeToMemory(privBytes)

	if err := os.WriteFile(privPath, privPem, 0600); err != nil {
		return "", err
	}

	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return "", err
	}
	pubBytes := ssh.MarshalAuthorizedKey(sshPub)
	if err := os.WriteFile(pubPath, pubBytes, 0644); err != nil {
		return "", err
	}

	fmt.Printf("🗝️ Generated new anonymous keypair: %s\n", pubPath)
	return pubPath, nil
}
