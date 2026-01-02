package cmd

import (
	"entropy/internal/api"
	"fmt"
	//"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Securely link your EVM wallet using a Private Key",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Enter Private Key (Will not be displayed): ")
		byteKey, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Printf("❌ Error reading key: %v\n", err)
			return
		}

		privKey := strings.TrimSpace(string(byteKey))

		// Derive the address automatically
		address, err := api.DeriveAddress(privKey)
		if err != nil {
			fmt.Println("❌ Invalid Private Key: Could not derive EVM address.")
			return
		}

		// Save Key & Derived Address
		keyring.Set(keyringService, userAccount+"-key", privKey)
		keyring.Set(keyringService, userAccount+"-addr", address)

		fmt.Println("✅ Identity linked successfully.")
		fmt.Printf("Derived Address: %s\n", address)
		fmt.Println("This address will be used for all X-VM-PAYER headers.")
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
