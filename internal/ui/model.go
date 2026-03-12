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
	focusMain    // Top Pane: Tabs + Actions + Viewport
	focusContent // Bottom Pane: Viewport
)

type focusSubArea int

const (
	subAreaList focusSubArea = iota
	subAreaActions
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
	Tab      key.Binding
	ShiftTab key.Binding
	Delete   key.Binding
	PrevTab  key.Binding
	NextTab  key.Binding
}

var keys = keyMap{
	Up:    key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:  key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:  key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
	Right: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "enter/action")),
	Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "exit/back")),
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Help:  key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Tab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next focus")),
	ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev focus")),
	Delete:  key.NewBinding(key.WithKeys("x", "delete"), key.WithHelp("x", "delete")),
	PrevTab: key.NewBinding(key.WithKeys("["), key.WithHelp("[", "prev tab")),
	NextTab: key.NewBinding(key.WithKeys("]"), key.WithHelp("]", "next tab")),
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Tab, k.PrevTab, k.NextTab, k.Enter, k.Back, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.ShiftTab, k.PrevTab, k.NextTab},
		{k.Enter, k.Back, k.Quit},
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
	isEntered   bool
	subFocus    focusSubArea
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
	graphLines    []string
	
	// Stage view
	statusItems []git.StatusItem
	statusIdx   int

	// Action Bar
	actionIdx int

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
		focus:       focusMain,
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
		if key.Matches(msg, keys.Quit) && m.state != dialogView && !m.isEntered {
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
			if !m.isEntered {
				// NAVIGATION MODE
				if key.Matches(msg, keys.Tab) {
					m.focus = (m.focus + 1) % 3
					return m, nil
				}
				if key.Matches(msg, keys.ShiftTab) {
					m.focus = (m.focus + 2) % 3
					return m, nil
				}
				if key.Matches(msg, keys.Enter) {
					m.isEntered = true
					m.subFocus = subAreaList
					return m, nil
				}
				if key.Matches(msg, keys.Back) {
					m.state = welcomeView
					return m, nil
				}
			} else {
				// INTERACTION MODE (ACTIVE BUBBLE)
				if key.Matches(msg, keys.Back) {
					m.isEntered = false
					return m, nil
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

				case focusMain:
					actions := m.getActionsForTab()
					
					if m.subFocus == subAreaActions {
						switch {
						case key.Matches(msg, keys.Left):
							if m.actionIdx > 0 { m.actionIdx-- }
						case key.Matches(msg, keys.Right):
							if m.actionIdx < len(actions)-1 { m.actionIdx++ }
						case key.Matches(msg, keys.Down):
							m.subFocus = subAreaList
						case key.Matches(msg, keys.Enter):
							if len(actions) > 0 {
								m.executeAction(actions[m.actionIdx])
							}
						}
					} else {
						// subAreaList
						switch {
						case key.Matches(msg, keys.Up):
							if m.activeTab == tabGraph && m.commitIdx == 0 { m.subFocus = subAreaActions; return m, nil }
							if m.activeTab == tabStage && m.statusIdx == 0 { m.subFocus = subAreaActions; return m, nil }
							
							switch m.activeTab {
							case tabGraph:
								if m.commitIdx > 0 { m.commitIdx--; m.refreshTabContent(); m.updateDiffFromCommit() }
							case tabStage:
								if m.statusIdx > 0 { m.statusIdx--; m.refreshTabContent(); m.updateDiffFromStatus() }
							}
						case key.Matches(msg, keys.Down):
							switch m.activeTab {
							case tabGraph:
								if m.commitIdx < len(m.graphLines)-1 { m.commitIdx++; m.refreshTabContent(); m.updateDiffFromCommit() }
							case tabStage:
								if m.statusIdx < len(m.statusItems)-1 { m.statusIdx++; m.refreshTabContent(); m.updateDiffFromStatus() }
							}
						case key.Matches(msg, keys.Enter):
							switch m.activeTab {
							case tabStage:
								if len(m.statusItems) > 0 {
									item := m.statusItems[m.statusIdx]
									var err error
									if item.Staged { err = m.currentRepo.UnstageFile(item.Path) } else { err = m.currentRepo.StageFile(item.Path) }
									if err == nil { m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
								}
							case tabGraph:
								m.updateDiffFromCommit()
							}
						case key.Matches(msg, keys.Left):
							if m.activeTab > 0 {
								m.activeTab--
								m.actionIdx = 0
								m.commitIdx = 0
								m.statusIdx = 0
								m.refreshTabContent()
							}
						case key.Matches(msg, keys.Right):
							if m.activeTab < 6 {
								m.activeTab++
								m.actionIdx = 0
								m.commitIdx = 0
								m.statusIdx = 0
								m.refreshTabContent()
							}
						}
						
						// Viewport updates only for relevant tabs and NOT for keys we handled
						if m.activeTab != tabGraph && m.activeTab != tabStage {
							m.logViewport, cmd = m.logViewport.Update(msg)
							cmds = append(cmds, cmd)
						}
					}

				case focusContent:
					m.contentViewport, cmd = m.contentViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
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

func (m *Model) getActionsForTab() []string {
	switch m.activeTab {
	case tabGraph:
		return []string{"FETCH", "PULL", "PUSH", "CHERRY-PICK", "REFRESH"}
	case tabStage:
		return []string{"COMMIT", "AMEND", "STAGE ALL", "UNSTAGE ALL", "REFRESH"}
	case tabBranches:
		return []string{"NEW BRANCH", "REFRESH"}
	case tabTags:
		return []string{"NEW TAG", "REFRESH"}
	case tabRemotes:
		return []string{"REFRESH"}
	default:
		return []string{"REFRESH"}
	}
}

func (m *Model) executeAction(action string) {
	switch action {
	case "FETCH":
		m.setStatus("Fetching...", false)
		go func() { _ = m.currentRepo.Fetch() }()
	case "PULL":
		m.setStatus("Pulling...", false)
		err := m.currentRepo.Pull()
		if err != nil { m.setStatus("Error: "+err.Error(), true) } else { m.setStatus("Pulled.", false); m.refreshRepoData() }
	case "PUSH":
		m.setStatus("Pushing...", false)
		err := m.currentRepo.Push()
		if err != nil { m.setStatus("Error: "+err.Error(), true) } else { m.setStatus("Pushed.", false) }
	case "COMMIT":
		m.openDialog(dialogCommit, "Commit message...")
	case "AMEND":
		m.openDialog(dialogAmend, "Amend message (leave empty to keep current)...")
	case "STAGE ALL":
		err := m.currentRepo.StageAll()
		if err == nil { m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
	case "UNSTAGE ALL":
		err := m.currentRepo.UnstageAll()
		if err == nil { m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
	case "CHERRY-PICK":
		if len(m.commits) > 0 {
			hash := strings.Split(m.commits[m.commitIdx], " ")[0]
			m.openDialog(dialogCherryPick, "Cherry-pick " + hash + "? (enter to confirm)")
			m.textInput.SetValue(hash)
		}
	case "NEW BRANCH":
		m.openDialog(dialogCreateBranch, "New branch name...")
	case "NEW TAG":
		m.openDialog(dialogCreateTag, "New tag name...")
	case "REFRESH":
		m.refreshRepoData()
		m.setStatus("Refreshed.", false)
	}
}

func (m *Model) updateSizes() {
	m.repoList.SetSize(m.width-4, m.height-6)
	
	sidebarWidth := 30
	mainWidth := m.width - sidebarWidth
	headerHeight := 2
	footerHeight := 2
	contentHeight := m.height - headerHeight - footerHeight - 2
	
	if contentHeight < 10 { contentHeight = 10 }
	
	logHeight := contentHeight / 2
	viewHeight := contentHeight - logHeight
	
	m.sidebarList.SetSize(sidebarWidth-2, contentHeight-2)
	
	m.logViewport.Width = mainWidth - 4
	m.logViewport.Height = logHeight - 6 
	if m.logViewport.Height < 1 { m.logViewport.Height = 1 }
	
	m.contentViewport.Width = mainWidth - 4
	m.contentViewport.Height = viewHeight - 2
	if m.contentViewport.Height < 1 { m.contentViewport.Height = 1 }
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
	if v1 == "" && m.activeDlg != dialogCommit && m.activeDlg != dialogAmend { return }

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
	case dialogCreateBranch:
		err = m.currentRepo.CreateBranch(v1)
		if err == nil {
			m.setStatus("Branch created: "+v1, false)
		} else { m.setStatus("Create failed: "+err.Error(), true) }
		m.state = mainView
		m.refreshRepoData()
	case dialogCreateTag:
		err = m.currentRepo.CreateTag(v1)
		if err == nil {
			m.setStatus("Tag created: "+v1, false)
		} else { m.setStatus("Create failed: "+err.Error(), true) }
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
	
	g, _ := m.currentRepo.GetGraph(100)
	m.graphLines = strings.Split(strings.TrimSpace(g), "\n")
	
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
		m.renderGraphView()
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
			"• Sidebar (Left): Branches, tags, remotes.\n" +
			"• Main Pane (Top Right): Active tab content and Action Bar.\n" +
			"• Bottom Pane (Bottom Right): Detailed diff view.\n\n" +
			SelectedStyle.Render("NAVIGATION") + "\n" +
			"• Tab / Shift+Tab: Cycle focus between bubbles (Pink border).\n" +
			"• Enter: Enter the focused bubble (Green border).\n" +
			"• Esc: Exit active bubble back to navigation mode.\n\n" +
			SelectedStyle.Render("INSIDE MAIN PANE") + "\n" +
			"• Left / Right: Switch between tabs (Log, Stage, etc.).\n" +
			"• Arrows: Navigate lists or scroll content.\n" +
			"• Up (from top): Reach the Action Bar buttons.\n" +
			"• Enter: Stage/Unstage (Stage tab) or Select Commit (Log tab)."
		m.logViewport.SetContent(helpText)
	}
}

func (m *Model) renderGraphView() {
	var sb strings.Builder
	for i, line := range m.graphLines {
		if i == m.commitIdx {
			sb.WriteString(SelectedStyle.Render("> " + line) + "\n")
		} else {
			sb.WriteString("  " + line + "\n")
		}
	}
	m.logViewport.SetContent(sb.String())
	
	// Sync viewport scroll
	if m.commitIdx < m.logViewport.YOffset {
		m.logViewport.YOffset = m.commitIdx
	} else if m.commitIdx >= m.logViewport.YOffset+m.logViewport.Height {
		m.logViewport.YOffset = m.commitIdx - m.logViewport.Height + 1
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
	
	// Sync viewport scroll
	if m.statusIdx < m.logViewport.YOffset {
		m.logViewport.YOffset = m.statusIdx
	} else if m.statusIdx >= m.logViewport.YOffset+m.logViewport.Height {
		m.logViewport.YOffset = m.statusIdx - m.logViewport.Height + 1
	}
}

func (m *Model) updateDiffFromCommit() {
	if len(m.commits) > 0 && m.commitIdx < len(m.commits) {
		// Attempt to extract hash from either m.commits or m.graphLines
		// graphLines often starts with * hash msg
		line := ""
		if m.commitIdx < len(m.commits) {
			line = m.commits[m.commitIdx]
		}
		
		parts := strings.Split(strings.TrimSpace(line), " ")
		if len(parts) > 0 {
			hash := parts[0]
			diff, err := m.currentRepo.GetCommitDiff(hash)
			if err == nil {
				m.contentViewport.SetContent(diff)
				m.contentViewport.GotoTop()
			}
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
			m.contentViewport.GotoTop()
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
		if m.isEntered {
			sbStyle = sbStyle.BorderForeground(Green)
		} else {
			sbStyle = sbStyle.BorderForeground(Pink)
		}
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

	// Action Bar
	actionItems := m.getActionsForTab()
	actionBar := ""
	for i, a := range actionItems {
		style := InactiveStyle.Copy().Padding(0, 1).Background(DarkGray)
		if m.focus == focusMain && m.isEntered && m.subFocus == subAreaActions && i == m.actionIdx {
			style = SelectedStyle.Copy().Padding(0, 1).Background(Magenta).Foreground(White)
		}
		actionBar += style.Render(a) + " "
	}

	// Log/Graph area (Main Pane)
	logStyle := MainBoxStyle.Copy().Width(mainWidth).Height(logHeight)
	if m.focus == focusMain {
		if m.isEntered {
			logStyle = logStyle.BorderForeground(Green)
		} else {
			logStyle = logStyle.BorderForeground(Pink)
		}
	}
	
	logArea := logStyle.Render(lipgloss.JoinVertical(lipgloss.Left, 
		tabHeader, 
		"", // spacer
		actionBar, 
		"", // spacer
		m.logViewport.View(),
	))

	// Content/Diff area
	contentStyle := MainBoxStyle.Copy().Width(mainWidth).Height(viewHeight)
	if m.focus == focusContent {
		if m.isEntered {
			contentStyle = contentStyle.BorderForeground(Green)
		} else {
			contentStyle = contentStyle.BorderForeground(Pink)
		}
	}
	contentArea := contentStyle.Render(m.contentViewport.View())

	mainViewUI := lipgloss.JoinVertical(lipgloss.Left, logArea, contentArea)

	return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainViewUI)
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
