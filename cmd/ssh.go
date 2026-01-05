package cmd

import (
	"github.com/x402-Systems/entropy/internal/db"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh [alias]",
	Short: "Connect to a VM via SSH using its local alias",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		aliasArg := args[0]

		// 1. Lookup VM in Local SQLite
		var vm db.LocalVM
		err := db.DB.Where("alias = ? OR ip = ?", aliasArg, aliasArg).First(&vm).Error
		if err != nil {
			fmt.Printf("❌ VM [%s] not found in local registry.\n", aliasArg)
			return
		}

		// 2. Check if IP is ready
		if vm.IP == "IP-Allocating" || vm.IP == "" {
			fmt.Println("⏳ IP is still being allocated by the orchestrator. Try again in 10 seconds.")
			return
		}

		// 3. Resolve Private Key Path
		// If the user stored the .pub path, we need the private key (usually same name without .pub)
		privateKeyPath := vm.SSHKeyPath
		if strings.HasSuffix(privateKeyPath, ".pub") {
			privateKeyPath = strings.TrimSuffix(privateKeyPath, ".pub")
		}

		fmt.Printf("🚀 Connecting to %s (%s) as root...\n", vm.Alias, vm.IP)

		// 4. Build SSH Command
		// Flags explained:
		// -i: identity file
		// -o StrictHostKeyChecking=no: Don't prompt to add to known_hosts (essential for ephemeral nodes)
		// -o UserKnownHostsFile=/dev/null: Don't save the host key (prevents "Host Identification Changed" errors later)
		sshArgs := []string{
			"-i", privateKeyPath,
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR", // Hide the "Warning: Permanently added..." message
			fmt.Sprintf("root@%s", vm.IP),
		}

		// 5. Execute SSH
		// We use os.Exec to replace the current 'entropy' process with the 'ssh' process
		// This ensures signals (Ctrl+C) and terminal resizing work perfectly.
		c := exec.Command("ssh", sshArgs...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		err = c.Run()
		if err != nil {
			// Check if it's just a normal exit
			if exitError, ok := err.(*exec.ExitError); ok {
				os.Exit(exitError.ExitCode())
			}
			fmt.Printf("❌ SSH session closed with error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(sshCmd)
}
