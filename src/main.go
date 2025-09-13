package main

import (
  "fmt"
  tea "github.com/charmbracelet/bubbletea"
  "github.com/charmbracelet/glamour"
  "github.com/microcosm-cc/bluemonday"
  "golang.org/x/text/cases"
  "golang.org/x/text/language"
  "log"
  "os"
  "regexp"
  "strings"
)

type state int

const (
  documentListView state = iota
  documentReadView
)

type model struct {
  state            state
  documents        []Document
  allDocuments     []Document
  categories       []Category
  selectedCategory int
  selected         int
  currentLocation  string
  content          string
  contentLines     []string
  scrollOffset     int
  width            int
  height           int
  api              *ReaderAPI
  loading          bool
  err              error
  renderer         *glamour.TermRenderer
}

type Category struct {
  Name     string
  Location string
  Count    int
}

type Document struct {
  ID            string `json:"id"`
  Title         string `json:"title"`
  Author        string `json:"author"`
  Summary       string `json:"summary"`
  URL           string `json:"url"`
  SourceURL     string `json:"source_url"`
  Category      string `json:"category"`
  Location      string `json:"location"`
  WordCount     int    `json:"word_count"`
  PublishedDate any    `json:"published_date"`
  HTMLContent   string `json:"html_content"`
}

func (d Document) GetPublishedDate() string {
  if d.PublishedDate == nil {
    return ""
  }

  switch v := d.PublishedDate.(type) {
  case string:
    return v
  case float64:
    return fmt.Sprintf("%.0f", v)
  default:
    return fmt.Sprintf("%v", v)
  }
}

func htmlToMarkdown(html string) string {
  if html == "" {
    return ""
  }

  content := html
  content = regexp.MustCompile(`<h1[^>]*>(.*?)</h1>`).ReplaceAllString(content, "# $1")
  content = regexp.MustCompile(`<h2[^>]*>(.*?)</h2>`).ReplaceAllString(content, "## $1")
  content = regexp.MustCompile(`<h3[^>]*>(.*?)</h3>`).ReplaceAllString(content, "### $1")
  content = regexp.MustCompile(`<h4[^>]*>(.*?)</h4>`).ReplaceAllString(content, "#### $1")
  content = regexp.MustCompile(`<h5[^>]*>(.*?)</h5>`).ReplaceAllString(content, "##### $1")
  content = regexp.MustCompile(`<h6[^>]*>(.*?)</h6>`).ReplaceAllString(content, "###### $1")
  content = regexp.MustCompile(`<strong[^>]*>(.*?)</strong>`).ReplaceAllString(content, "**$1**")
  content = regexp.MustCompile(`<b[^>]*>(.*?)</b>`).ReplaceAllString(content, "**$1**")
  content = regexp.MustCompile(`<em[^>]*>(.*?)</em>`).ReplaceAllString(content, "*$1*")
  content = regexp.MustCompile(`<i[^>]*>(.*?)</i>`).ReplaceAllString(content, "*$1*")
  content = regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`).ReplaceAllString(content, "[$2]($1)")
  content = regexp.MustCompile(`<p[^>]*>`).ReplaceAllString(content, "\n")
  content = regexp.MustCompile(`</p>`).ReplaceAllString(content, "\n")
  content = regexp.MustCompile(`<br[^>]*/?>`).ReplaceAllString(content, "\n")
  content = regexp.MustCompile(`<blockquote[^>]*>`).ReplaceAllString(content, "\n> ")
  content = regexp.MustCompile(`</blockquote>`).ReplaceAllString(content, "\n")

  p := bluemonday.StrictPolicy()

  content = p.Sanitize(content)
  content = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(content, "\n\n")
  content = strings.TrimSpace(content)

  return content
}

func (m model) Init() tea.Cmd {
  return loadAllDocuments(m.api)
}

type allDocumentsLoadedMsg []Document
type documentContentMsg string
type errorMsg error

func loadAllDocuments(api *ReaderAPI) tea.Cmd {
  return func() tea.Msg {
    docs, err := api.GetDocuments("", 50)

    if err != nil {
      return errorMsg(err)
    }

    return allDocumentsLoadedMsg(docs)
  }
}

func loadDocumentContent(doc Document) tea.Cmd {
  return func() tea.Msg {
    content := doc.HTMLContent

    if content == "" {
      content = doc.Summary
    }

    if strings.Contains(content, "<") {
      content = htmlToMarkdown(content)
    }

    if !strings.Contains(content, doc.Title) {
      fullContent := fmt.Sprintf("# %s\n\n", doc.Title)

      if doc.Author != "" {
        fullContent += fmt.Sprintf("*by %s*\n\n", doc.Author)
      }

      fullContent += content

      content = fullContent
    }

    return documentContentMsg(content)
  }
}

func buildCategories(documents []Document) []Category {
  locationCounts := make(map[string]int)

  for _, doc := range documents {
    if strings.TrimSpace(doc.Title) != "" {
      locationCounts[doc.Location]++
    }
  }

  var categories []Category

  locationNames := map[string]string{
    "new":       "📥 New",
    "later":     "🕐 Later",
    "archive":   "📦 Archive",
    "feed":      "📰 Feed",
    "shortlist": "⭐ Shortlist",
  }

  preferredOrder := []string{"new", "later", "archive", "feed", "shortlist"}

  for _, location := range preferredOrder {
    if count, exists := locationCounts[location]; exists && count > 0 {
      name := locationNames[location]
      categories = append(categories, Category{
        Name:     name,
        Location: location,
        Count:    count,
      })
    }
  }

  for location, count := range locationCounts {
    if count > 0 {
      found := false

      for _, existing := range categories {
        if existing.Location == location {
          found = true
          break
        }
      }

      if !found {
        name := locationNames[location]

        if name == "" {
          name = cases.Title(language.English).String(location)
        }

        categories = append(categories, Category{
          Name:     name,
          Location: location,
          Count:    count,
        })
      }
    }
  }

  return categories
}

func (m model) filterDocumentsByLocation(location string) []Document {
  var filtered []Document

  for _, doc := range m.allDocuments {
    if doc.Location == location {
      filtered = append(filtered, doc)
    }
  }

  return filtered
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
  switch msg := msg.(type) {

  case allDocumentsLoadedMsg:
    allDocs := []Document(msg)

    filteredDocs := make([]Document, 0)

    for _, doc := range allDocs {
      if strings.TrimSpace(doc.Title) != "" {
        filteredDocs = append(filteredDocs, doc)
      }
    }

    m.allDocuments = filteredDocs
    m.categories = buildCategories(filteredDocs)
    m.loading = false
    m.err = nil
    m.selectedCategory = 0

    if len(m.categories) > 0 {
      m.currentLocation = m.categories[0].Location
      m.documents = m.filterDocumentsByLocation(m.currentLocation)
    }

    m.state = documentListView

  case documentContentMsg:
    content := string(msg)

    m.content = content
    m.scrollOffset = 0

    if rendered, err := m.renderer.Render(content); err == nil {
      m.contentLines = strings.Split(rendered, "\n")
    } else {
      m.contentLines = strings.Split(content, "\n")
    }

  case errorMsg:
    m.err = error(msg)
    m.loading = false
  case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
  case tea.KeyMsg:
    switch msg.String() {
    case "ctrl+c", "q":
      return m, tea.Quit
    case "up":
      if m.state == documentListView && len(m.documents) > 0 && m.selected > 0 {
        m.selected--
      } else if m.state == documentReadView && m.scrollOffset > 0 {
        m.scrollOffset--
      }
    case "down":
      if m.state == documentListView && len(m.documents) > 0 && m.selected < len(m.documents)-1 {
        m.selected++
      } else if m.state == documentReadView && len(m.contentLines) > 0 {
        maxScroll := len(m.contentLines) - (m.height - 6)

        maxScroll = max(maxScroll, 0)

        if m.scrollOffset < maxScroll {
          m.scrollOffset++
        }
      }
    case "k":
      if m.state == documentListView && len(m.documents) > 0 && m.selected > 0 {
        m.selected--
      } else if m.state == documentReadView && m.scrollOffset > 0 {
        m.scrollOffset--
      }
    case "j":
      if m.state == documentListView && len(m.documents) > 0 && m.selected < len(m.documents)-1 {
        m.selected++
      } else if m.state == documentReadView && len(m.contentLines) > 0 {
        maxScroll := len(m.contentLines) - (m.height - 6)

        maxScroll = max(maxScroll, 0)

        if m.scrollOffset < maxScroll {
          m.scrollOffset++
        }
      }
    case "ctrl+u":
      // Page up
      if m.state == documentListView && len(m.documents) > 0 {
        pageSize := max(1, (m.height-8)/2) // Half page scroll
        m.selected = max(0, m.selected-pageSize)
      } else if m.state == documentReadView {
        pageSize := max(1, (m.height-6)/2) // Half page scroll
        m.scrollOffset = max(0, m.scrollOffset-pageSize)
      }
    case "ctrl+d":
      // Page down
      if m.state == documentListView && len(m.documents) > 0 {
        pageSize := max(1, (m.height-8)/2) // Half page scroll
        m.selected = min(len(m.documents)-1, m.selected+pageSize)
      } else if m.state == documentReadView && len(m.contentLines) > 0 {
        pageSize := max(1, (m.height-6)/2) // Half page scroll
        maxScroll := len(m.contentLines) - (m.height - 6)
        maxScroll = max(maxScroll, 0)
        m.scrollOffset = min(maxScroll, m.scrollOffset+pageSize)
      }
    case "left", "h":
      if m.state == documentListView && len(m.categories) > 0 && m.selectedCategory > 0 {
        m.selectedCategory--
        m.currentLocation = m.categories[m.selectedCategory].Location
        m.documents = m.filterDocumentsByLocation(m.currentLocation)
        m.selected = 0
      }
    case "right", "l":
      if m.state == documentListView && len(m.categories) > 0 && m.selectedCategory < len(m.categories)-1 {
        m.selectedCategory++
        m.currentLocation = m.categories[m.selectedCategory].Location
        m.documents = m.filterDocumentsByLocation(m.currentLocation)
        m.selected = 0
      }
    case "enter":
      if m.state == documentListView && len(m.documents) > 0 {
        m.state = documentReadView
        m.content = "" // Clear content while loading
        return m, loadDocumentContent(m.documents[m.selected])
      }
    case "esc", "backspace":
      if m.state == documentReadView {
        m.state = documentListView
        m.content = "" // Clear content when going back
        m.scrollOffset = 0
      }
    case "r":
      if m.state == documentListView {
        m.loading = true
        m.err = nil
        return m, loadAllDocuments(m.api)
      }
    case "?":
      // Could add help screen here in the future
    }
  }

  return m, nil
}

func (m model) View() string {
  switch m.state {
  case documentListView:
    return m.renderDocumentList()
  case documentReadView:
    return m.renderDocument()
  default:
    return "Unknown state"
  }
}

func (m model) renderDocumentList() string {
  s := "📚 Reader\n\n"

  if len(m.categories) > 1 {
    for i, category := range m.categories {
      if i > 0 {
        s += " | "
      }
      if i == m.selectedCategory {
        s += fmt.Sprintf("[%s (%d)]", category.Name, category.Count)
      } else {
        s += fmt.Sprintf("%s (%d)", category.Name, category.Count)
      }
    }
    s += "\n\n"
  } else if len(m.categories) == 1 {
    s += fmt.Sprintf("%s (%d)\n\n", m.categories[0].Name, m.categories[0].Count)
  }

  if m.err != nil {
    s += fmt.Sprintf("Error: %s\n", m.err.Error())
    s += "\nSet token: reader config set-token <token>\n"
  } else if m.loading {
    s += "Loading...\n"
  } else if len(m.documents) == 0 {
    s += "No documents found.\n"
  } else {
    maxVisible := m.height - 8 // Leave space for header/footer
    maxVisible = max(maxVisible, 5)

    start := 0
    end := len(m.documents)

    // If we have more documents than can fit, center around selected
    if len(m.documents) > maxVisible {
      start = m.selected - maxVisible/2
      start = max(start, 0)
      end = start + maxVisible
      if end > len(m.documents) {
        end = len(m.documents)
        start = end - maxVisible
      }
    }

    for i := start; i < end; i++ {
      cursor := " "

      if i == m.selected {
        cursor = ">"
      }

      s += fmt.Sprintf("%s %s\n", cursor, m.documents[i].Title)
    }

    // Show position indicator if there are more documents
    if len(m.documents) > maxVisible {
      s += fmt.Sprintf("\n(%d/%d)", m.selected+1, len(m.documents))
    }
  }

  helpText := "↑/↓ j/k move, enter read"

  if len(m.categories) > 1 {
    helpText += ", ←/→ h/l switch category"
  }

  helpText += ", r refresh, q quit"

  s += "\n\n" + helpText

  return s
}

func (m model) renderDocument() string {
  s := "📖 Reading\n\n"

  if m.content == "" {
    s += "Loading content..."
  } else if len(m.contentLines) > 0 {
    availableHeight := m.height - 6 // Leave space for header and footer

    if availableHeight < 1 {
      availableHeight = 10 // Minimum height
    }

    // Show the visible portion of content
    start := m.scrollOffset

    end := start + availableHeight
    end = min(end, len(m.contentLines))

    for i := start; i < end; i++ {
      s += m.contentLines[i] + "\n"
    }

    // Show scroll indicator
    if len(m.contentLines) > availableHeight {
      scrollPercent := float64(m.scrollOffset) / float64(len(m.contentLines)-availableHeight) * 100
      s += fmt.Sprintf("\n[%.0f%%]", scrollPercent)
    }
  }

  s += "\n\n↑/↓ j/k scroll, esc back, q quit"

  return s
}

func initialModel() model {
  token, err := getToken()

  if err != nil {
    fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
    os.Exit(1)
  }

  api := NewReaderAPI(token)

  renderer, err := glamour.NewTermRenderer(
    glamour.WithAutoStyle(),
    glamour.WithWordWrap(80),
  )

  if err != nil {
    log.Fatal(err)
  }

  return model{
    state:    documentListView,
    api:      api,
    loading:  true,
    selected: 0,
    renderer: renderer,
  }
}

func printUsage() {
  fmt.Println("reader - Terminal user-interface for Readwise Reader")
  fmt.Println()
  fmt.Println("Usage:")
  fmt.Println("  reader                    Start the TUI")
  fmt.Println("  reader config set-token <token>  Set your Readwise access token")
  fmt.Println()
  fmt.Println("Get your token from: https://readwise.io/access_token")
}

func run() {
  p := tea.NewProgram(initialModel(), tea.WithAltScreen())

  if _, err := p.Run(); err != nil {
    log.Fatal(err)
    os.Exit(1)
  }
}

func main() {
  args := os.Args[1:]

  if len(args) == 0 {
    run()
    return
  }

  switch args[0] {
  case "config":
    if len(args) < 2 {
      fmt.Fprintf(os.Stderr, "Error: config command requires a subcommand\n\n")
      printUsage()
      os.Exit(1)
    }

    switch args[1] {
    case "set-token":
      if len(args) < 3 {
        fmt.Fprintf(os.Stderr, "Error: set-token requires a token argument\n\n")
        printUsage()
        os.Exit(1)
      }
      if err := setToken(args[2]); err != nil {
        fmt.Fprintf(os.Stderr, "Error setting token: %s\n", err.Error())
        os.Exit(1)
      }
    default:
      fmt.Fprintf(os.Stderr, "Error: unknown config subcommand '%s'\n\n", args[1])
      printUsage()
      os.Exit(1)
    }

  case "help", "--help", "-h":
    printUsage()

  default:
    fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", args[0])
    printUsage()
    os.Exit(1)
  }
}
