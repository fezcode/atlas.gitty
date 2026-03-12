package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"atlas.git/internal/config"
	"atlas.git/internal/git"
)

type sessionState int

const (
	welcomeView sessionState = iota
	repoSelectView
	mainView
	dialogView
)

type focusArea int

const (
	focusSidebar focusArea = iota
	focusLog
	focusContent
)

type tab int

const (
	tabGraph tab = iota
	tabStage
	tabBranches
	tabTags
	tabRemotes
	tabDiff
	tabHelp
)

type dialogType int

const (
	dialogCommit dialogType = iota
	dialogCreateBranch
	dialogCreateTag
	dialogCheckout
	dialogAddRepo
	dialogCloneRepo
	dialogInitRepo
	dialogAmend
	dialogCherryPick
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Help    key.Binding
	Tab     key.Binding
	ShiftTab key.Binding
	
	Fetch      key.Binding
	Pull       key.Binding
	Push       key.Binding
	Commit     key.Binding
	Amend      key.Binding
	CherryPick key.Binding
	Checkout   key.Binding
	Refresh    key.Binding
	Delete     key.Binding
}

var keys = keyMap{
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "prev tab")),
	Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next tab")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "action")),
	Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Tab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next focus")),
	ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev focus")),

	Fetch:      key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "fetch")),
	Pull:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "pull")),
	Push:       key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "push")),
	Commit:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "commit")),
	Amend:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "amend")),
	CherryPick: key.NewBinding(key.WithKeys("v"), key.WithHelp("v", "cherry-pick")),
	Checkout:   key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "switch branch")),
	Refresh:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Delete:     key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "delete")),
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.Enter, k.Back, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.ShiftTab, k.Enter, k.Back},
		{k.Fetch, k.Pull, k.Push, k.Commit, k.Amend},
		{k.CherryPick, k.Checkout, k.Refresh, k.Delete, k.Quit},
	}
}

type item struct {
	title, desc string
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type Model struct {
	state       sessionState
	focus       focusArea
	activeTab   tab
	activeDlg   dialogType
	width       int
	height      int
	cfg         *config.Config
	repoList    list.Model
	currentRepo *git.GitRepo

	// Welcome menu
	welcomeMenu []string
	welcomeIdx  int

	// Sidebar
	sidebarList list.Model

	// Main view
	commits       []string
	commitIdx     int
	currentBranch string
	
	// Stage view
	statusItems []git.StatusItem
	statusIdx   int

	// Viewports
	logViewport    viewport.Model
	contentViewport viewport.Model

	// Dialog
	textInput  textinput.Model
	textInput2 textinput.Model 
	isMultiInput bool

	// UI Helpers
	help    help.Model
	showHelp bool
	lastMsg string
	isError bool
}

func NewInitialModel() Model {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	
	items := []list.Item{}
	for _, r := range cfg.Repositories {
		items = append(items, item{title: filepath.Base(r.Path), desc: r.Path})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = SelectedStyle
	delegate.Styles.SelectedDesc = InactiveStyle

	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.Styles.Title = HeaderStyle

	// Sidebar list
	sbDelegate := list.NewDefaultDelegate()
	sbDelegate.ShowDescription = false
	sbDelegate.Styles.SelectedTitle = SelectedStyle
	sbList := list.New([]list.Item{}, sbDelegate, 0, 0)
	sbList.SetShowTitle(false)
	sbList.SetShowHelp(false)
	sbList.SetShowStatusBar(false)

	ti := textinput.New()
	ti.Prompt = " > "
	ti.Cursor.Style = CursorStyle

	ti2 := textinput.New()
	ti2.Prompt = " > "
	ti2.Cursor.Style = CursorStyle

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(Magenta)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(Gray)

	return Model{
		state:       welcomeView,
		welcomeMenu: []string{"SELECT REPOSITORY", "ADD LOCAL REPOSITORY", "CLONE REPOSITORY", "INIT REPOSITORY"},
		cfg:         cfg,
		repoList:    l,
		sidebarList: sbList,
		textInput:   ti,
		textInput2:  ti2,
		help:        h,
		focus:       focusLog,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.updateSizes()

	case tea.KeyMsg:
		if key.Matches(msg, keys.Quit) && m.state != dialogView {
			return m, tea.Quit
		}

		switch m.state {
		case welcomeView:
			switch {
			case key.Matches(msg, keys.Up):
				if m.welcomeIdx > 0 { m.welcomeIdx-- }
			case key.Matches(msg, keys.Down):
				if m.welcomeIdx < len(m.welcomeMenu)-1 { m.welcomeIdx++ }
			case key.Matches(msg, keys.Enter):
				switch m.welcomeIdx {
				case 0: m.state = repoSelectView
				case 1: m.openDialog(dialogAddRepo, "Path to local repo...")
				case 2: m.openMultiDialog(dialogCloneRepo, "Repository URL...", "Destination Path...")
				case 3: m.openDialog(dialogInitRepo, "Path to new repo...")
				}
			}

		case repoSelectView:
			switch {
			case key.Matches(msg, keys.Back):
				m.state = welcomeView
				return m, nil
			case key.Matches(msg, keys.Delete):
				if i := m.repoList.SelectedItem(); i != nil {
					path := i.(item).desc
					m.cfg.RemoveRepository(path)
					_ = m.cfg.Save()
					m.updateRepoList()
					m.setStatus("Repo removed from list.", false)
				}
				return m, nil
			case key.Matches(msg, keys.Enter):
				if i := m.repoList.SelectedItem(); i != nil {
					repo, err := git.OpenRepo(i.(item).desc)
					if err == nil {
						m.currentRepo = repo
						m.state = mainView
						m.refreshRepoData()
					} else {
						m.setStatus("Error opening repo: "+err.Error(), true)
					}
				}
			}
			m.repoList, cmd = m.repoList.Update(msg)
			return m, cmd

		case mainView:
			// Global MainView Keys
			if key.Matches(msg, keys.Tab) {
				m.focus = (m.focus + 1) % 3
				return m, nil
			}
			if key.Matches(msg, keys.ShiftTab) {
				m.focus = (m.focus + 2) % 3
				return m, nil
			}
			if key.Matches(msg, keys.Refresh) {
				m.refreshRepoData()
				m.setStatus("Refreshed.", false)
				return m, nil
			}
			if key.Matches(msg, keys.Fetch) {
				m.setStatus("Fetching...", false)
				go func() { _ = m.currentRepo.Fetch() }()
				return m, nil
			}
			if key.Matches(msg, keys.Pull) {
				m.setStatus("Pulling...", false)
				err := m.currentRepo.Pull()
				if err != nil { m.setStatus("Error: "+err.Error(), true) } else { m.setStatus("Pulled.", false); m.refreshRepoData() }
				return m, nil
			}
			if key.Matches(msg, keys.Push) {
				m.setStatus("Pushing...", false)
				err := m.currentRepo.Push()
				if err != nil { m.setStatus("Error: "+err.Error(), true) } else { m.setStatus("Pushed.", false) }
				return m, nil
			}
			if key.Matches(msg, keys.Commit) {
				m.openDialog(dialogCommit, "Commit message...")
				return m, nil
			}
			if key.Matches(msg, keys.Amend) {
				m.openDialog(dialogAmend, "Amend message (leave empty to keep current)...")
				return m, nil
			}
			if key.Matches(msg, keys.Checkout) {
				m.openDialog(dialogCheckout, "Branch name to switch to...")
				return m, nil
			}
			if key.Matches(msg, keys.CherryPick) {
				if len(m.commits) > 0 {
					hash := strings.Split(m.commits[m.commitIdx], " ")[0]
					m.openDialog(dialogCherryPick, "Cherry-pick " + hash + "? (enter to confirm)")
					m.textInput.SetValue(hash)
					return m, nil
				}
			}

			switch m.focus {
			case focusSidebar:
				switch {
				case key.Matches(msg, keys.Enter):
					if i := m.sidebarList.SelectedItem(); i != nil {
						title := i.(item).title
						if strings.HasPrefix(title, "B: ") {
							branch := strings.TrimPrefix(title, "B: ")
							branch = strings.TrimPrefix(branch, "* ")
							branch = strings.TrimSpace(branch)
							err := m.currentRepo.Checkout(branch)
							if err == nil {
								m.setStatus("Checked out "+branch, false)
								m.refreshRepoData()
							} else {
								m.setStatus("Error: "+err.Error(), true)
							}
						}
					}
				}
				m.sidebarList, cmd = m.sidebarList.Update(msg)
				cmds = append(cmds, cmd)

			case focusLog:
				switch {
				case key.Matches(msg, keys.Up):
					switch m.activeTab {
					case tabGraph:
						if m.commitIdx > 0 { m.commitIdx--; m.updateDiffFromCommit() }
					case tabStage:
						if m.statusIdx > 0 { m.statusIdx--; m.updateDiffFromStatus() }
					}
				case key.Matches(msg, keys.Down):
					switch m.activeTab {
					case tabGraph:
						if m.commitIdx < len(m.commits)-1 { m.commitIdx++; m.updateDiffFromCommit() }
					case tabStage:
						if m.statusIdx < len(m.statusItems)-1 { m.statusIdx++; m.updateDiffFromStatus() }
					}
				case key.Matches(msg, keys.Enter):
					if m.activeTab == tabStage && len(m.statusItems) > 0 {
						item := m.statusItems[m.statusIdx]
						var err error
						if item.Staged {
							err = m.currentRepo.UnstageFile(item.Path)
						} else {
							err = m.currentRepo.StageFile(item.Path)
						}
						if err == nil { m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
					}
				case key.Matches(msg, keys.Left):
					m.activeTab = (m.activeTab + 6) % 7
					m.refreshTabContent()
				case key.Matches(msg, keys.Right):
					m.activeTab = (m.activeTab + 1) % 7
					m.refreshTabContent()
				}
				m.logViewport, cmd = m.logViewport.Update(msg)
				cmds = append(cmds, cmd)

			case focusContent:
				m.contentViewport, cmd = m.contentViewport.Update(msg)
				cmds = append(cmds, cmd)
			}

			if key.Matches(msg, keys.Back) {
				m.state = welcomeView
			}

		case dialogView:
			if msg.String() == "esc" {
				m.state = mainView
				if m.currentRepo == nil { m.state = welcomeView }
			} else if msg.String() == "enter" {
				m.handleDialogSubmit()
			} else if msg.String() == "tab" && m.isMultiInput {
				if m.textInput.Focused() {
					m.textInput.Blur(); m.textInput2.Focus()
				} else {
					m.textInput2.Blur(); m.textInput.Focus()
				}
			}
			m.textInput, cmd = m.textInput.Update(msg)
			cmds = append(cmds, cmd)
			m.textInput2, cmd = m.textInput2.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) updateSizes() {
	m.repoList.SetSize(m.width-4, m.height-6)
	
	sidebarWidth := 30
	mainWidth := m.width - sidebarWidth
	headerHeight := 2 // Text + Border
	footerHeight := 2 // Text + Border
	contentHeight := m.height - headerHeight - footerHeight - 2 // -2 for JoinVertical gaps
	
	if contentHeight < 10 { contentHeight = 10 }
	
	logHeight := contentHeight / 2
	viewHeight := contentHeight - logHeight
	
	m.sidebarList.SetSize(sidebarWidth-2, contentHeight-2)
	
	m.logViewport.Width = mainWidth - 4
	m.logViewport.Height = logHeight - 3 // -1 for tab header, -2 for borders
	
	m.contentViewport.Width = mainWidth - 4
	m.contentViewport.Height = viewHeight - 2
}

func (m *Model) openDialog(t dialogType, placeholder string) {
	m.activeDlg = t
	m.state = dialogView
	m.isMultiInput = false
	m.textInput.Placeholder = placeholder
	m.textInput.SetValue("")
	m.textInput.Focus()
}

func (m *Model) openMultiDialog(t dialogType, p1, p2 string) {
	m.activeDlg = t
	m.state = dialogView
	m.isMultiInput = true
	m.textInput.Placeholder = p1
	m.textInput.SetValue("")
	m.textInput.Focus()
	m.textInput2.Placeholder = p2
	m.textInput2.SetValue("")
}

func (m *Model) handleDialogSubmit() {
	v1 := m.textInput.Value()
	if v1 == "" && m.activeDlg != dialogCommit { return }

	var err error
	switch m.activeDlg {
	case dialogAddRepo:
		repo, errOpen := git.OpenRepo(v1)
		if errOpen == nil {
			m.cfg.AddRepository(repo.Path)
			_ = m.cfg.Save()
			m.updateRepoList()
			m.setStatus("Repo added.", false)
		} else { m.setStatus("Invalid git repo: "+errOpen.Error(), true) }
		m.state = welcomeView
		return
	case dialogCloneRepo:
		v2 := m.textInput2.Value()
		m.setStatus("Cloning...", false)
		err = git.CloneRepo(v1, v2)
		if err == nil {
			m.cfg.AddRepository(v2); _ = m.cfg.Save(); m.updateRepoList()
			m.setStatus("Cloned.", false)
		} else { m.setStatus("Clone failed: "+err.Error(), true) }
		m.state = welcomeView
	case dialogInitRepo:
		err = git.InitRepo(v1)
		if err == nil {
			m.cfg.AddRepository(v1); _ = m.cfg.Save(); m.updateRepoList()
			m.setStatus("Initialized.", false)
		} else { m.setStatus("Init failed: "+err.Error(), true) }
		m.state = welcomeView
	case dialogCommit:
		err = m.currentRepo.Commit(v1)
		if err == nil {
			m.setStatus("Committed.", false)
		} else { m.setStatus("Commit failed: "+err.Error(), true) }
		m.state = mainView
		m.refreshRepoData()
	case dialogAmend:
		err = m.currentRepo.Amend(v1)
		if err == nil {
			m.setStatus("Amended.", false)
		} else { m.setStatus("Amend failed: "+err.Error(), true) }
		m.state = mainView
		m.refreshRepoData()
	case dialogCherryPick:
		err = m.currentRepo.CherryPick(v1)
		if err == nil {
			m.setStatus("Cherry-picked.", false)
		} else { m.setStatus("Cherry-pick failed: "+err.Error(), true) }
		m.state = mainView
		m.refreshRepoData()
	case dialogCheckout:
		err = m.currentRepo.Checkout(v1)
		if err == nil {
			m.setStatus("Switched to "+v1, false)
		} else { m.setStatus("Switch failed: "+err.Error(), true) }
		m.state = mainView
		m.refreshRepoData()
	}
	if err != nil { m.setStatus("Error: "+err.Error(), true) }
}

func (m *Model) updateRepoList() {
	items := []list.Item{}
	for _, r := range m.cfg.Repositories {
		items = append(items, item{title: filepath.Base(r.Path), desc: r.Path})
	}
	m.repoList.SetItems(items)
}

func (m *Model) setStatus(msg string, isErr bool) {
	m.isError = isErr
	m.lastMsg = fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
}

func (m *Model) refreshRepoData() {
	if m.currentRepo == nil { return }
	m.currentBranch, _ = m.currentRepo.GetCurrentBranch()
	m.commits, _ = m.currentRepo.GetCommits(100)
	m.statusItems, _ = m.currentRepo.GetStatusItems()
	
	// Sidebar items
	var items []list.Item
	branches, _ := m.currentRepo.GetBranches()
	for _, b := range branches {
		prefix := "  "
		if b == m.currentBranch { prefix = "* " }
		items = append(items, item{title: "B: " + prefix + b})
	}
	tags, _ := m.currentRepo.GetTags()
	for _, t := range tags {
		items = append(items, item{title: "T: " + t})
	}
	remotes, _ := m.currentRepo.GetRemotes()
	for _, r := range remotes {
		items = append(items, item{title: "R: " + r})
	}
	m.sidebarList.SetItems(items)

	m.refreshTabContent()
	
	if m.activeTab == tabStage {
		m.updateDiffFromStatus()
	} else {
		m.updateDiffFromCommit()
	}
}

func (m *Model) refreshTabContent() {
	if m.currentRepo == nil { return }
	
	switch m.activeTab {
	case tabGraph:
		g, _ := m.currentRepo.GetGraph(100)
		m.logViewport.SetContent(g)
	case tabStage:
		m.renderStageView()
	case tabBranches:
		b, _ := m.currentRepo.GetBranches()
		m.logViewport.SetContent(HeaderStyle.Render("BRANCHES") + "\n\n" + strings.Join(b, "\n"))
	case tabTags:
		t, _ := m.currentRepo.GetTags()
		m.logViewport.SetContent(HeaderStyle.Render("TAGS") + "\n\n" + strings.Join(t, "\n"))
	case tabRemotes:
		r, _ := m.currentRepo.GetRemotes()
		m.logViewport.SetContent(HeaderStyle.Render("REMOTES") + "\n\n" + strings.Join(r, "\n"))
	case tabDiff:
		d, _ := m.currentRepo.GetDiff("")
		m.logViewport.SetContent(d)
	case tabHelp:
		helpText := HeaderStyle.Render("ATLAS.GIT USAGE GUIDE") + "\n\n" +
			SelectedStyle.Render("LAYOUT") + "\n" +
			"• Sidebar (Left): Shows branches, tags, and remotes.\n" +
			"• Top Pane (Right): Active tab content (Graph, Stage, etc.).\n" +
			"• Bottom Pane (Right): Detailed diff of the selection.\n\n" +
			SelectedStyle.Render("NAVIGATION") + "\n" +
			"• Tab / Shift+Tab: Cycle focus between panes.\n" +
			"• Arrows / HJKL: Navigate lists and viewports.\n" +
			"• Left / Right: Switch between tabs (when Top Pane is focused).\n\n" +
			SelectedStyle.Render("STAGE TAB") + "\n" +
			"• Arrows: Navigate file list.\n" +
			"• Enter: Stage/Unstage selected file.\n" +
			"• c: Commit staged changes.\n\n" +
			SelectedStyle.Render("GIT COMMANDS (GLOBAL)") + "\n" +
			"• f: Fetch | p: Pull | P: Push\n" +
			"• c: Commit | a: Amend | v: Cherry-pick | S: Switch branch\n" +
			"• r: Refresh repository data\n\n" +
			SelectedStyle.Render("SIDEBAR") + "\n" +
			"• Enter: Checkout selected branch."
		m.logViewport.SetContent(helpText)
	}
}

func (m *Model) renderStageView() {
	var sb strings.Builder
	sb.WriteString(HeaderStyle.Render("STAGING AREA (Press ENTER to stage/unstage)") + "\n\n")
	if len(m.statusItems) == 0 {
		sb.WriteString(InactiveStyle.Render("  Working tree clean."))
	} else {
		for i, item := range m.statusItems {
			prefix := "  "
			if i == m.statusIdx { prefix = "> " }
			
			box := "[ ]"
			if item.Staged { box = "[x]" }
			
			line := fmt.Sprintf("%s %s %s (%s)", prefix, box, item.Path, item.Status)
			if i == m.statusIdx {
				sb.WriteString(SelectedStyle.Render(line) + "\n")
			} else if item.Staged {
				sb.WriteString(SuccessStyle.Render(line) + "\n")
			} else {
				sb.WriteString(line + "\n")
			}
		}
	}
	m.logViewport.SetContent(sb.String())
}

func (m *Model) updateDiffFromCommit() {
	if len(m.commits) > 0 && m.commitIdx < len(m.commits) {
		hash := strings.Split(m.commits[m.commitIdx], " ")[0]
		diff, err := m.currentRepo.GetCommitDiff(hash)
		if err == nil {
			m.contentViewport.SetContent(diff)
		}
	}
}

func (m *Model) updateDiffFromStatus() {
	if len(m.statusItems) > 0 && m.statusIdx < len(m.statusItems) {
		path := m.statusItems[m.statusIdx].Path
		diff, err := m.currentRepo.GetDiff(path)
		if err == nil {
			if diff == "" { diff = "No changes or binary file." }
			m.contentViewport.SetContent(diff)
		}
	} else {
		m.contentViewport.SetContent("")
	}
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 { return "Initializing..." }

	header := m.renderHeader()
	footer := m.renderFooter()

	var content string
	switch m.state {
	case welcomeView: content = m.renderWelcome()
	case repoSelectView: content = m.renderRepoSelect()
	case mainView: content = m.renderMain()
	case dialogView: content = m.renderDialog()
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m Model) renderHeader() string {
	repoName := "NO REPOSITORY"
	if m.currentRepo != nil {
		repoName = filepath.Base(m.currentRepo.Path)
	}
	
	title := HeaderStyle.Render(" ATLAS.GIT ")
	repoInfo := PathStyle.Render(" " + repoName + " ")
	branchInfo := ""
	if m.currentBranch != "" {
		branchInfo = SuccessStyle.Render("  " + m.currentBranch + " ")
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, repoInfo, branchInfo)
	
	return HeaderBoxStyle.Width(m.width).Render(left)
}

func (m Model) renderFooter() string {
	var msg string
	if m.lastMsg != "" {
		style := SuccessMessageStyle
		if m.isError { style = ErrorMessageStyle }
		msg = style.Render(m.lastMsg)
	}
	
	helpStr := m.help.View(keys)
	
	gap := m.width - lipgloss.Width(msg) - lipgloss.Width(helpStr) - 2
	if gap < 0 { gap = 0 }
	spacer := strings.Repeat(" ", gap)
	
	return FooterBoxStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, msg, spacer, helpStr),
	)
}

func (m Model) renderWelcome() string {
	var menu string
	for i, item := range m.welcomeMenu {
		if i == m.welcomeIdx { menu += SelectedStyle.Render("> " + item) + "\n" } else { menu += InactiveStyle.Render("  " + item) + "\n" }
	}
	box := MainBoxStyle.Copy().Padding(1, 4).BorderForeground(Magenta).Render(
		lipgloss.JoinVertical(lipgloss.Center, HeaderStyle.Render("ATLAS GIT CLIENT"), "\n", menu),
	)
	return lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderRepoSelect() string {
	box := MainBoxStyle.Copy().Width(m.width - 4).Height(m.height - 8).Render(m.repoList.View())
	return lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderMain() string {
	sidebarWidth := 30
	mainWidth := m.width - sidebarWidth
	headerHeight := 2
	footerHeight := 2
	contentHeight := m.height - headerHeight - footerHeight - 2
	
	if contentHeight < 10 { contentHeight = 10 }
	
	logHeight := contentHeight / 2
	viewHeight := contentHeight - logHeight

	// Sidebar
	sbStyle := MainBoxStyle.Copy().Width(sidebarWidth).Height(contentHeight)
	if m.focus == focusSidebar {
		sbStyle = sbStyle.BorderForeground(Magenta)
	}
	sidebar := sbStyle.Render(m.sidebarList.View())

	// Tabs
	tabs := []string{"LOG", "STAGE", "BRANCHES", "TAGS", "REMOTES", "DIFF", "HELP"}
	tabHeader := ""
	for i, t := range tabs {
		style := InactiveStyle.Copy().Padding(0, 1)
		if int(m.activeTab) == i {
			style = SelectedStyle.Copy().Padding(0, 1).Underline(true)
		}
		tabHeader += style.Render(t)
	}

	// Log/Graph area
	logStyle := MainBoxStyle.Copy().Width(mainWidth).Height(logHeight)
	if m.focus == focusLog {
		logStyle = logStyle.BorderForeground(Magenta)
	}
	
	logArea := logStyle.Render(lipgloss.JoinVertical(lipgloss.Left, tabHeader, m.logViewport.View()))

	// Content/Diff area
	contentStyle := MainBoxStyle.Copy().Width(mainWidth).Height(viewHeight)
	if m.focus == focusContent {
		contentStyle = contentStyle.BorderForeground(Magenta)
	}
	contentArea := contentStyle.Render(m.contentViewport.View())

	mainView := lipgloss.JoinVertical(lipgloss.Left, logArea, contentArea)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainView)
}

func (m Model) renderDialog() string {
	inputUI := m.textInput.View()
	if m.isMultiInput {
		inputUI = lipgloss.JoinVertical(lipgloss.Left, m.textInput.View(), "\n", m.textInput2.View())
	}
	ui := lipgloss.JoinVertical(lipgloss.Left, HeaderStyle.Render("INPUT"), "\n", inputUI, "\n", InactiveStyle.Render("enter: confirm | esc: cancel"))
	box := MainBoxStyle.Copy().Padding(1, 2).Width(60).BorderForeground(Magenta)
	return lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box.Render(ui))
}
