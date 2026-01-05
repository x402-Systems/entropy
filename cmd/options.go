package cmd

import (
	"encoding/json"
	"github.com/x402-Systems/entropy/internal/config"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var optionsCmd = &cobra.Command{
	Use:   "options",
	Short: "List available hardware tiers, regions, and distros",
	Run: func(cmd *cobra.Command, args []string) {
		if !outputJSON {
			fmt.Printf("📡 Querying available resources from %s...\n", config.BaseURL)
		}

		resp, err := http.Get(config.BaseURL + "/options")
		if err != nil {
			fmt.Printf("❌ Orchestrator unreachable: %v\n", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var options map[string]interface{}
		if err := json.Unmarshal(body, &options); err != nil {
			fmt.Printf("❌ Failed to parse manifest: %v\n", err)
			return
		}

		if outputJSON {
			fmt.Println(string(body))
			return
		}

		renderOptions(options)
	},
}

func renderOptions(data map[string]interface{}) {
	red := lipgloss.Color("#FF0000")
	grey := lipgloss.Color("#444444")
	headerStyle := lipgloss.NewStyle().Foreground(red).Bold(true).MarginTop(1)

	fmt.Println(headerStyle.Render("[ AVAILABLE_HARDWARE_TIERS ]"))
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(grey)).
		Headers("TIER", "CPU", "RAM", "DISK", "REGIONS (EST. HOURLY)")

	if tiers, ok := data["tiers"].(map[string]interface{}); ok {
		for name, info := range tiers {
			val := info.(map[string]interface{})

			regList := []string{}
			if regions, ok := val["Regions"].(map[string]interface{}); ok {
				for regName, regInfo := range regions {
					ri := regInfo.(map[string]interface{})
					regList = append(regList, fmt.Sprintf("%s ($%v)", regName, ri["HourlyCost"]))
				}
			}

			t.Row(
				name,
				fmt.Sprintf("%v", val["CPU"]),
				fmt.Sprintf("%v", val["RAM"]),
				fmt.Sprintf("%v", val["Disk"]),
				strings.Join(regList, ", "),
			)
		}
	}
	fmt.Println(t.Render())

	fmt.Println(headerStyle.Render("[ SUPPORTED_DISTROS ]"))
	if distros, ok := data["distros"].([]interface{}); ok {
		fmt.Printf(" %v\n", distros)
	}

	fmt.Println(headerStyle.Render("[ GEO_REGIONS ]"))
	if regions, ok := data["regions"].([]interface{}); ok {
		fmt.Printf(" %v\n", regions)
	}

	if note, ok := data["note"].(string); ok {
		fmt.Printf("\n%s\n", lipgloss.NewStyle().Foreground(grey).Italic(true).Render("NOTE: "+note))
	}
}

func init() {
	rootCmd.AddCommand(optionsCmd)
}
