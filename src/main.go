package main

import (
  "fmt"
  tea "github.com/charmbracelet/bubbletea"
  "log"
  "os"
)

func help() {
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
      fmt.Fprintf(os.Stderr, "error: config command requires a subcommand\n\n")
      help()
      os.Exit(1)
    }
    switch args[1] {
    case "set-token":
      if len(args) < 3 {
        fmt.Fprintf(os.Stderr, "error: set-token requires a token argument\n\n")
        help()
        os.Exit(1)
      }
      if err := setToken(args[2]); err != nil {
        fmt.Fprintf(os.Stderr, "error setting token: %s\n", err.Error())
        os.Exit(1)
      }
    default:
      fmt.Fprintf(os.Stderr, "error: unknown config subcommand '%s'\n\n", args[1])
      help()
      os.Exit(1)
    }
  case "help", "--help", "-h":
    help()
  default:
    fmt.Fprintf(os.Stderr, "error: unknown command '%s'\n\n", args[0])
    help()
    os.Exit(1)
  }
}
