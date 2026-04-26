// Package config loads bite's runtime configuration.
//
// All settings come from environment variables — parsed declaratively from
// struct tags by github.com/caarlos0/env. A .env file in the working directory
// is loaded best-effort by github.com/joho/godotenv before parsing.
//
// To add a new setting: add a field to Config with `env:"…"` tags. That's it.
// No init code, no extra wiring, no constants to maintain.
package config
