package ui

import (
	"entropy/internal/db"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	red   = lipgloss.Color("#FF0000")
	grey  = lipgloss.Color("#444444")
	white = lipgloss.Color("#FFFFFF")
	black = lipgloss.Color("#080808")

	// Styles
	headerStyle = lipgloss.NewStyle().
			Foreground(white).
			Background(red).
			Padding(0, 1).
			Bold(true).
			Italic(true).
			MarginBottom(1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderLeftForeground(red).
			PaddingLeft(2)

	detailKeyStyle = lipgloss.NewStyle().
			Foreground(grey).
			Width(15).
			Render

	detailValStyle = lipgloss.NewStyle().
			Foreground(white).
			Bold(true).
			Render

	helpStyle = lipgloss.NewStyle().Foreground(grey).MarginTop(1)
)

type Model struct {
	table    table.Model
	wallet   string
	status   string
	terminal string // "LIST", "PROVISIONING", "SSH"
	width    int
	height   int
}

// InitialModel sets up the base UI state
func InitialModel(walletAddr string) Model {
	columns := []table.Column{
		{Title: "ALIAS", Width: 15},
		{Title: "IP_ADDR", Width: 15},
		{Title: "TIER", Width: 10},
		{Title: "REGION", Width: 8},
		{Title: "TTL", Width: 12},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Style the table to match the brutalist look
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(grey).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(white).
		Background(red).
		Bold(true)
	t.SetStyles(s)

	return Model{
		table:    t,
		wallet:   walletAddr,
		status:   "IDLE",
		terminal: "LIST",
	}
}

// Init loads the local VMs from the database when the TUI starts
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		var vms []db.LocalVM
		// Query SQLite
		db.DB.Order("expires_at desc").Find(&vms)
		return vms
	}
}

// Update handles interactions and data updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case []db.LocalVM:
		// Map DB records to Table rows
		var rows []table.Row
		for _, v := range msg {
			ttl := time.Until(v.ExpiresAt).Round(time.Second).String()
			if time.Now().After(v.ExpiresAt) {
				ttl = "EXPIRED"
			}
			rows = append(rows, table.Row{
				v.Alias,
				v.IP,
				v.Tier,
				v.Region,
				ttl,
			})
		}
		m.table.SetRows(rows)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			// Refresh logic: Triggers Init again
			m.status = strings.ToUpper("Refreshing...")
			return m, m.Init()
		case "s":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				m.status = strings.ToUpper("Selected: " + curr[0])
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the TUI to the screen
func (m Model) View() string {
	// 1. Header
	header := headerStyle.Render("X402_SYSTEMS // AGENT_TERMINAL_V1.0")
	wallet := lipgloss.NewStyle().Foreground(grey).Render(fmt.Sprintf(" AUTH_WALLET: %s", m.wallet))
	statusLine := lipgloss.NewStyle().Foreground(red).Render(fmt.Sprintf(" STATUS: %s", m.status))

	// 2. Main Content (Table + Details Sidebar)
	var body string
	if m.terminal == "LIST" {
		tableBox := m.table.View()

		// Sidebar Details
		currRow := m.table.SelectedRow()
		var details string
		if len(currRow) > 0 {
			details = lipgloss.JoinVertical(lipgloss.Left,
				detailKeyStyle("NODE_ALIAS")+detailValStyle(currRow[0]),
				detailKeyStyle("NETWORK_IP")+detailValStyle(currRow[1]),
				detailKeyStyle("HARDWARE_TIER")+detailValStyle(currRow[2]),
				detailKeyStyle("GEO_LOCATION")+detailValStyle(currRow[3]),
				detailKeyStyle("LEASE_TTL")+detailValStyle(currRow[4]),
				"",
				lipgloss.NewStyle().Foreground(red).Render("STATUS: TRACKED"),
			)
		} else {
			details = lipgloss.NewStyle().Foreground(grey).Italic(true).Render("No nodes found.")
		}

		body = lipgloss.JoinHorizontal(lipgloss.Top,
			tableBox,
			lipgloss.NewStyle().MarginLeft(4).Render(borderStyle.Render(details)),
		)
	}

	// 3. Footer / Help
	help := helpStyle.Render(" q: quit • r: refresh • s: select • n: new (cli) • entropy ssh <alias>")

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		wallet,
		statusLine,
		"\n",
		body,
		help,
	)
}
