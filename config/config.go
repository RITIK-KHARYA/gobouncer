package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisAddr  string
	ServerPort string
}

func Load() Config {
	// load .env file — only in development
	// in production, env vars are set by the system directly
	if err := godotenv.Load(); err != nil {
		log.Println("config: no .env file found, reading from environment")
	}

	cfg := Config{
		RedisAddr:  getEnv("REDIS_ADDR", "localhost:6379"),
		ServerPort: ":" + getEnv("PORT", "8080"),
	}

	// validate at startup — fail loudly rather than silently misconfigured
	if err := cfg.validate(); err != nil {
		log.Fatalf("config: invalid configuration: %v", err)
	}

	log.Printf("config: loaded — Redis: %s  Port: %s", cfg.RedisAddr, cfg.ServerPort)
	return cfg
}

// getEnv reads an env var and falls back to a default if not set
func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

// validate checks required fields are present
func (c *Config) validate() error {
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	if c.ServerPort == "" {
		return fmt.Errorf("PORT is required")
	}
	return nil
}
