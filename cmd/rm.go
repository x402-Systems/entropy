package cmd

import (
	"fmt"
	"github.com/x402-Systems/entropy/internal/api"
	"github.com/x402-Systems/entropy/internal/db"
	"net/url"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm [alias]",
	Short: "Immediately destroy a VM",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		alias := args[0]

		var vm db.LocalVM
		if err := db.DB.Where("alias = ? OR server_name = ?", alias, alias).First(&vm).Error; err != nil {
			fmt.Printf("❌ VM [%s] not found in local registry.\n", alias)
			return
		}

		client, err := api.NewClient(payMethod)
		if err != nil {
			fmt.Println(err)
			return
		}

		params := url.Values{}
		params.Add("vm_name", vm.ServerName)

		if !outputJSON {
			fmt.Printf("🗑️ Sending teardown signal for %s...\n", vm.Alias)
		}

		headers := map[string]string{"X-VM-NAME": vm.ServerName}
		resp, err := client.DoRequest(cmd.Context(), "DELETE", "/provision?"+params.Encode(), nil, headers)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println("❌ Teardown failed. The server may have already reaped this instance.")
			return
		}

		if err := db.DB.Delete(&vm).Error; err != nil {
			if !outputJSON {
				fmt.Printf("⚠️  VM destroyed on server but local DB update failed: %v\n", err)
			}
		}

		if outputJSON {
			fmt.Printf(`{"status": "success", "action": "destroy", "alias": "%s", "server_name": "%s"}`+"\n", vm.Alias, vm.ServerName)
			return
		}

		fmt.Printf("✅ VM [%s] destroyed and removed from local registry.\n", vm.Alias)
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
