package main

import (
  "bytes"
  "encoding/json"
  "fmt"
  "io"
  "net/http"
  "time"
)

type ReaderAPI struct {
  token   string
  baseURL string
  client  *http.Client
}

type DocumentsResponse struct {
  Count          int        `json:"count"`
  NextPageCursor string     `json:"nextPageCursor"`
  Results        []Document `json:"results"`
}

func NewReaderAPI(token string) *ReaderAPI {
  return &ReaderAPI{
    token:   token,
    baseURL: "https://readwise.io/api/v3",
    client: &http.Client{
      Timeout: 30 * time.Second,
    },
  }
}

func (r *ReaderAPI) makeRequest(method, endpoint string, body any) (*http.Response, error) {
  var bodyReader io.Reader

  if body != nil {
    jsonBody, err := json.Marshal(body)

    if err != nil {
      return nil, fmt.Errorf("failed to marshal request body: %w", err)
    }

    bodyReader = bytes.NewBuffer(jsonBody)
  }

  req, err := http.NewRequest(method, r.baseURL+endpoint, bodyReader)

  if err != nil {
    return nil, fmt.Errorf("failed to create request: %w", err)
  }

  req.Header.Set("Authorization", "Token "+r.token)
  req.Header.Set("Content-Type", "application/json")

  resp, err := r.client.Do(req)

  if err != nil {
    return nil, fmt.Errorf("request failed: %w", err)
  }

  return resp, nil
}

func (r *ReaderAPI) GetDocuments(location string, limit int) ([]Document, error) {
  endpoint := "/list/?withHtmlContent=true"

  if location != "" {
    endpoint += "&location=" + location
  }

  resp, err := r.makeRequest("GET", endpoint, nil)

  if err != nil {
    return nil, err
  }

  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
  }

  var documentsResp DocumentsResponse

  if err := json.NewDecoder(resp.Body).Decode(&documentsResp); err != nil {
    return nil, fmt.Errorf("failed to decode response: %w", err)
  }

  return documentsResp.Results, nil
}

func (r *ReaderAPI) GetDocumentContent(documentID string) (string, error) {
  endpoint := "/list/?id=" + documentID + "&withHtmlContent=true"

  resp, err := r.makeRequest("GET", endpoint, nil)

  if err != nil {
    return "", err
  }

  defer resp.Body.Close()

  if resp.StatusCode != http.StatusOK {
    return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
  }

  var documentsResp DocumentsResponse

  if err := json.NewDecoder(resp.Body).Decode(&documentsResp); err != nil {
    return "", fmt.Errorf("failed to decode response: %w", err)
  }

  if len(documentsResp.Results) == 0 {
    return "", fmt.Errorf("document not found")
  }

  return documentsResp.Results[0].Summary, nil
}

func (r *ReaderAPI) ValidateToken() error {
  resp, err := http.Get("https://readwise.io/api/v2/auth/")

  if err != nil {
    return fmt.Errorf("failed to validate token: %w", err)
  }

  defer resp.Body.Close()

  if resp.StatusCode != http.StatusNoContent {
    return fmt.Errorf("invalid token")
  }

  return nil
}
