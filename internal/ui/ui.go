package ui

import (
	"context"
	"encoding/json"
	"entropy/internal/api"
	"entropy/internal/db"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message types
type syncMsg struct {
	rows    []table.Row
	remotes map[int64]api.RemoteVM
}
type tickMsg time.Time
type errMsg error
type statusMsg string

var (
	red   = lipgloss.Color("#FF0000")
	green = lipgloss.Color("#00FF00")
	grey  = lipgloss.Color("#444444")
	white = lipgloss.Color("#FFFFFF")

	headerStyle = lipgloss.NewStyle().Foreground(white).Background(red).Padding(0, 1).Bold(true).Italic(true)
	borderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderLeftForeground(red).PaddingLeft(2)
	helpStyle   = lipgloss.NewStyle().Foreground(grey)
)

type Model struct {
	table    table.Model
	wallet   string
	status   string
	SSHToRun string
	remotes  map[int64]api.RemoteVM
	width    int
	height   int
	lastSync time.Time
}

func InitialModel(walletAddr string) Model {
	columns := []table.Column{
		{Title: "ALIAS", Width: 40},
		{Title: "STATUS", Width: 10},
		{Title: "IP_ADDR", Width: 16},
		{Title: "TTL", Width: 12},
		{Title: "REGION", Width: 8},
	}

	t := table.New(table.WithColumns(columns), table.WithFocused(true))

	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(grey).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(white).Background(red).Bold(true)
	t.SetStyles(s)

	return Model{
		table:    t,
		wallet:   walletAddr,
		status:   "INITIALIZING_SYSTEM...",
		remotes:  make(map[int64]api.RemoteVM),
		lastSync: time.Now(),
	}
}

func syncData() tea.Msg {
	var locals []db.LocalVM
	db.DB.Order("expires_at desc").Find(&locals)

	client, err := api.NewClient()
	if err != nil {
		return errMsg(err)
	}

	resp, err := client.DoRequest(context.Background(), "GET", "/list", nil, nil)
	remotes := make(map[int64]api.RemoteVM)
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var listResp api.ListResponse
		json.Unmarshal(body, &listResp)
		for _, r := range listResp.VMs {
			remotes[r.ProviderID] = r
		}
	}

	rows := []table.Row{}
	for _, l := range locals {
		statusText := "DEAD"
		ttl := "0s"
		if _, ok := remotes[l.ProviderID]; ok {
			statusText = "ALIVE"
		}

		rows = append(rows, table.Row{
			l.Alias,
			statusText,
			l.IP,
			ttl,
			l.Region,
		})
	}
	return syncMsg{rows: rows, remotes: remotes}
}

func doTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(syncData, doTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(m.height - 12)
		return m, nil

	case syncMsg:
		m.remotes = msg.remotes
		m.table.SetRows(msg.rows)
		m.status = "FLEET_SYNCED"
		m.lastSync = time.Now()
		return m, nil

	case tickMsg:
		var locals []db.LocalVM
		db.DB.Find(&locals)

		newRows := []table.Row{}
		for _, l := range locals {
			statusText := "DEAD"
			ttl := "0s"
			if r, ok := m.remotes[l.ProviderID]; ok {
				statusText = "ALIVE"
				remaining := time.Until(r.ExpiresAt).Round(time.Second)
				if remaining > 0 {
					ttl = remaining.String()
				}
			}
			newRows = append(newRows, table.Row{l.Alias, statusText, l.IP, ttl, l.Region})
		}
		m.table.SetRows(newRows)

		if time.Since(m.lastSync) > 30*time.Second {
			return m, tea.Batch(doTick(), syncData)
		}
		return m, doTick()

	case statusMsg:
		m.status = string(msg)
		return m, syncData

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "ctrl+r":
			m.status = "FORCING_SYNC..."
			return m, syncData
		case "s":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				m.SSHToRun = curr[0]
				return m, tea.Quit
			}
		case "r":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				alias := curr[0]
				return m, func() tea.Msg {
					var vm db.LocalVM
					db.DB.Where("alias = ?", alias).First(&vm)
					client, _ := api.NewClient()
					params := url.Values{}
					params.Add("vm_name", vm.ServerName)
					params.Add("duration", "1h")
					client.DoRequest(context.Background(), "POST", "/renew?"+params.Encode(), nil, nil)
					return statusMsg("RENEWED_" + alias)
				}
			}
		case "d":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				alias := curr[0]
				return m, func() tea.Msg {
					var vm db.LocalVM
					if err := db.DB.Where("alias = ?", alias).First(&vm).Error; err == nil {
						client, _ := api.NewClient()
						params := url.Values{}
						params.Add("vm_name", vm.ServerName)
						client.DoRequest(context.Background(), "DELETE", "/provision?"+params.Encode(), nil, nil)
						db.DB.Delete(&vm)
						return statusMsg("DESTROYED_" + alias)
					}
					return statusMsg("VM_NOT_FOUND")
				}
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		return "INITIALIZING_VIRTUAL_TERMINAL..."
	}

	header := headerStyle.Render("X402_SYSTEMS // AGENT_TERMINAL_V1.1")
	wallet := lipgloss.NewStyle().Foreground(grey).Render(" AUTH_WALLET: " + m.wallet)

	tableBox := m.table.View()

	currRow := m.table.SelectedRow()
	var details string
	if len(currRow) > 0 {
		stColor := grey
		if currRow[1] == "ALIVE" {
			stColor = green
		}

		details = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(grey).Render("NODE_ALIAS:    ")+lipgloss.NewStyle().Foreground(white).Render(currRow[0]),
			lipgloss.NewStyle().Foreground(grey).Render("CURRENT_IP:    ")+lipgloss.NewStyle().Foreground(white).Render(currRow[2]),
			lipgloss.NewStyle().Foreground(grey).Render("LEASE_TTL:     ")+lipgloss.NewStyle().Foreground(white).Render(currRow[3]),
			lipgloss.NewStyle().Foreground(grey).Render("GEO_REGION:    ")+lipgloss.NewStyle().Foreground(white).Render(currRow[4]),
			"",
			lipgloss.NewStyle().Foreground(grey).Render("STATUS:        ")+lipgloss.NewStyle().Foreground(stColor).Bold(true).Render(currRow[1]),
			lipgloss.NewStyle().Foreground(grey).Render("MGMT:          ")+lipgloss.NewStyle().Foreground(red).Render(m.status),
		)
	}

	mainContent := lipgloss.JoinHorizontal(lipgloss.Top,
		tableBox,
		lipgloss.NewStyle().PaddingLeft(2).Width(40).Render(borderStyle.Render(details)),
	)

	footer := helpStyle.Render(fmt.Sprintf(" %dx%d • ctrl+r: sync • s: ssh • r: renew • d: delete • q: quit", m.width, m.height))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		wallet,
		"\n",
		mainContent,
		"\n",
		footer,
	)
}
