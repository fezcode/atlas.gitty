package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"atlas.gitty/internal/config"
	"atlas.gitty/internal/git"
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
	tabRepo
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
	dialogAddRemote
	dialogDeleteBranch
	dialogDeleteTag
	dialogDeleteRemote
	dialogUnlistRepo
	dialogNukeRepo
	dialogMerge
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Space   key.Binding
	Back    key.Binding
	Quit    key.Binding
	Help    key.Binding
	Tab     key.Binding
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
	Space: key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "diff")),
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
	return []key.Binding{k.Tab, k.PrevTab, k.NextTab, k.Enter, k.Space, k.Back, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Tab, k.ShiftTab, k.PrevTab, k.NextTab},
		{k.Enter, k.Space, k.Back, k.Quit},
	}
}

type item struct {
	title, desc string
}

type repoDataUpdatedMsg struct {
	status string
	isErr  bool
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

	// List views for other tabs
	branchList  []string
	branchIdx   int
	tagList     []string
	tagIdx      int
	remoteList  []string
	remoteIdx   int

	// Action Bar
	actionIdx int

	// Viewports
	logViewport    viewport.Model
	contentViewport viewport.Model

	// Dialog
	textInput  textinput.Model
	textInput2 textinput.Model 
	isMultiInput bool
	dialogCheckbox bool

	// UI Helpers
	help    help.Model
	showHelp bool
	lastMsg string
	isError bool
	cwd     string
}

func NewInitialModel() Model {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	
	dir, _ := os.Getwd()

	items := []list.Item{}
	for _, r := range cfg.Repositories {
		items = append(items, item{title: r.Path, desc: r.Path})
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
		cwd:         dir,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case repoDataUpdatedMsg:
		m.setStatus(msg.status, msg.isErr)
		m.refreshRepoData()
		return m, nil

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
				case 2: m.openMultiDialog(dialogCloneRepo, "Repository URL...", "Folder name or path...")
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
			if key.Matches(msg, keys.PrevTab) {
				m.activeTab = (m.activeTab + 7) % 8
				m.resetIdx(); m.refreshTabContent(); return m, nil
			}
			if key.Matches(msg, keys.NextTab) {
				m.activeTab = (m.activeTab + 1) % 8
				m.resetIdx(); m.refreshTabContent(); return m, nil
			}

			if !m.isEntered {
				if key.Matches(msg, keys.Tab) { m.focus = (m.focus + 1) % 3; return m, nil }
				if key.Matches(msg, keys.ShiftTab) { m.focus = (m.focus + 2) % 3; return m, nil }
				if key.Matches(msg, keys.Enter) { m.isEntered = true; m.subFocus = subAreaList; return m, nil }
				if key.Matches(msg, keys.Back) { m.state = welcomeView; return m, nil }
			} else {
				if key.Matches(msg, keys.Back) { m.isEntered = false; return m, nil }

				switch m.focus {
				case focusSidebar:
					m.sidebarList, cmd = m.sidebarList.Update(msg)
					cmds = append(cmds, cmd)
					if key.Matches(msg, keys.Enter) {
						if i := m.sidebarList.SelectedItem(); i != nil {
							title := i.(item).title
							if strings.HasPrefix(title, "B: ") {
								branch := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(title, "B: "), "* "))
								err := m.currentRepo.Checkout(branch)
								if err == nil { m.setStatus("Checked out "+branch, false); m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
							}
						}
					}

				case focusMain:
					actions := m.getActionsForTab()
					if m.subFocus == subAreaActions {
						switch {
						case key.Matches(msg, keys.Left): if m.actionIdx > 0 { m.actionIdx-- }
						case key.Matches(msg, keys.Right): if m.actionIdx < len(actions)-1 { m.actionIdx++ }
						case key.Matches(msg, keys.Down): m.subFocus = subAreaList
						case key.Matches(msg, keys.Enter):
							if len(actions) > 0 { return m, m.executeAction(actions[m.actionIdx]) }
						}
					} else {
						switch {
						case key.Matches(msg, keys.Up):
							if m.isAtTop() { m.subFocus = subAreaActions; return m, nil }
							m.moveUp()
						case key.Matches(msg, keys.Down):
							m.moveDown()
						case key.Matches(msg, keys.Enter):
							m.handleListEnter()
						case key.Matches(msg, keys.Space):
							if m.activeTab == tabStage {
								m.updateDiffFromStatus()
							} else if m.activeTab == tabGraph {
								m.updateDiffFromCommit()
							}
						}
						if m.activeTab != tabGraph && m.activeTab != tabStage && m.activeTab != tabBranches && m.activeTab != tabTags && m.activeTab != tabRemotes {
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
			if msg.String() == "ctrl+d" && m.activeDlg == dialogMerge {
				m.dialogCheckbox = !m.dialogCheckbox
				return m, nil
			}
			if msg.String() == "esc" { m.state = mainView; if m.currentRepo == nil { m.state = welcomeView } } else if msg.String() == "enter" { m.handleDialogSubmit() } else if msg.String() == "tab" && m.isMultiInput {
				if m.textInput.Focused() { m.textInput.Blur(); m.textInput2.Focus() } else { m.textInput2.Blur(); m.textInput.Focus() }
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

func (m *Model) isAtTop() bool {
	switch m.activeTab {
	case tabGraph: return m.commitIdx == 0
	case tabStage: return m.statusIdx == 0
	case tabBranches: return m.branchIdx == 0
	case tabTags: return m.tagIdx == 0
	case tabRemotes: return m.remoteIdx == 0
	}
	return true
}

func (m *Model) moveUp() {
	switch m.activeTab {
	case tabGraph: if m.commitIdx > 0 { m.commitIdx--; m.refreshTabContent() }
	case tabStage: if m.statusIdx > 0 { m.statusIdx--; m.refreshTabContent() }
	case tabBranches: if m.branchIdx > 0 { m.branchIdx--; m.refreshTabContent() }
	case tabTags: if m.tagIdx > 0 { m.tagIdx--; m.refreshTabContent() }
	case tabRemotes: if m.remoteIdx > 0 { m.remoteIdx--; m.refreshTabContent() }
	}
}

func (m *Model) moveDown() {
	switch m.activeTab {
	case tabGraph: if m.commitIdx < len(m.graphLines)-1 { m.commitIdx++; m.refreshTabContent() }
	case tabStage: if m.statusIdx < len(m.statusItems)-1 { m.statusIdx++; m.refreshTabContent() }
	case tabBranches: if m.branchIdx < len(m.branchList)-1 { m.branchIdx++; m.refreshTabContent() }
	case tabTags: if m.tagIdx < len(m.tagList)-1 { m.tagIdx++; m.refreshTabContent() }
	case tabRemotes: if m.remoteIdx < len(m.remoteList)-1 { m.remoteIdx++; m.refreshTabContent() }
	}
}

func (m *Model) handleListEnter() {
	switch m.activeTab {
	case tabStage:
		if len(m.statusItems) > 0 {
			item := m.statusItems[m.statusIdx]; var err error
			if item.Staged { err = m.currentRepo.UnstageFile(item.Path) } else { err = m.currentRepo.StageFile(item.Path) }
			if err == nil { m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
		}
	case tabGraph: m.updateDiffFromCommit()
	case tabBranches:
		if len(m.branchList) > 0 {
			branch := strings.TrimSpace(strings.TrimPrefix(m.branchList[m.branchIdx], "* "))
			err := m.currentRepo.Checkout(branch)
			if err == nil { m.setStatus("Checked out "+branch, false); m.refreshRepoData() } else { m.setStatus("Error: "+err.Error(), true) }
		}
	}
}

func (m *Model) resetIdx() {
	m.actionIdx = 0; m.commitIdx = 0; m.statusIdx = 0; m.branchIdx = 0; m.tagIdx = 0; m.remoteIdx = 0
}

func (m *Model) getActionsForTab() []string {
	switch m.activeTab {
	case tabGraph: return []string{"FETCH", "PULL", "PUSH", "CHERRY-PICK", "REFRESH"}
	case tabStage: return []string{"COMMIT", "AMEND", "STAGE ALL", "UNSTAGE ALL", "REFRESH"}
	case tabBranches: return []string{"NEW BRANCH", "DELETE BRANCH", "MERGE", "REFRESH"}
	case tabTags: return []string{"NEW TAG", "DELETE TAG", "PUSH TAGS", "REFRESH"}
	case tabRemotes: return []string{"ADD REMOTE", "DELETE REMOTE", "REFRESH"}
	case tabRepo: return []string{"OPEN IN EXPLORER", "UNLIST REPO", "NUKE REPO", "REFRESH"}
	default: return []string{"REFRESH"}
	}
}

func (m *Model) doStageAll() tea.Cmd {
	return func() tea.Msg {
		err := m.currentRepo.StageAll()
		if err == nil { return repoDataUpdatedMsg{status: "All files staged.", isErr: false} }
		return repoDataUpdatedMsg{status: "Error: " + err.Error(), isErr: true}
	}
}

func (m *Model) doUnstageAll() tea.Cmd {
	return func() tea.Msg {
		err := m.currentRepo.UnstageAll()
		if err == nil { return repoDataUpdatedMsg{status: "All files unstaged.", isErr: false} }
		return repoDataUpdatedMsg{status: "Error: " + err.Error(), isErr: true}
	}
}

func (m *Model) executeAction(action string) tea.Cmd {
	switch action {
	case "FETCH":
		m.setStatus("Fetching...", false)
		return func() tea.Msg {
			viaCli, err := m.currentRepo.Fetch()
			status := "Fetched."
			if err != nil { return repoDataUpdatedMsg{status: "Error: " + err.Error(), isErr: true} }
			if viaCli { status = "Fetched (via CLI)." }
			return repoDataUpdatedMsg{status: status, isErr: false}
		}
	case "PULL":
		m.setStatus("Pulling...", false)
		return func() tea.Msg {
			viaCli, err := m.currentRepo.Pull()
			status := "Pulled."
			if err != nil { return repoDataUpdatedMsg{status: "Error: " + err.Error(), isErr: true} }
			if viaCli { status = "Pulled (via CLI)." }
			return repoDataUpdatedMsg{status: status, isErr: false}
		}
	case "PUSH":
		m.setStatus("Pushing...", false)
		return func() tea.Msg {
			viaCli, err := m.currentRepo.Push()
			status := "Pushed."
			if err != nil { return repoDataUpdatedMsg{status: "Error: " + err.Error(), isErr: true} }
			if viaCli { status = "Pushed (via CLI)." }
			return repoDataUpdatedMsg{status: status, isErr: false}
		}
	case "COMMIT": m.openDialog(dialogCommit, "Commit message...")
	case "AMEND": m.openDialog(dialogAmend, "Amend message (leave empty to keep current)...")
	case "STAGE ALL":
		m.setStatus("Staging all files...", false)
		return m.doStageAll()
	case "UNSTAGE ALL":
		m.setStatus("Unstaging all files...", false)
		return m.doUnstageAll()
	case "CHERRY-PICK":
		if len(m.commits) > 0 {
			hash := strings.Split(m.commits[m.commitIdx], " ")[0]
			m.openDialog(dialogCherryPick, "Cherry-pick " + hash + "? (enter to confirm)"); m.textInput.SetValue(hash)
		}
	case "NEW BRANCH": m.openDialog(dialogCreateBranch, "New branch name...")
	case "DELETE BRANCH": if len(m.branchList) > 0 { branch := strings.TrimSpace(strings.TrimPrefix(m.branchList[m.branchIdx], "* ")); m.openDialog(dialogDeleteBranch, "Type branch name to DELETE: "+branch) }
	case "MERGE":
		if len(m.branchList) > 0 {
			branch := strings.TrimSpace(strings.TrimPrefix(m.branchList[m.branchIdx], "* "))
			m.openMultiDialog(dialogMerge, "Merge From (Source Branch)", "Merge To (Target Branch)")
			m.textInput.SetValue(branch)
			m.textInput2.SetValue(m.currentBranch)
			m.dialogCheckbox = false
		}
	case "NEW TAG": m.openDialog(dialogCreateTag, "New tag name...")
	case "DELETE TAG": if len(m.tagList) > 0 { tag := m.tagList[m.tagIdx]; m.openDialog(dialogDeleteTag, "Type tag name to DELETE: "+tag) }
	case "PUSH TAGS":
		m.setStatus("Pushing tags...", false)
		return func() tea.Msg {
			viaCli, err := m.currentRepo.PushTags()
			status := "Tags pushed."
			if err != nil { return repoDataUpdatedMsg{status: "Error: " + err.Error(), isErr: true} }
			if viaCli { status = "Tags pushed (via CLI)." }
			return repoDataUpdatedMsg{status: status, isErr: false}
		}
	case "ADD REMOTE": m.openMultiDialog(dialogAddRemote, "Remote Name (e.g. origin)", "Remote URL")
	case "DELETE REMOTE": if len(m.remoteList) > 0 { name := strings.Split(m.remoteList[m.remoteIdx], " ")[0]; m.openDialog(dialogDeleteRemote, "Type remote name to DELETE: "+name) }
	case "OPEN IN EXPLORER": openExplorer(m.currentRepo.Path)
	case "UNLIST REPO": m.openDialog(dialogUnlistRepo, "Type 'YES' to remove repo from list (NOT disk)")
	case "NUKE REPO": m.openDialog(dialogNukeRepo, "Type 'YES' to DELETE repo from DISK forever!")
	case "REFRESH": m.refreshRepoData(); m.setStatus("Refreshed.", false)
	}
	return nil
}

func openExplorer(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows": cmd = exec.Command("explorer", path)
	case "darwin": cmd = exec.Command("open", path)
	default: cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

func (m *Model) handleDialogSubmit() {
	v1 := m.textInput.Value(); v2 := m.textInput2.Value()
	if v1 == "" && m.activeDlg != dialogCommit && m.activeDlg != dialogAmend { return }
	var err error
	switch m.activeDlg {
	case dialogAddRepo:
		absPath, errAbs := filepath.Abs(v1)
		if errAbs != nil { m.setStatus("Invalid path: "+errAbs.Error(), true); return }
		repo, errOpen := git.OpenRepo(absPath)
		if errOpen == nil {
			m.cfg.AddRepository(repo.Path)
			_ = m.cfg.Save()
			m.updateRepoList()
			m.setStatus("Repo added: "+repo.Path, false)
		} else { m.setStatus("Invalid git repo: "+errOpen.Error(), true) }
		m.state = welcomeView
		return
	case dialogCloneRepo:
		dest := v2
		if !filepath.IsAbs(dest) { cwd, _ := filepath.Abs("."); dest = filepath.Join(cwd, v2) }
		absDest, _ := filepath.Abs(dest)
		m.setStatus("Cloning to "+absDest+"...", false)
		viaCli, err := git.CloneRepo(v1, absDest)
		if err == nil {
			m.cfg.AddRepository(absDest); _ = m.cfg.Save(); m.updateRepoList()
			msg := "Cloned to "+absDest
			if viaCli { msg = "Cloned (via CLI) to "+absDest }
			m.setStatus(msg, false)
		} else { m.setStatus("Clone failed: "+err.Error(), true) }
		m.state = welcomeView; return
	case dialogInitRepo:
		absPath, _ := filepath.Abs(v1)
		err = git.InitRepo(absPath)
		if err == nil {
			m.cfg.AddRepository(absPath); _ = m.cfg.Save(); m.updateRepoList()
			m.setStatus("Initialized at "+absPath, false)
		} else { m.setStatus("Init failed: "+err.Error(), true) }
		m.state = welcomeView; return
	case dialogCommit: err = m.currentRepo.Commit(v1); if err == nil { m.setStatus("Committed.", false) } else { m.setStatus("Commit failed: "+err.Error(), true) }
	case dialogAmend: err = m.currentRepo.Amend(v1); if err == nil { m.setStatus("Amended.", false) } else { m.setStatus("Amend failed: "+err.Error(), true) }
	case dialogCherryPick: err = m.currentRepo.CherryPick(v1); if err == nil { m.setStatus("Cherry-picked.", false) } else { m.setStatus("Cherry-pick failed: "+err.Error(), true) }
	case dialogCheckout: err = m.currentRepo.Checkout(v1); if err == nil { m.setStatus("Switched to "+v1, false) } else { m.setStatus("Switch failed: "+err.Error(), true) }
	case dialogCreateBranch: err = m.currentRepo.CreateBranch(v1); if err == nil { m.setStatus("Branch created: "+v1, false) } else { m.setStatus("Create failed: "+err.Error(), true) }
	case dialogDeleteBranch: err = m.currentRepo.DeleteBranch(v1); if err == nil { m.setStatus("Branch deleted: "+v1, false) } else { m.setStatus("Delete failed: "+err.Error(), true) }
	case dialogCreateTag: err = m.currentRepo.CreateTag(v1); if err == nil { m.setStatus("Tag created: "+v1, false) } else { m.setStatus("Create failed: "+err.Error(), true) }
	case dialogDeleteTag: err = m.currentRepo.DeleteTag(v1); if err == nil { m.setStatus("Tag deleted: "+v1, false) } else { m.setStatus("Delete failed: "+err.Error(), true) }
	case dialogAddRemote: err = m.currentRepo.CreateRemote(v1, v2); if err == nil { m.setStatus("Remote added: "+v1, false) } else { m.setStatus("Add failed: "+err.Error(), true) }
	case dialogDeleteRemote: err = m.currentRepo.DeleteRemote(v1); if err == nil { m.setStatus("Remote deleted: "+v1, false) } else { m.setStatus("Delete failed: "+err.Error(), true) }
	case dialogMerge:
		// v1 is From (Source), v2 is To (Target)
		m.setStatus(fmt.Sprintf("Merging %s into %s...", v1, v2), false)
		err = m.currentRepo.Checkout(v2)
		if err == nil {
			err = m.currentRepo.Merge(v1)
			if err == nil {
				m.setStatus(fmt.Sprintf("Successfully merged %s into %s.", v1, v2), false)
				if m.dialogCheckbox {
					errDel := m.currentRepo.DeleteBranch(v1)
					if errDel == nil {
						m.setStatus(fmt.Sprintf("Merged and deleted branch %s.", v1), false)
					} else {
						m.setStatus(fmt.Sprintf("Merged, but failed to delete branch %s: %v", v1, errDel), true)
					}
				}
			} else { m.setStatus("Merge failed: "+err.Error(), true) }
		} else { m.setStatus("Checkout failed: "+err.Error(), true) }
	case dialogUnlistRepo:
		if strings.ToUpper(v1) == "YES" {
			m.cfg.RemoveRepository(m.currentRepo.Path); _ = m.cfg.Save(); m.updateRepoList()
			m.currentRepo = nil; m.state = welcomeView; m.setStatus("Repo unlisted.", false); return
		}
	case dialogNukeRepo:
		if strings.ToUpper(v1) == "YES" {
			path := m.currentRepo.Path
			m.cfg.RemoveRepository(path); _ = m.cfg.Save(); m.updateRepoList()
			err = os.RemoveAll(path)
			m.currentRepo = nil; m.state = welcomeView
			if err == nil { m.setStatus("Repo NUKED from disk.", false) } else { m.setStatus("Nuke error: "+err.Error(), true) }
			return
		}
	}
	if err != nil { m.setStatus("Error: "+err.Error(), true) }
	m.state = mainView; m.refreshRepoData()
}

func (m *Model) updateSizes() {
	m.repoList.SetSize(m.width-4, m.height-6)
	
	sidebarWidth := 26
	mainWidth := m.width - sidebarWidth - 6 // Extra margin for right border
	if mainWidth < 10 { mainWidth = 10 }
	
	contentHeight := m.height - 8 // Extra margin for top/bottom
	if contentHeight < 10 { contentHeight = 10 }
	
	logHeight := contentHeight / 2
	viewHeight := contentHeight - logHeight
	
	m.sidebarList.SetSize(sidebarWidth-4, contentHeight-2)
	
	m.logViewport.Width = mainWidth - 4
	m.logViewport.Height = logHeight - 4
	if m.logViewport.Height < 1 { m.logViewport.Height = 1 }
	
	m.contentViewport.Width = mainWidth - 4
	m.contentViewport.Height = viewHeight - 2
	if m.contentViewport.Height < 1 { m.contentViewport.Height = 1 }
}

func (m *Model) openDialog(t dialogType, placeholder string) {
	m.activeDlg = t; m.state = dialogView; m.isMultiInput = false; m.textInput.Placeholder = placeholder; m.textInput.SetValue(""); m.textInput.Focus()
}

func (m *Model) openMultiDialog(t dialogType, p1, p2 string) {
	m.activeDlg = t; m.state = dialogView; m.isMultiInput = true; m.textInput.Placeholder = p1; m.textInput.SetValue(""); m.textInput.Focus(); m.textInput2.Placeholder = p2; m.textInput2.SetValue("")
}

func (m *Model) setStatus(msg string, isErr bool) {
	m.isError = isErr; m.lastMsg = fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), msg)
}

func (m *Model) updateRepoList() {
	items := []list.Item{}
	for _, r := range m.cfg.Repositories { items = append(items, item{title: r.Path, desc: r.Path}) }
	m.repoList.SetItems(items)
}

func (m *Model) refreshRepoData() {
	if m.currentRepo == nil { return }
	m.currentBranch, _ = m.currentRepo.GetCurrentBranch()
	m.commits, _ = m.currentRepo.GetCommits(100)
	m.statusItems, _ = m.currentRepo.GetStatusItems()
	m.branchList, _ = m.currentRepo.GetBranches()
	m.tagList, _ = m.currentRepo.GetTags()
	m.remoteList, _ = m.currentRepo.GetRemotes()
	g, _ := m.currentRepo.GetGraph(100); m.graphLines = strings.Split(strings.TrimSpace(g), "\n")
	var items []list.Item
	for _, b := range m.branchList { prefix := "  "; if b == m.currentBranch { prefix = "* " }; items = append(items, item{title: "B: " + prefix + b}) }
	for _, t := range m.tagList { items = append(items, item{title: "T: " + t}) }
	for _, r := range m.remoteList { items = append(items, item{title: "R: " + r}) }
	m.sidebarList.SetItems(items); m.refreshTabContent()
	if m.activeTab == tabStage { m.updateDiffFromStatus() } else { m.updateDiffFromCommit() }
}

func (m *Model) refreshTabContent() {
	if m.currentRepo == nil { return }
	switch m.activeTab {
	case tabGraph: m.renderGraphView()
	case tabStage: m.renderStageView()
	case tabBranches: m.renderListView("BRANCHES", m.branchList, &m.branchIdx)
	case tabTags: m.renderListView("TAGS", m.tagList, &m.tagIdx)
	case tabRemotes: m.renderListView("REMOTES", m.remoteList, &m.remoteIdx)
	case tabDiff: d, _ := m.currentRepo.GetDiff(""); m.logViewport.SetContent(d)
	case tabHelp:
		helpText := HeaderStyle.Render("ATLAS.GITTY USAGE GUIDE") + "\n\n" +
			SelectedStyle.Render("LAYOUT") + "\n" +
			"• Sidebar (Left): Repository navigation (Branches, Tags, Remotes).\n" +
			"• Main Pane (Top Right): Active tab content and Action Bar.\n" +
			"• Bottom Pane (Bottom Right): Detailed diff view.\n\n" +
			SelectedStyle.Render("NAVIGATION") + "\n" +
			"• Tab / Shift+Tab: Cycle focus between bubbles (Pink border = hovered).\n" +
			"• Enter: ENTER a focused bubble (Green border = active interaction).\n" +
			"• Esc: EXIT active bubble back to navigation mode.\n" +
			"• [ / ]: Switch tabs globally.\n\n" +
			SelectedStyle.Render("INTERACTIONS") + "\n" +
			"• Main Pane (Active): Use Arrows to scroll lists. Press UP from top to reach Action Bar.\n" +
			"• Sidebar (Active): Use Arrows to select, Enter to checkout branch.\n" +
			"• Action Bar: Use Left/Right to select a button, Enter to execute.\n" +
			"• Merge Dialog: Use Ctrl+D to toggle source branch deletion after merge.\n\n" +
			SelectedStyle.Render("AUTHENTICATION & FALLBACK") + "\n" +
			"• PUSH/PULL/FETCH operations first attempt using internal library.\n" +
			"• If authentication fails (e.g. SSH/HTTPS creds), it falls back to system 'git' CLI.\n" +
			"• This allows leveraging your system's Git credential helpers automatically.\n\n" +
			SelectedStyle.Render("TABS") + "\n" +
			"• LOG: Browse commits. Press Space on commit to see diff below.\n" +
			"• STAGE: Enter to toggle Stage/Unstage. Press Space to see diff. Top Action Bar has COMMIT.\n" +
			"• BRANCHES/TAGS/REMOTES: Browse and manage items using Action Bar."
		m.logViewport.SetContent(helpText)
	case tabRepo:
		m.logViewport.SetContent(HeaderStyle.Render("REPOSITORY SETTINGS") + "\n\n" +
			"Path: " + m.currentRepo.Path + "\n\n" +
			"Use the Action Bar above to open folder, unlist, or NUKE the repo.")
	}
}

func (m *Model) renderListView(title string, list []string, idx *int) {
	var sb strings.Builder; sb.WriteString(HeaderStyle.Render(title) + "\n\n")
	if len(list) == 0 { sb.WriteString(InactiveStyle.Render("  None found.")) } else {
		for i, item := range list { prefix := "  "; if i == *idx { prefix = "> " }; line := prefix + item; if i == *idx { sb.WriteString(SelectedStyle.Render(line) + "\n") } else { sb.WriteString(line + "\n") } }
	}
	m.logViewport.SetContent(sb.String())
	targetLine := *idx + 2
	if targetLine < m.logViewport.YOffset { m.logViewport.YOffset = targetLine } else if targetLine >= m.logViewport.YOffset+m.logViewport.Height { m.logViewport.YOffset = targetLine - m.logViewport.Height + 1 }
}

func (m *Model) renderGraphView() {
	var sb strings.Builder
	for i, line := range m.graphLines { if i == m.commitIdx { sb.WriteString(SelectedStyle.Render("> " + line) + "\n") } else { sb.WriteString("  " + line + "\n") } }
	m.logViewport.SetContent(sb.String())
	if m.commitIdx < m.logViewport.YOffset { m.logViewport.YOffset = m.commitIdx } else if m.commitIdx >= m.logViewport.YOffset+m.logViewport.Height { m.logViewport.YOffset = m.commitIdx - m.logViewport.Height + 1 }
}

func (m *Model) renderStageView() {
	count := len(m.statusItems)
	title := fmt.Sprintf("STAGING AREA (%d files)", count)
	var sb strings.Builder; sb.WriteString(HeaderStyle.Render(title) + "\n\n")
	if count == 0 { sb.WriteString(InactiveStyle.Render("  Working tree clean.")) } else {
		for i, item := range m.statusItems { prefix := "  "; if i == m.statusIdx { prefix = "> " }; box := "[ ]"; if item.Staged { box = "[x]" }; line := fmt.Sprintf("%s %s %s (%s)", prefix, box, item.Path, item.Status); if i == m.statusIdx { sb.WriteString(SelectedStyle.Render(line) + "\n") } else if item.Staged { sb.WriteString(SuccessStyle.Render(line) + "\n") } else { sb.WriteString(line + "\n") } }
	}
	m.logViewport.SetContent(sb.String())
	targetLine := m.statusIdx + 2
	if targetLine < m.logViewport.YOffset { m.logViewport.YOffset = targetLine } else if targetLine >= m.logViewport.YOffset+m.logViewport.Height { m.logViewport.YOffset = targetLine - m.logViewport.Height + 1 }
}

func (m *Model) updateDiffFromCommit() {
	if len(m.commits) > 0 && m.commitIdx < len(m.commits) {
		parts := strings.Split(strings.TrimSpace(m.commits[m.commitIdx]), " ")
		if len(parts) > 0 {
			diff, err := m.currentRepo.GetCommitDiff(parts[0])
			if err == nil { m.contentViewport.SetContent(diff); m.contentViewport.GotoTop(); return }
		}
	}
	m.contentViewport.SetContent("")
}

func (m *Model) updateDiffFromStatus() {
	if len(m.statusItems) > 0 && m.statusIdx < len(m.statusItems) {
		diff, err := m.currentRepo.GetDiff(m.statusItems[m.statusIdx].Path)
		if err == nil {
			if diff == "" { diff = "No changes or binary file." }
			m.contentViewport.SetContent(diff); m.contentViewport.GotoTop()
			return
		}
	}
	m.contentViewport.SetContent("")
}

func (m Model) View() string {
	if m.width == 0 || m.height == 0 { return "Initializing..." }
	header := m.renderHeader(); footer := m.renderFooter(); var content string
	switch m.state {
	case welcomeView: content = m.renderWelcome()
	case repoSelectView: content = m.renderRepoSelect()
	case mainView: content = m.renderMain()
	case dialogView: content = m.renderDialog()
	}
	// Strict vertical join with no manual spacing to prevent overlap
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

func (m Model) renderHeader() string {
	repoName := "NO REPOSITORY"; if m.currentRepo != nil { repoName = filepath.Base(m.currentRepo.Path) }
	title := HeaderStyle.Render(" ATLAS.GITTY "); repoInfo := PathStyle.Render(" " + repoName + " "); branchInfo := ""
	if m.currentBranch != "" { branchInfo = SuccessStyle.Render("  " + m.currentBranch + " ") }
	left := lipgloss.JoinHorizontal(lipgloss.Center, title, repoInfo, branchInfo); return HeaderBoxStyle.Width(m.width).Render(left)
}

func (m Model) renderFooter() string {
	var msg string; if m.lastMsg != "" { style := SuccessMessageStyle; if m.isError { style = ErrorMessageStyle }; msg = style.Render(m.lastMsg) }
	helpStr := m.help.View(keys)
	
	// CWD display on the left (truncated)
	maxCwdWidth := m.width / 3
	cwdText := "Run from: " + m.cwd
	if len(cwdText) > maxCwdWidth { cwdText = "..." + cwdText[len(cwdText)-maxCwdWidth+3:] }
	cwdStyle := lipgloss.NewStyle().Foreground(Blue).Italic(true).Padding(0, 1)
	cwdStr := cwdStyle.Render(cwdText)
	
	gap := m.width - lipgloss.Width(cwdStr) - lipgloss.Width(msg) - lipgloss.Width(helpStr) - 2
	if gap < 0 { gap = 0 }
	spacer := strings.Repeat(" ", gap)
	
	return FooterBoxStyle.Width(m.width).Render(
		lipgloss.JoinHorizontal(lipgloss.Top, cwdStr, msg, spacer, helpStr),
	)
}

func (m Model) renderWelcome() string {
	var menu string; for i, item := range m.welcomeMenu { if i == m.welcomeIdx { menu += SelectedStyle.Render("> " + item) + "\n" } else { menu += InactiveStyle.Render("  " + item) + "\n" } }
	box := MainBoxStyle.Copy().Padding(1, 4).BorderForeground(Magenta).Render(lipgloss.JoinVertical(lipgloss.Center, HeaderStyle.Render("ATLAS GIT CLIENT"), "\n", menu)); return lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box)
}

func (m Model) renderRepoSelect() string { box := MainBoxStyle.Copy().Width(m.width - 4).Height(m.height - 8).Render(m.repoList.View()); return lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box) }

func (m Model) renderMain() string {
	sidebarWidth := 26; mainWidth := m.width - sidebarWidth - 6; contentHeight := m.height - 8
	if contentHeight < 10 { contentHeight = 10 }; logHeight := contentHeight / 2; viewHeight := contentHeight - logHeight
	sbStyle := MainBoxStyle.Copy().Width(sidebarWidth).Height(contentHeight); if m.focus == focusSidebar { if m.isEntered { sbStyle = sbStyle.BorderForeground(Green) } else { sbStyle = sbStyle.BorderForeground(Pink) } }
	sidebar := sbStyle.Render(m.sidebarList.View()); tabs := []string{"LOG", "STAGE", "BRANCHES", "TAGS", "REMOTES", "DIFF", "HELP", "REPO"}
	tabHeader := ""; for i, t := range tabs { style := InactiveStyle.Copy().Padding(0, 1); if int(m.activeTab) == i { style = SelectedStyle.Copy().Padding(0, 1).Underline(true) }; tabHeader += style.Render(t) }
	actionItems := m.getActionsForTab(); actionBar := ""; for i, a := range actionItems { style := InactiveStyle.Copy().Padding(0, 1).Background(DarkGray); if m.focus == focusMain && m.isEntered && m.subFocus == subAreaActions && i == m.actionIdx { style = SelectedStyle.Copy().Padding(0, 1).Background(Magenta).Foreground(White) }; actionBar += style.Render(a) + " " }
	logStyle := MainBoxStyle.Copy().Width(mainWidth).Height(logHeight); if m.focus == focusMain { if m.isEntered { logStyle = logStyle.BorderForeground(Green) } else { logStyle = logStyle.BorderForeground(Pink) } }
	logArea := logStyle.Render(lipgloss.JoinVertical(lipgloss.Left, tabHeader, actionBar, m.logViewport.View()))
	contentStyle := MainBoxStyle.Copy().Width(mainWidth).Height(viewHeight); if m.focus == focusContent { if m.isEntered { contentStyle = contentStyle.BorderForeground(Green) } else { contentStyle = contentStyle.BorderForeground(Pink) } }
	contentArea := contentStyle.Render(m.contentViewport.View()); return lipgloss.JoinHorizontal(lipgloss.Top, sidebar, lipgloss.JoinVertical(lipgloss.Left, logArea, contentArea))
}

func (m Model) renderDialog() string {
	title := "INPUT"
	var extra string
	switch m.activeDlg {
	case dialogMerge:
		title = "MERGE BRANCHES"
		extra = "\n" + SuccessStyle.Render(" Merge ") + " " + m.textInput.Value() + " " + SuccessStyle.Render(" into ") + " " + m.textInput2.Value() + "\n"
		cb := "[ ] Delete source branch after merge"
		if m.dialogCheckbox {
			cb = "[x] Delete source branch after merge"
		}
		extra += "\n" + InactiveStyle.Render(cb) + " (ctrl+d to toggle)\n"
	case dialogCommit:
		title = "COMMIT CHANGES"
	case dialogCreateBranch:
		title = "CREATE BRANCH"
	case dialogDeleteBranch:
		title = "DELETE BRANCH"
	case dialogAddRepo:
		title = "ADD REPOSITORY"
	case dialogCloneRepo:
		title = "CLONE REPOSITORY"
	case dialogInitRepo:
		title = "INIT REPOSITORY"
	}

	inputUI := m.textInput.View()
	if m.isMultiInput {
		inputUI = lipgloss.JoinVertical(lipgloss.Left, m.textInput.View(), "\n", m.textInput2.View())
	}
	ui := lipgloss.JoinVertical(lipgloss.Left, HeaderStyle.Render(title), extra, inputUI, "\n", InactiveStyle.Render("enter: confirm | esc: cancel"))
	box := MainBoxStyle.Copy().Padding(1, 2).Width(60).BorderForeground(Magenta)
	return lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, box.Render(ui))
}
