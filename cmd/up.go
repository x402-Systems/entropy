package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/x402-Systems/entropy/internal/api"
	"github.com/x402-Systems/entropy/internal/db"
	"github.com/x402-Systems/entropy/internal/sshmgr"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type ProvisionResponse struct {
	Status string `json:"status"`
	VM     struct {
		ProviderID int64     `json:"ProviderID"`
		Name       string    `json:"Name"`
		IP         string    `json:"IP"`
		Tier       string    `json:"Tier"`
		Region     string    `json:"Region"`
		Password   string    `json:"Password"`
		ExpiresAt  time.Time `json:"ExpiresAt"`
	} `json:"vm"`
}

var (
	tier     string
	distro   string
	region   string
	duration string
	sshKey   string
	alias    string
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Provision a new ephemeral VM",
	Long:  `Triggers an x402 payment and provisions a VM. Metadata is saved locally.`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.NewClient(payMethod)
		if err != nil {
			fmt.Printf("❌ Auth Error: %v\n", err)
			return
		}

		if sshKey == "" {
			generatedPath, err := sshmgr.GetDefaultKey()
			if err != nil {
				fmt.Printf("❌ SSH Key Manager error: %v\n", err)
				return
			}
			sshKey = generatedPath
		}

		keyContent, err := os.ReadFile(sshKey)
		if err != nil {
			fmt.Printf("❌ Failed to read SSH key file [%s]: %v\n", sshKey, err)
			return
		}
		finalSSHKey := strings.TrimSpace(string(keyContent))

		headers := map[string]string{
			"X-VM-TIER":     tier,
			"X-VM-DURATION": duration,
			"X-VM-REGION":   region,
		}

		params := url.Values{}
		params.Add("tier", tier)
		params.Add("distro", distro)
		params.Add("duration", duration)
		params.Add("ssh_key", finalSSHKey)

		if !outputJSON {
			fmt.Printf("📡 Initializing provisioning for %s tier (%s)...\n", tier, duration)
			fmt.Println("💰 This request requires an x402 payment. Checking wallet...")
		}

		validateResp, err := client.DoRequest(cmd.Context(), "POST", "/validate", nil, headers)
		if err == nil {
			defer validateResp.Body.Close()
			if validateResp.StatusCode == 403 {
				body, _ := io.ReadAll(validateResp.Body)
				fmt.Printf("❌ Eligibility check failed: %s\n", string(body))
				return
			}
		}

		path := fmt.Sprintf("/provision?%s", params.Encode())
		resp, err := client.DoRequest(cmd.Context(), "POST", path, nil, headers)
		if err != nil {
			fmt.Printf("❌ Provisioning failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			fmt.Printf("❌ Server Error (%d): %s\n", resp.StatusCode, string(body))
			return
		}

		var result ProvisionResponse
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("❌ Failed to parse server response: %v\n", err)
			return
		}

		localVM := db.LocalVM{
			ProviderID:  result.VM.ProviderID,
			ServerName:  result.VM.Name,
			Alias:       alias,
			IP:          result.VM.IP,
			Tier:        result.VM.Tier,
			Region:      result.VM.Region,
			ExpiresAt:   result.VM.ExpiresAt,
			SSHKeyPath:  sshKey,
			OwnerWallet: client.PayerID,
		}

		if localVM.Alias == "" {
			localVM.Alias = result.VM.Name
		}

		if err := db.DB.Create(&localVM).Error; err != nil {
			fmt.Printf("⚠️  VM provisioned but failed to save to local DB: %v\n", err)
		}

		if outputJSON {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return
		}
		// 7. Final Output
		fmt.Println("\n✨ PROVISION_SUCCESSFUL")
		fmt.Printf("ID:       %d\n", result.VM.ProviderID)
		fmt.Printf("NAME:     %s\n", result.VM.Name)
		fmt.Printf("ALIAS:    %s\n", localVM.Alias)
		fmt.Printf("IP:       %s\n", result.VM.IP)
		fmt.Printf("PASSWORD: %s\n", result.VM.Password)
		fmt.Printf("EXPIRES:  %s\n", result.VM.ExpiresAt.Format(time.RFC1123))
		fmt.Println("\nRun 'entropy ssh " + localVM.Alias + "' to connect once the IP is live.")
	},
}

func init() {
	rootCmd.AddCommand(upCmd)

	upCmd.Flags().StringVarP(&tier, "tier", "t", "eco-small", "Hardware tier")
	upCmd.Flags().StringVarP(&distro, "distro", "d", "ubuntu-24.04", "OS Distro")
	upCmd.Flags().StringVarP(&region, "region", "r", "nbg1", "Region")
	upCmd.Flags().StringVarP(&duration, "duration", "l", "1h", "Lease duration")
	upCmd.Flags().StringVarP(&sshKey, "key", "k", "", "Path to public SSH key")
	upCmd.Flags().StringVarP(&alias, "alias", "a", "", "Local nickname")
}
