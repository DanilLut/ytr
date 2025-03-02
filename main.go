package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"runtime"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fatih/color"
)

const (
	configDirName  = "ytr"
	dbFileName     = "data.json"
	shortIDLength  = 6
	charset        = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Margin(1, 2)
)

type VideoEntry struct {
	ID        string `json:"id"`
	ShortID   string `json:"short_id"`
	URL       string `json:"url"`
	VideoTitle string `json:"title"`
	Timestamp int64  `json:"timestamp"`
}

func (v VideoEntry) FilterValue() string { return v.VideoTitle }
func (v VideoEntry) Title() string       { return v.VideoTitle }
func (v VideoEntry) Description() string { 
	return fmt.Sprintf("Added: %s", time.Unix(v.Timestamp, 0).Format("2006-01-02 15:04"))
}


type Model struct {
	entries      []VideoEntry
	list         list.Model
	textInput    textinput.Model
	state        int
	selectedItem *VideoEntry
	err          error
	loading      bool
}

type addVideoMsg struct {
	entry VideoEntry
	err   error
}

func printHelp() {
    fmt.Println("Usage:")
    fmt.Println("  -t            Run TUI mode")
    fmt.Println("  <YouTube URL> Add a YouTube video")
    fmt.Println("  -h, --help    Show this help message")
}

func main() {
    rand.New(rand.NewSource(time.Now().UnixNano()))

    if len(os.Args) > 1 {
        arg := os.Args[1]

        if arg == "-h" || arg == "--help" {
            printHelp()
            os.Exit(0)
        }

        if arg == "-t" {
            m, err := initialModel()
            if err != nil {
                color.Red("Error initializing model: %v\n", err)
                os.Exit(1)
            }

            p := tea.NewProgram(m, tea.WithAltScreen())
            if _, err := p.Run(); err != nil {
                color.Red("Error running program: %v\n", err)
                os.Exit(1)
            }
            return
        }

        if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
            runAddVideo(arg)
            return
        }

        printHelp()
        os.Exit(1)
    }

    runRandomVideo()

}

func runRandomVideo() {
	entries, err := loadDB()
	if err != nil || len(entries) == 0 {
		fmt.Println("No videos available.")
		os.Exit(1)
	}

	idx := rand.Intn(len(entries))
	entry := entries[idx]

	openURL(entry.URL)

	entries = append(entries[:idx], entries[idx+1:]...)
	saveDB(entries)
}

func runAddVideo(rawURL string) {
	videoID, err := extractYoutubeID(rawURL)
	if err != nil {
		color.Red("Invalid YouTube URL: %s", err)
		os.Exit(1)
	}

	entries, err := loadDB()
	if err != nil {
		color.Red("Error loading database: %s", err)
		os.Exit(1)
	}

	for _, e := range entries {
		if e.ID == videoID || e.URL == rawURL {
			color.Red("Duplicate entry: %s", e.VideoTitle)
			os.Exit(1)
		}
	}

	title, err := getYoutubeTitle(videoID)
	if err != nil {
		title = rawURL
	}

	entry := VideoEntry{
		ID:         videoID,
		ShortID:    generateShortID(entries),
		URL:        rawURL,
		VideoTitle: title,
		Timestamp:  time.Now().Unix(),
	}

	entries = append(entries, entry)
	saveDB(entries)

	color.Green("Added video: %s", title)
}

func initialModel() (*Model, error) {
	entries, err := loadDB()
	if err != nil {
		return nil, err
	}

	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = e
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "YouTube Videos"
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "add video"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "delete video"),
			),
			key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "play random"),
			),
			key.NewBinding(
				key.WithKeys("q"),
				key.WithHelp("q", "quit"),
			),
		}
	}

	ti := textinput.New()
	ti.Placeholder = "Enter YouTube URL"
	ti.Focus()

	return &Model{
		entries:   entries,
		list:      l,
		textInput: ti,
		state:     0,
	}, nil
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.err != nil {
			m.err = nil
			return m, nil
		}

		if m.list.FilterState() == list.Filtering {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch m.state {
		case 0:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit

			case "a":
				m.state = 1
				return m, m.textInput.Focus()

			case "d":
				if selected, ok := m.list.SelectedItem().(VideoEntry); ok {
					newEntries := make([]VideoEntry, 0, len(m.entries)-1)
					for _, e := range m.entries {
						if e.ShortID != selected.ShortID {
							newEntries = append(newEntries, e)
						}
					}
					m.entries = newEntries
					saveDB(m.entries)

					items := make([]list.Item, len(m.entries))
					for i, e := range m.entries {
						items[i] = e
					}
					m.list.SetItems(items)
				}
				return m, nil

			case "r":
				if len(m.entries) == 0 {
					m.err = fmt.Errorf("no videos available")
					return m, nil
				}

				idx := rand.Intn(len(m.entries))
				entry := m.entries[idx]
                openURL(entry.URL)

				m.entries = append(m.entries[:idx], m.entries[idx+1:]...)
				saveDB(m.entries)

				items := make([]list.Item, len(m.entries))
				for i, e := range m.entries {
					items[i] = e
				}
				m.list.SetItems(items)
				return m, nil

			case "enter":
				if selected, ok := m.list.SelectedItem().(VideoEntry); ok {
                    openURL(selected.URL)

					newEntries := make([]VideoEntry, 0, len(m.entries)-1)
					for _, e := range m.entries {
						if e.ShortID != selected.ShortID {
							newEntries = append(newEntries, e)
						}
					}
					m.entries = newEntries
					saveDB(m.entries)

					items := make([]list.Item, len(m.entries))
					for i, e := range m.entries {
						items[i] = e
					}
					m.list.SetItems(items)
				}
				return m, nil
			}

		case 1:
			switch msg.String() {
			case "esc":
				m.state = 0
				return m, nil

			case "enter":
				url := m.textInput.Value()
				m.textInput.Reset()
				m.state = 0
				return m, m.processAddURL(url)
			}
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case addVideoMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.entries = append(m.entries, msg.entry)
		saveDB(m.entries)
		items := make([]list.Item, len(m.entries))
		for i, e := range m.entries {
			items[i] = e
		}
		m.list.SetItems(items)
		return m, nil
	}

	var cmd tea.Cmd
	if m.state == 0 {
		m.list, cmd = m.list.Update(msg)
	} else if m.state == 1 {
		m.textInput, cmd = m.textInput.Update(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress any key to continue.", m.err)
	}

	switch m.state {
	case 0:
		listView := docStyle.Render(m.list.View())
		help := helpStyle.Render("a: add • d: delete • r: play random • q: quit")
		return lipgloss.JoinVertical(lipgloss.Left, listView, help)
	case 1:
		return fmt.Sprintf(
			"Enter YouTube URL:\n%s\n\n(esc to cancel)",
			m.textInput.View(),
		)
	default:
		return ""
	}
}

func (m *Model) processAddURL(rawURL string) tea.Cmd {
	return func() tea.Msg {
		videoID, err := extractYoutubeID(rawURL)
		if err != nil {
			return addVideoMsg{err: err}
		}

		for _, e := range m.entries {
			if e.ID == videoID || e.URL == rawURL {
				return addVideoMsg{err: fmt.Errorf("duplicate entry: %s", e.VideoTitle)}
			}
		}

		title, err := getYoutubeTitle(videoID)
		if err != nil {
			title = rawURL
		}

		entry := VideoEntry{
			ID:        videoID,
			ShortID:   generateShortID(m.entries),
			URL:       rawURL,
			VideoTitle: title,
			Timestamp: time.Now().Unix(),
		}

		return addVideoMsg{entry: entry}
	}
}

func extractYoutubeID(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	switch {
	case u.Host == "youtu.be":
		return strings.TrimPrefix(u.Path, "/"), nil
	case strings.Contains(u.Host, "youtube.com"):
		if u.Path == "/watch" {
			return u.Query().Get("v"), nil
		}
		if strings.HasPrefix(u.Path, "/embed/") {
			parts := strings.Split(u.Path, "/")
			if len(parts) >= 3 {
				return parts[2], nil
			}
		}
	}
	return "", fmt.Errorf("invalid YouTube URL")
}

func getYoutubeTitle(videoID string) (string, error) {
	resp, err := http.Get(fmt.Sprintf(
		"https://www.youtube.com/oembed?url=https://www.youtube.com/watch?v=%s&format=json",
		videoID,
	))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.Title, nil
}

func generateShortID(entries []VideoEntry) string {
	existing := make(map[string]bool)
	for _, e := range entries {
		existing[e.ShortID] = true
	}

	for {
		id := make([]byte, shortIDLength)
		for i := range id {
			id[i] = charset[rand.Intn(len(charset))]
		}
		shortID := string(id)
		if !existing[shortID] {
			return shortID
		}
	}
}

func getConfigDir() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, configDirName)
}

func loadDB() ([]VideoEntry, error) {
	configDir := getConfigDir()
	dbPath := filepath.Join(configDir, dbFileName)

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	file, err := os.Open(dbPath)
	if os.IsNotExist(err) {
		return []VideoEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []VideoEntry
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		return nil, err
	}

    sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	return entries, nil
}

func saveDB(entries []VideoEntry) error {
	configDir := getConfigDir()
	dbPath := filepath.Join(configDir, dbFileName)

    sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	file, err := os.Create(dbPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

func openURL(url string) {
	var cmd *exec.Cmd

	switch {
	case strings.Contains(url, "http"):
		if isWSL() {
			cmd = exec.Command("wslview", url) // WSL
		} else if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", url) // Linux
		} else if _, err := exec.LookPath("open"); err == nil {
			cmd = exec.Command("open", url) // macOS
		} else if _, err := exec.LookPath("cmd"); err == nil {
			cmd = exec.Command("cmd", "/c", "start", url) // Windows
		} else {
			color.Red("No supported method to open URLs found.")
			return
		}

		cmd.Stderr = nil
		cmd.Stdout = nil
		cmd.Start()
	}
}

func isWSL() bool {
	if runtime.GOOS == "linux" {
		if output, err := exec.Command("uname", "-r").Output(); err == nil {
			return strings.Contains(string(output), "microsoft")
		}
	}
	return false
}
