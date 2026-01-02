package cmd

import (
	"encoding/json"
	"entropy/internal/api"
	"entropy/internal/db"
	"fmt"
	"io"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

// RemoteVM represents the JSON structure returned by the server /list endpoint
type RemoteVM struct {
	ProviderID    int64     `json:"ProviderID"`
	IP            string    `json:"IP"`
	ExpiresAt     time.Time `json:"ExpiresAt"`
	TimeRemaining string    `json:"time_remaining"`
}

type ListResponse struct {
	Count int        `json:"count"`
	VMs   []RemoteVM `json:"vms"`
}

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all VMs in your local and remote registry",
	Run: func(cmd *cobra.Command, args []string) {
		var locals []db.LocalVM
		db.DB.Order("expires_at desc").Find(&locals)

		client, err := api.NewClient()
		if err != nil {
			fmt.Printf("⚠️  Offline Mode: %v\n", err)
			renderTable(locals, nil)
			return
		}

		fmt.Println("📡 Syncing with X402 Gateway...")
		resp, err := client.DoRequest(cmd.Context(), "GET", "/list", nil, nil)

		var remotes map[int64]RemoteVM
		if err == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var listResp ListResponse
			if json.Unmarshal(body, &listResp) == nil {
				remotes = make(map[int64]RemoteVM)
				for _, r := range listResp.VMs {
					remotes[r.ProviderID] = r
				}
			}
		}

		renderTable(locals, remotes)
	},
}

func renderTable(locals []db.LocalVM, remotes map[int64]RemoteVM) {
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")).Bold(true).Padding(0, 1)
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		Headers("ALIAS", "IP_ADDRESS", "TIER", "REGION", "STATUS", "TTL")

	for _, l := range locals {
		status := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444")).Render("EXPIRED")
		ttl := "0s"

		if r, ok := remotes[l.ProviderID]; ok {
			status = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")).Render("ALIVE")
			ttl = r.TimeRemaining

			// Update local IP if it was pending
			if l.IP == "IP-Allocating" && r.IP != "" {
				db.DB.Model(&l).Update("ip", r.IP)
				l.IP = r.IP
			}
		}

		t.Row(
			l.Alias,
			l.IP,
			l.Tier,
			l.Region,
			status,
			ttl,
		)
	}

	fmt.Println(headerStyle.Render("\n[ X402_FLEET_MANIFEST ]"))
	fmt.Println(t.Render())
	fmt.Printf("\nTotal tracked nodes: %d\n", len(locals))
}

func init() {
	rootCmd.AddCommand(lsCmd)
}
