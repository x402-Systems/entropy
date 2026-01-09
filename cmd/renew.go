package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/x402-Systems/entropy/internal/api"
	"github.com/x402-Systems/entropy/internal/db"
	"io"
	"net/url"

	"github.com/spf13/cobra"
)

var renewCmd = &cobra.Command{
	Use:   "renew [alias]",
	Short: "Extend the lease of an active VM",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		alias := args[0]

		var vm db.LocalVM
		if err := db.DB.Where("alias = ?", alias).First(&vm).Error; err != nil {
			fmt.Printf("❌ VM [%s] not found.\n", alias)
			return
		}

		client, err := api.NewClient(payMethod)
		if err != nil {
			fmt.Println(err)
			return
		}

		params := url.Values{}
		params.Add("vm_name", vm.ServerName)
		params.Add("duration", duration)

		if !outputJSON {
			fmt.Printf("⏳ Renewing %s for another %s...\n", alias, duration)
		}

		headers := map[string]string{"X-VM-NAME": vm.ServerName, "X-VM-DURATION": duration}
		resp, err := client.DoRequest(cmd.Context(), "POST", "/renew?"+params.Encode(), nil, headers)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println("❌ Renewal failed. Check balance or if VM is already reaped.")
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var serverRes struct {
			Status    string `json:"status"`
			NewExpiry string `json:"new_expiry"`
		}
		json.Unmarshal(body, &serverRes)

		if outputJSON {
			res := map[string]interface{}{
				"status":     "success",
				"action":     "renew",
				"alias":      alias,
				"duration":   duration,
				"new_expiry": serverRes.NewExpiry,
			}
			data, _ := json.MarshalIndent(res, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Printf("✅ Lease extended by %s. New expiry: %s\n", duration, serverRes.NewExpiry)
	},
}

func init() {
	rootCmd.AddCommand(renewCmd)
}
