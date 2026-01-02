package cmd

import (
	"bytes"
	"encoding/json"
	"entropy/internal/api"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

var (
	notifMethod string
	notifURL    string
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Configure VM expiry alerts (Telegram or Webhook)",
	Long:  `Sets your notification preferences. For Telegram, the system will provide a magic link to link your account.`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.NewClient()
		if err != nil {
			fmt.Printf("❌ Auth Error: %v\n", err)
			return
		}

		payload := map[string]string{
			"method":     notifMethod,
			"identifier": notifURL,
		}
		jsonBytes, _ := json.Marshal(payload)

		fmt.Printf("📡 Requesting %s alerts for wallet %s...\n", notifMethod, client.Address)
		fmt.Println("💰 This registration requires a $0.0001 anti-spam payment. Checking wallet...")

		resp, err := client.DoRequest(cmd.Context(), "POST", "/notifications", bytes.NewReader(jsonBytes), map[string]string{
			"Content-Type": "application/json",
		})
		if err != nil {
			fmt.Printf("❌ Request failed: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			fmt.Printf("❌ Server Error (%d): %s\n", resp.StatusCode, string(body))
			return
		}

		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			fmt.Printf("❌ Failed to parse response: %v\n", err)
			return
		}

		methodLower := strings.ToLower(notifMethod)
		if methodLower == "telegram" {
			fmt.Println("\n🤖 TELEGRAM_LINK_GENERATED")
			fmt.Printf("MAGIC_LINK: %s\n", result["link"])
			fmt.Printf("INSTRUCTIONS: %s\n", result["instructions"])
			fmt.Println("\nNote: Alerts will not be active until you click 'Start' in the bot.")
		} else {
			fmt.Printf("\n✅ %s alerts configured successfully.\n", strings.ToUpper(notifMethod))
		}
	},
}

func init() {
	rootCmd.AddCommand(notifyCmd)

	// Flags
	notifyCmd.Flags().StringVarP(&notifMethod, "method", "m", "telegram", "Notification method (telegram, webhook)")
	notifyCmd.Flags().StringVarP(&notifURL, "id", "i", "", "Webhook URL (required if method is webhook)")
}
