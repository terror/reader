package main

import (
  "encoding/json"
  "fmt"
  "os"
  "path/filepath"
)

type Config struct {
  Token string `json:"token"`
}

func getConfigDir() (string, error) {
  homeDir, err := os.UserHomeDir()

  if err != nil {
    return "", fmt.Errorf("failed to get user home directory: %w", err)
  }

  configDir := filepath.Join(homeDir, ".config", "reader-tui")

  if err := os.MkdirAll(configDir, 0755); err != nil {
    return "", fmt.Errorf("failed to create config directory: %w", err)
  }

  return configDir, nil
}

func getConfigPath() (string, error) {
  configDir, err := getConfigDir()

  if err != nil {
    return "", err
  }

  return filepath.Join(configDir, "config.json"), nil
}

func loadConfig() (*Config, error) {
  configPath, err := getConfigPath()

  if err != nil {
    return nil, err
  }

  if _, err := os.Stat(configPath); os.IsNotExist(err) {
    return &Config{}, nil
  }

  data, err := os.ReadFile(configPath)

  if err != nil {
    return nil, fmt.Errorf("failed to read config file: %w", err)
  }

  var config Config

  if err := json.Unmarshal(data, &config); err != nil {
    return nil, fmt.Errorf("failed to parse config file: %w", err)
  }

  return &config, nil
}

func saveConfig(config *Config) error {
  configPath, err := getConfigPath()

  if err != nil {
    return err
  }

  data, err := json.MarshalIndent(config, "", "  ")

  if err != nil {
    return fmt.Errorf("failed to marshal config: %w", err)
  }

  if err := os.WriteFile(configPath, data, 0600); err != nil {
    return fmt.Errorf("failed to write config file: %w", err)
  }

  return nil
}

func setToken(token string) error {
  config, err := loadConfig()

  if err != nil {
    return err
  }

  config.Token = token

  if err := saveConfig(config); err != nil {
    return err
  }

  fmt.Printf("Token saved to config file.\n")

  return nil
}

func getToken() (string, error) {
  if token := os.Getenv("READWISE_TOKEN"); token != "" {
    return token, nil
  }

  config, err := loadConfig()

  if err != nil {
    return "", err
  }

  if config.Token == "" {
    return "", fmt.Errorf("no token found. Set it with: reader-tui config set-token <token>\nOr set READWISE_TOKEN environment variable\nGet your token from: https://readwise.io/access_token")
  }

  return config.Token, nil
}
