package main

import (
  "fmt"
  tea "github.com/charmbracelet/bubbletea"
  "github.com/charmbracelet/glamour"
  "log"
  "os"
  "strings"
)

type state int

const (
  documentListView state = iota
  documentReadView
)

type allDocumentsLoadedMsg []Document
type documentContentMsg string
type errorMsg error

type App struct {
  allDocuments     []Document
  api              *ReaderAPI
  categories       []Category
  content          string
  contentLines     []string
  currentLocation  string
  documents        []Document
  err              error
  height           int
  loading          bool
  renderer         *glamour.TermRenderer
  scrollOffset     int
  selected         int
  selectedCategory int
  state            state
  width            int
}

func (m App) Init() tea.Cmd {
  return loadAllDocuments(m.api)
}

func NewModel() App {
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

  return App{
    state:    documentListView,
    api:      api,
    loading:  true,
    selected: 0,
    renderer: renderer,
  }
}

func (m App) View() string {
  switch m.state {
  case documentListView:
    return m.renderDocumentList()
  case documentReadView:
    return m.renderDocument()
  default:
    return "Unknown state"
  }
}

func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
      if m.state == documentListView && len(m.documents) > 0 {
        pageSize := max(1, (m.height-8)/2)
        m.selected = max(0, m.selected-pageSize)
      } else if m.state == documentReadView {
        pageSize := max(1, (m.height-6)/2)
        m.scrollOffset = max(0, m.scrollOffset-pageSize)
      }
    case "ctrl+d":
      if m.state == documentListView && len(m.documents) > 0 {
        pageSize := max(1, (m.height-8)/2)
        m.selected = min(len(m.documents)-1, m.selected+pageSize)
      } else if m.state == documentReadView && len(m.contentLines) > 0 {
        pageSize := max(1, (m.height-6)/2)
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
        m.content = ""
        return m, loadDocumentContent(m.documents[m.selected])
      }
    case "esc", "backspace":
      if m.state == documentReadView {
        m.state = documentListView
        m.content = ""
        m.scrollOffset = 0
      }
    case "r":
      if m.state == documentListView {
        m.loading = true
        m.err = nil
        return m, loadAllDocuments(m.api)
      }
    }
  }

  return m, nil
}

func (m App) renderDocumentList() string {
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
    maxVisible := m.height - 8
    maxVisible = max(maxVisible, 5)

    start := 0
    end := len(m.documents)

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

func (m App) renderDocument() string {
  s := "📖 Reading\n\n"

  if m.content == "" {
    s += "Loading content..."
  } else if len(m.contentLines) > 0 {
    availableHeight := m.height - 6

    if availableHeight < 1 {
      availableHeight = 10
    }

    start := m.scrollOffset

    end := start + availableHeight
    end = min(end, len(m.contentLines))

    for i := start; i < end; i++ {
      s += m.contentLines[i] + "\n"
    }

    if len(m.contentLines) > availableHeight {
      scrollPercent := float64(m.scrollOffset) / float64(len(m.contentLines)-availableHeight) * 100
      s += fmt.Sprintf("\n[%.0f%%]", scrollPercent)
    }
  }

  s += "\n\n↑/↓ j/k scroll, esc back, q quit"

  return s
}

func (m App) filterDocumentsByLocation(location string) []Document {
  var filtered []Document

  for _, doc := range m.allDocuments {
    if doc.Location == location {
      filtered = append(filtered, doc)
    }
  }

  return filtered
}
