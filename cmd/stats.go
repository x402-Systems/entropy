package cmd

import (
	"encoding/json"
	"entropy/internal/config"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Check global orchestrator health and capacity",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("📡 Querying %s...\n", config.BaseURL)

		resp, err := http.Get(config.BaseURL + "/stats")
		if err != nil {
			fmt.Printf("❌ Orchestrator unreachable: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var stats map[string]interface{}
		json.Unmarshal(body, &stats)

		if outputJSON {
			data, _ := json.MarshalIndent(stats, "", "  ")
			fmt.Println(string(data))
			return
		}

		fmt.Println("\n[ X402_SYSTEM_TELEMETRY ]")
		fmt.Printf("STATUS:      %v\n", stats["status"])
		fmt.Printf("ACTIVE_VMS:  %v\n", stats["active_vms"])
		fmt.Printf("GATEWAY_CPU: %v%%\n", stats["cpu_usage"])
		fmt.Printf("UPTIME:      %v seconds\n", stats["uptime"])
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
