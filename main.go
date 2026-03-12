package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"atlas.gitty/internal/ui"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "-v" || arg == "--version" {
			fmt.Printf("atlas.gitty v%s\n", Version)
			return
		}
		if arg == "-h" || arg == "--help" {
			fmt.Printf("atlas.gitty v%s - Comprehensive TUI git client\n\n", Version)
			fmt.Println("Usage: atlas.gitty [options]")
			fmt.Println("\nOptions:")
			fmt.Println("  -v, --version  Show version information")
			fmt.Println("  -h, --help     Show this help message")
			fmt.Println("\nTUI Navigation:")
			fmt.Println("  - Tab / Shift+Tab: Cycle focus between bubbles (Pink border = hovered)")
			fmt.Println("  - Enter: ENTER a focused bubble (Green border = active interaction)")
			fmt.Println("  - Esc: EXIT active bubble back to navigation mode")
			fmt.Println("  - [ / ]: Switch tabs globally")
			fmt.Println("\nCore Features:")
			fmt.Println("  - LOG: Browse commit history. Press SPACE on a commit to view its diff.")
			fmt.Println("  - STAGE: Enter to toggle stage/unstage. Press SPACE to view diff.")
			fmt.Println("           Supports fast bulk 'STAGE ALL' via system Git CLI.")
			fmt.Println("  - BRANCHES: Manage branches. Click 'MERGE' for a safe, multi-option merge.")
			fmt.Println("              Use CTRL+D in the merge dialog to toggle auto-deletion of source.")
			fmt.Println("  - REPO LIST: Press DELETE on any repository to remove it from your list.")
			fmt.Println("\nAuthentication:")
			fmt.Println("  - Transparently falls back to system 'git' CLI for SSH/HTTPS authentication,")
			fmt.Println("    ensuring your existing credential helpers work out of the box.")
			return
		}
	}

	// TUI Mode
	p := tea.NewProgram(ui.NewInitialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
