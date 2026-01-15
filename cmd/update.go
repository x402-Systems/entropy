package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/x402-Systems/entropy/internal/updater"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Check for and install updates for Entropy",
	Run: func(cmd *cobra.Command, args []string) {
		available, newVer, err := updater.CheckUpdateAvailable()
		if err != nil {
			fmt.Printf("❌ Update check failed: %v\n", err)
			return
		}

		if !available {
			fmt.Println("✅ You are already using the latest version of Entropy.")
			return
		}

		fmt.Printf("🔭 A new version is available: %s\n", newVer)
		fmt.Printf("Proceed with update? [y/N]: ")

		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Update cancelled.")
			return
		}

		if err := updater.RunUpdate(); err != nil {
			fmt.Printf("❌ Update failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\n✨ UPDATE_SUCCESSFUL")
		fmt.Println("The binary has been replaced. Please restart entropy.")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
