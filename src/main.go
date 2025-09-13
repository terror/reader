package main

import (
  "fmt"
  tea "github.com/charmbracelet/bubbletea"
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

type Category struct {
  Name     string
  Count    int
  Location string
}

type allDocumentsLoadedMsg []Document
type documentContentMsg string
type errorMsg error

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

func printUsage() {
  fmt.Println("reader - Terminal user-interface for Readwise Reader")
  fmt.Println()
  fmt.Println("Usage:")
  fmt.Println("  reader                           Start the TUI")
  fmt.Println("  reader config set-token <token>  Set your Readwise access token")
  fmt.Println()
  fmt.Println("Get your token from: https://readwise.io/access_token")
}

func run() {
  p := tea.NewProgram(NewModel(), tea.WithAltScreen())

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
