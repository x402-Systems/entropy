package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/x402-Systems/entropy/internal/api"
	"github.com/x402-Systems/entropy/internal/config"
	"github.com/x402-Systems/entropy/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
)

var (
	keyringService = "entropy-systems"
	userAccount    = "active-signer"
	payMethod      string
)

var outputJSON bool

var rootCmd = &cobra.Command{
	Use:   "entropy",
	Short: "X402 Digital Entropy CLI // Anonymous Cloud Orchestrator",
	Long: `A brutalist CLI/TUI for managing ephemeral infrastructure.
Standardized for x402 payment protocol on Base Network.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			launchTUI()
			return
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Version = config.Version
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output response in raw JSON format")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVarP(&payMethod, "pay", "p", "usdc", "Payment method (usdc or xmr)")
}

func launchTUI() {
	walletAddr, err := keyring.Get(config.KeyringService, userAccount+"-addr")
	if err != nil {
		if xmrAddr, err := keyring.Get(config.KeyringService, config.UserAccount+"-xmr-addr"); err == nil {
			walletAddr = api.DeriveMoneroID(xmrAddr)
		} else {
			walletAddr = "0xUNREGISTERED"
		}
	}

	f, err := tea.LogToFile("entropy.log", "debug")
	if err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
	defer f.Close()

	m := ui.InitialModel(walletAddr)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	if m, ok := finalModel.(ui.Model); ok && m.SSHToRun != "" {
		fmt.Printf("🔌 Dropping into SSH for: %s\n", m.SSHToRun)

		sshCmd.Run(sshCmd, []string{m.SSHToRun})
	}
}

func GetSecureKey() (string, error) {
	return keyring.Get(keyringService, userAccount+"-key")
}

func SetSecureKey(address, privateKey string) error {
	if err := keyring.Set(keyringService, userAccount+"-addr", address); err != nil {
		return err
	}
	return keyring.Set(keyringService, userAccount+"-key", privateKey)
}
