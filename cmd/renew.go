package cmd

import (
	"entropy/internal/api"
	"entropy/internal/db"
	"fmt"
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

		client, err := api.NewClient()
		if err != nil {
			fmt.Println(err)
			return
		}

		params := url.Values{}
		params.Add("vm_name", vm.ServerName)
		params.Add("duration", duration)

		fmt.Printf("⏳ Renewing %s for another %s...\n", alias, duration)

		headers := map[string]string{"X-VM-NAME": vm.ServerName, "X-VM-DURATION": duration}
		resp, err := client.DoRequest(cmd.Context(), "POST", "/renew?"+params.Encode(), nil, headers)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println("❌ Renewal failed. Check balance or if VM is already reaped.")
			return
		}

		fmt.Println("✅ Lease extended.")
	},
}

func init() {
	rootCmd.AddCommand(renewCmd)
}
