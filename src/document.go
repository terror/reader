package main

type Document struct {
  ID            string `json:"id"`
  Author        string `json:"author"`
  Category      string `json:"category"`
  HTMLContent   string `json:"html_content"`
  Location      string `json:"location"`
  PublishedDate any    `json:"published_date"`
  SourceURL     string `json:"source_url"`
  Summary       string `json:"summary"`
  Title         string `json:"title"`
  URL           string `json:"url"`
  WordCount     int    `json:"word_count"`
}
