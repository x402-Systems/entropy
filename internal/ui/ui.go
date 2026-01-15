package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/x402-Systems/entropy/internal/api"
	"github.com/x402-Systems/entropy/internal/config"
	"github.com/x402-Systems/entropy/internal/db"
	"github.com/x402-Systems/entropy/internal/sshmgr"

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

type syncMsg struct {
	rows    []table.Row
	remotes map[int64]api.RemoteVM
}
type tickMsg time.Time
type statusMsg string
type provisionResultMsg struct {
	err error
}

var (
	red    = lipgloss.Color("#FF0000")
	green  = lipgloss.Color("#00FF00")
	yellow = lipgloss.Color("#F1C40F")
	grey   = lipgloss.Color("#444444")
	white  = lipgloss.Color("#FFFFFF")

	headerStyle = lipgloss.NewStyle().Foreground(white).Background(red).Padding(0, 1).Bold(true).Italic(true)
	borderStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderLeftForeground(red).PaddingLeft(2)
	helpStyle   = lipgloss.NewStyle().Foreground(grey)
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
	columns := []table.Column{
		{Title: "ALIAS", Width: 25},
		{Title: "STATUS", Width: 10},
		{Title: "IP_ADDR", Width: 16},
		{Title: "TTL", Width: 15}, // Widened slightly
		{Title: "REGION", Width: 8},
	}
	t := table.New(table.WithColumns(columns), table.WithFocused(true))
	s := table.DefaultStyles()
	s.Header = s.Header.BorderStyle(lipgloss.NormalBorder()).BorderForeground(grey).BorderBottom(true).Bold(false)
	s.Selected = s.Selected.Foreground(white).Background(red).Bold(true)
	t.SetStyles(s)

	inputs := make([]textinput.Model, 5)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "alias (e.g. web-prod)"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "tier (eco-small, pro)"
	inputs[1].SetValue("eco-small")

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "region (nbg1, ash)"
	inputs[2].SetValue("nbg1")

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "duration (1h, 24h)"
	inputs[3].SetValue("1h")

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "payment (usdc, xmr)"
	inputs[4].SetValue("usdc")

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

func provisionVM(alias, tier, region, duration, payMethod string) tea.Cmd {
	return func() tea.Msg {
		client, err := api.NewClient(payMethod)
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
			body, _ := io.ReadAll(resp.Body)
			return provisionResultMsg{err: fmt.Errorf("server: %s", string(body))}
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

		db.DB.Create(&db.LocalVM{
			ProviderID:  res.VM.ProviderID,
			ServerName:  res.VM.Name,
			Alias:       alias,
			IP:          res.VM.IP,
			Tier:        res.VM.Tier,
			Region:      res.VM.Region,
			ExpiresAt:   res.VM.ExpiresAt,
			SSHKeyPath:  sshPath,
			OwnerWallet: client.PayerID,
		})

		return provisionResultMsg{err: nil}
	}
}

func syncData() tea.Msg {
	var locals []db.LocalVM
	db.DB.Order("expires_at desc").Find(&locals)

	client, err := api.NewClient("usdc")
	if err != nil {
		return provisionResultMsg{err: err}
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
			// Check Status from server
			if r.Status == "suspended" {
				statusText = "PAUSED"
				ttl = "GRACE PERIOD"
			} else {
				statusText = "ALIVE"
				remaining := time.Until(r.ExpiresAt).Round(time.Second)
				if remaining > 0 {
					ttl = remaining.String()
				}
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
		m.table.SetHeight(m.height - 14)

	case syncMsg:
		m.remotes = msg.remotes
		m.table.SetRows(msg.rows)
		m.status = "FLEET_SYNCED"
		m.lastSync = time.Now()

	case tickMsg:
		if m.state == stateList {
			if time.Since(m.lastSync) > 30*time.Second {
				return m, tea.Batch(doTick(), syncData)
			}
		}
		return m, doTick()

	case provisionResultMsg:
		m.state = stateList
		if msg.err != nil {
			m.status = "ERROR: " + msg.err.Error()
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
				if msg.String() == "up" || msg.String() == "shift+tab" {
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
				for i := range m.inputs {
					if i == m.focusIdx {
						cmds[i] = m.inputs[i].Focus()
					} else {
						m.inputs[i].Blur()
					}
				}
				return m, tea.Batch(cmds...)
			case "enter":
				m.status = "X402_NEGOTIATING_PAYMENT..."
				return m, provisionVM(
					m.inputs[0].Value(),
					m.inputs[1].Value(),
					m.inputs[2].Value(),
					m.inputs[3].Value(),
					m.inputs[4].Value(),
				)
			}
			for i := range m.inputs {
				m.inputs[i], cmd = m.inputs[i].Update(msg)
			}
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "n":
			m.state = stateProvisioning
			return m, nil
		case "ctrl+r":
			m.status = "FORCING_SYNC..."
			return m, syncData
		case "s":
			curr := m.table.SelectedRow()
			if len(curr) > 0 && curr[1] == "ALIVE" {
				m.SSHToRun = curr[0]
				return m, tea.Quit
			} else if len(curr) > 0 && curr[1] == "PAUSED" {
				m.status = "VM_IS_PAUSED_RENEW_TO_ACCESS"
			}
		case "d":
			curr := m.table.SelectedRow()
			if len(curr) > 0 {
				alias := curr[0]
				m.status = "DESTROYING_" + alias
				return m, func() tea.Msg {
					var vm db.LocalVM
					if err := db.DB.Where("alias = ?", alias).First(&vm).Error; err == nil {
						client, _ := api.NewClient("usdc")
						params := url.Values{}
						params.Add("vm_name", vm.ServerName)
						client.DoRequest(context.Background(), "DELETE", "/provision?"+params.Encode(), nil, nil)
						db.DB.Delete(&vm)
						return statusMsg("DESTROYED_" + alias)
					}
					return statusMsg("NOT_FOUND")
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
	wallet := lipgloss.NewStyle().Foreground(grey).Render(" AUTH_ID: " + m.wallet)

	var mainContent string
	if m.state == stateProvisioning {
		form := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Foreground(red).Bold(true).Render("\n[ PROVISION_NEW_NODE ]"),
			"",
			"Alias:    ", m.inputs[0].View(),
			"Tier:     ", m.inputs[1].View(),
			"Region:   ", m.inputs[2].View(),
			"Duration: ", m.inputs[3].View(),
			"Payment:  ", m.inputs[4].View(),
			"",
			helpStyle.Render("enter: confirm • esc: cancel • tab: navigate"),
			lipgloss.NewStyle().Foreground(grey).Italic(true).Render("\nNote: XMR verification may take up to 2 minutes."),
		)
		mainContent = lipgloss.NewStyle().Padding(1, 4).Render(form)
	} else {
		tableBox := m.table.View()
		currRow := m.table.SelectedRow()
		var details string
		if len(currRow) > 0 {
			// Color Logic for Status
			stColor := grey
			hintText := ""

			if currRow[1] == "ALIVE" {
				stColor = green
			} else if currRow[1] == "PAUSED" {
				stColor = yellow
				hintText = lipgloss.NewStyle().Foreground(yellow).Render("\n⚠ VM IS SUSPENDED\nRun 'entropy renew " + currRow[0] + "' to restore.")
			}

			details = lipgloss.JoinVertical(lipgloss.Left,
				lipgloss.NewStyle().Foreground(grey).Render("NODE_ALIAS:    ")+lipgloss.NewStyle().Foreground(white).Render(currRow[0]),
				lipgloss.NewStyle().Foreground(grey).Render("CURRENT_IP:    ")+lipgloss.NewStyle().Foreground(white).Render(currRow[2]),
				lipgloss.NewStyle().Foreground(grey).Render("LEASE_TTL:     ")+lipgloss.NewStyle().Foreground(white).Render(currRow[3]),
				lipgloss.NewStyle().Foreground(grey).Render("GEO_REGION:    ")+lipgloss.NewStyle().Foreground(white).Render(currRow[4]),
				"",
				lipgloss.NewStyle().Foreground(grey).Render("STATUS:        ")+lipgloss.NewStyle().Foreground(stColor).Bold(true).Render(currRow[1]),
				lipgloss.NewStyle().Foreground(grey).Render("MGMT:          ")+lipgloss.NewStyle().Foreground(red).Render(m.status),
				hintText,
			)
		}
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top,
			tableBox,
			lipgloss.NewStyle().PaddingLeft(2).Width(40).Render(borderStyle.Render(details)),
		)
	}

	footer := lipgloss.JoinVertical(lipgloss.Left,
		helpStyle.Render(fmt.Sprintf(" %dx%d • n: new node • ctrl+r: sync • s: ssh • d: delete • q: quit", m.width, m.height)),
		lipgloss.NewStyle().Foreground(red).Render(" [!] AUTO-SYNC ACTIVE ($0.001/refresh)"),
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		wallet,
		"\n",
		mainContent,
		"\n",
		footer,
	)
}
