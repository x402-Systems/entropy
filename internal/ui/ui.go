package ui

import (
	"context"
	"encoding/json"
	"entropy/internal/api"
	"entropy/internal/config"
	"entropy/internal/db"
	"entropy/internal/sshmgr"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type sessionState int

const (
	stateList sessionState = iota
	stateProvisioning
)

// Message types
type syncMsg struct {
	rows    []table.Row
	remotes map[int64]api.RemoteVM
}
type tickMsg time.Time
type errMsg error
type statusMsg string
type provisionResultMsg struct {
	err error
}

var (
	red   = lipgloss.Color("#FF0000")
	green = lipgloss.Color("#00FF00")
	grey  = lipgloss.Color("#444444")
	white = lipgloss.Color("#FFFFFF")

	headerStyle = lipgloss.NewStyle().Foreground(white).Background(red).Padding(0, 1).Bold(true).Italic(true)
	borderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderLeftForeground(red).PaddingLeft(2)
	helpStyle   = lipgloss.NewStyle().Foreground(grey)
	inputStyle  = lipgloss.NewStyle().Foreground(red)
)

type Model struct {
	state    sessionState
	table    table.Model
	inputs   []textinput.Model
	focusIdx int
	wallet   string
	status   string
	SSHToRun string
	remotes  map[int64]api.RemoteVM
	width    int
	height   int
	lastSync time.Time
}

func InitialModel(walletAddr string) Model {
	// Table Setup
	columns := []table.Column{
		{Title: "ALIAS", Width: 30},
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

	// Inputs Setup (for Provisioning)
	inputs := make([]textinput.Model, 4)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "alias (e.g. web-prod)"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "tier (eco-small, std-med)"
	inputs[1].SetValue("eco-small")

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "region (nbg1, hel1, ash)"
	inputs[2].SetValue("nbg1")

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "duration (1h, 24h)"
	inputs[3].SetValue("1h")

	return Model{
		state:    stateList,
		table:    t,
		inputs:   inputs,
		wallet:   walletAddr,
		status:   "IDLE",
		remotes:  make(map[int64]api.RemoteVM),
		lastSync: time.Now(),
	}
}

// Command: Run the provisioning logic (Ported from up.go)
func provisionVM(alias, tier, region, duration string) tea.Cmd {
	return func() tea.Msg {
		client, err := api.NewClient()
		if err != nil {
			return provisionResultMsg{err: err}
		}

		sshPath, err := sshmgr.GetDefaultKey()
		if err != nil {
			return provisionResultMsg{err: err}
		}
		keyContent, _ := os.ReadFile(sshPath)

		params := url.Values{}
		params.Add("tier", tier)
		params.Add("distro", "ubuntu-24.04")
		params.Add("duration", duration)
		params.Add("ssh_key", strings.TrimSpace(string(keyContent)))

		headers := map[string]string{
			"X-VM-TIER":     tier,
			"X-VM-DURATION": duration,
			"X-VM-REGION":   region,
		}

		resp, err := client.DoRequest(context.Background(), "POST", "/provision?"+params.Encode(), nil, headers)
		if err != nil {
			return provisionResultMsg{err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return provisionResultMsg{err: fmt.Errorf("server returned %d", resp.StatusCode)}
		}

		var res struct {
			VM struct {
				ProviderID int64     `json:"ProviderID"`
				Name       string    `json:"Name"`
				IP         string    `json:"IP"`
				Tier       string    `json:"Tier"`
				Region     string    `json:"Region"`
				ExpiresAt  time.Time `json:"ExpiresAt"`
			} `json:"vm"`
		}
		json.NewDecoder(resp.Body).Decode(&res)

		// Save to local DB
		db.DB.Create(&db.LocalVM{
			ProviderID:  res.VM.ProviderID,
			ServerName:  res.VM.Name,
			Alias:       alias,
			IP:          res.VM.IP,
			Tier:        res.VM.Tier,
			Region:      res.VM.Region,
			ExpiresAt:   res.VM.ExpiresAt,
			SSHKeyPath:  sshPath,
			OwnerWallet: client.Address,
		})

		return provisionResultMsg{err: nil}
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
		if r, ok := remotes[l.ProviderID]; ok {
			statusText = "ALIVE"
			remaining := time.Until(r.ExpiresAt).Round(time.Second)
			if remaining > 0 {
				ttl = remaining.String()
			}
		}

		rows = append(rows, table.Row{l.Alias, statusText, l.IP, ttl, l.Region})
	}
	return syncMsg{rows: rows, remotes: remotes}
}

func doTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(syncData, doTick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetHeight(m.height - 12)

	case syncMsg:
		m.remotes = msg.remotes
		m.table.SetRows(msg.rows)
		m.status = "FLEET_SYNCED"
		m.lastSync = time.Now()

	case tickMsg:
		if m.state == stateList {
			if time.Since(m.lastSync) > 20*time.Second {
				return m, tea.Batch(doTick(), syncData)
			}
		}
		return m, doTick()

	case provisionResultMsg:
		m.state = stateList
		if msg.err != nil {
			m.status = "PROVISION_ERROR: " + msg.err.Error()
		} else {
			m.status = "PROVISION_SUCCESS"
		}
		return m, syncData

	case tea.KeyMsg:
		if m.state == stateProvisioning {
			switch msg.String() {
			case "esc":
				m.state = stateList
				return m, nil
			case "tab", "shift+tab", "up", "down":
				s := msg.String()
				if s == "up" || s == "shift+tab" {
					m.focusIdx--
				} else {
					m.focusIdx++
				}
				if m.focusIdx > len(m.inputs)-1 {
					m.focusIdx = 0
				} else if m.focusIdx < 0 {
					m.focusIdx = len(m.inputs) - 1
				}
				cmds := make([]tea.Cmd, len(m.inputs))
				for i := 0; i <= len(m.inputs)-1; i++ {
					if i == m.focusIdx {
						cmds[i] = m.inputs[i].Focus()
						continue
					}
					m.inputs[i].Blur()
				}
				return m, tea.Batch(cmds...)
			case "enter":
				m.status = "PROVISIONING_X402_NODE..."
				pCmd := provisionVM(
					m.inputs[0].Value(),
					m.inputs[1].Value(),
					m.inputs[2].Value(),
					m.inputs[3].Value(),
				)
				return m, pCmd
			}
			// Handle text input updates
			for i := range m.inputs {
				m.inputs[i], cmd = m.inputs[i].Update(msg)
			}
			return m, cmd
		}

		// Table View Keybindings
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "n": // NEW VM
			m.state = stateProvisioning
			return m, nil
		case "ctrl+r":
			m.status = "FORCING_SYNC..."
			return m, syncData
		case "s":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				m.SSHToRun = curr[0]
				return m, tea.Quit
			}
		case "d":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				alias := curr[0]
				m.status = "DESTROYING_" + alias
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

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m Model) View() string {
	if m.width == 0 {
		return "INITIALIZING_VIRTUAL_TERMINAL..."
	}

	header := headerStyle.Render(fmt.Sprintf("X402_SYSTEMS // AGENT_TERMINAL_%s", config.Version))
	wallet := lipgloss.NewStyle().Foreground(grey).Render(" AUTH_WALLET: " + m.wallet)

	var mainContent string

	if m.state == stateProvisioning {
		// Provisioning Form View
		form := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(red).Bold(true).Render("\n[ PROVISION_NEW_NODE ]"),
			"",
			"Alias:", m.inputs[0].View(),
			"Tier:", m.inputs[1].View(),
			"Region:", m.inputs[2].View(),
			"Duration:", m.inputs[3].View(),
			"",
			helpStyle.Render("enter: confirm • esc: cancel • tab: navigate"),
		)
		mainContent = lipgloss.NewStyle().Padding(1, 4).Render(form)
	} else {
		// Table View
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

		mainContent = lipgloss.JoinHorizontal(lipgloss.Top,
			tableBox,
			lipgloss.NewStyle().PaddingLeft(2).Width(40).Render(borderStyle.Render(details)),
		)
	}

	footer := helpStyle.Render(fmt.Sprintf(" %dx%d • n: new node • ctrl+r: sync • s: ssh • d: delete • q: quit", m.width, m.height))

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		wallet,
		"\n",
		mainContent,
		"\n",
		footer,
	)
}
