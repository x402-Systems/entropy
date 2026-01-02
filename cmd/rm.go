package cmd

import (
	"entropy/internal/api"
	"entropy/internal/db"
	"fmt"
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

		fmt.Printf("🗑️ Sending teardown signal for %s...\n", alias)

		headers := map[string]string{"X-VM-NAME": vm.ServerName}
		resp, err := client.DoRequest(cmd.Context(), "DELETE", "/provision?"+params.Encode(), nil, headers)
		if err != nil || resp.StatusCode != 200 {
			fmt.Println("❌ Teardown failed.")
			return
		}

		// Remove from local DB
		db.DB.Delete(&vm)
		fmt.Println("✅ VM destroyed and removed from registry.")
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
