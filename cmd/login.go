package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/x402-Systems/entropy/internal/api"
	"github.com/x402-Systems/entropy/internal/config"
	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

var (
	xmrRPCURL string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Link a wallet identity (EVM or Monero)",
}

var evmCmd = &cobra.Command{
	Use:   "evm",
	Short: "Link an EVM wallet using a Private Key",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Enter Private Key (Will not be displayed): ")
		byteKey, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			fmt.Printf("❌ Error reading key: %v\n", err)
			return
		}

		privKey := strings.TrimSpace(string(byteKey))
		address, err := api.DeriveAddress(privKey)
		if err != nil {
			fmt.Println("❌ Invalid Private Key: Could not derive EVM address.")
			return
		}

		keyring.Set(config.KeyringService, config.UserAccount+"-key", privKey)
		keyring.Set(config.KeyringService, config.UserAccount+"-addr", address)

		fmt.Println("✅ EVM Identity linked successfully.")
		fmt.Printf("Management Address: %s\n", address)
	},
}

var xmrCmd = &cobra.Command{
	Use:   "xmr",
	Short: "Link a Monero wallet via monero-wallet-rpc",
	Run: func(cmd *cobra.Command, args []string) {
		if xmrRPCURL == "" {
			xmrRPCURL = config.DefaultMoneroRPC
		}

		fmt.Printf("📡 Connecting to Monero Wallet RPC at %s...\n", xmrRPCURL)

		address, err := fetchMoneroAddress(xmrRPCURL)
		if err != nil {
			fmt.Printf("❌ Connection Failed: %v\n", err)
			fmt.Println("Ensure monero-wallet-rpc is running and the wallet is open.")
			return
		}

		keyring.Set(config.KeyringService, config.UserAccount+"-xmr-rpc", xmrRPCURL)
		keyring.Set(config.KeyringService, config.UserAccount+"-xmr-addr", address)

		fmt.Println("✅ Monero Wallet linked successfully.")
		fmt.Printf("Primary Address: %s\n", address)

		if _, err := keyring.Get(config.KeyringService, config.UserAccount+"-addr"); err != nil {
			entropyID := api.DeriveMoneroID(address)
			fmt.Printf("Entropy Management ID: %s\n", entropyID)
		}
	},
}

func fetchMoneroAddress(rpcURL string) (string, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "0",
		"method":  "get_address",
		"params":  map[string]interface{}{"account_index": 0},
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var rpcResp struct {
		Result struct {
			Address string `json:"address"`
		} `json:"result"`
		Error interface{} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", fmt.Errorf("failed to parse RPC response")
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("RPC Error: %v", rpcResp.Error)
	}

	return rpcResp.Result.Address, nil
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.AddCommand(evmCmd)
	loginCmd.AddCommand(xmrCmd)

	xmrCmd.Flags().StringVarP(&xmrRPCURL, "rpc", "u", "", "Monero wallet RPC URL")
}
