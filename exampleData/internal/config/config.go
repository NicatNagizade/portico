package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	UserCount      int
	PostsMin       int
	PostsMax       int
	CommentsMin    int
	CommentsMax    int
	ReactionChance float64
	ReactionsMin   int
	ReactionsMax   int
	UserBatchSize  int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "postgres"),
		DBPassword: getEnv("DB_PASSWORD", "postgres"),
		DBName:     getEnv("DB_NAME", "portico_example"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		UserCount:      getEnvInt("USER_COUNT", 100_000),
		PostsMin:       getEnvInt("POSTS_MIN", 10),
		PostsMax:       getEnvInt("POSTS_MAX", 30),
		CommentsMin:    getEnvInt("COMMENTS_MIN", 20),
		CommentsMax:    getEnvInt("COMMENTS_MAX", 30),
		ReactionChance: getEnvFloat("REACTION_CHANCE", 0.35),
		ReactionsMin:   getEnvInt("REACTIONS_MIN", 1),
		ReactionsMax:   getEnvInt("REACTIONS_MAX", 4),
		UserBatchSize:  getEnvInt("USER_BATCH_SIZE", 500),
	}

	if cfg.UserCount < 1 {
		return nil, fmt.Errorf("USER_COUNT must be >= 1")
	}
	if cfg.PostsMin < 1 || cfg.PostsMax < cfg.PostsMin {
		return nil, fmt.Errorf("invalid POSTS_MIN/POSTS_MAX")
	}
	if cfg.CommentsMin < 1 || cfg.CommentsMax < cfg.CommentsMin {
		return nil, fmt.Errorf("invalid COMMENTS_MIN/COMMENTS_MAX")
	}
	if cfg.ReactionsMin < 1 || cfg.ReactionsMax < cfg.ReactionsMin {
		return nil, fmt.Errorf("invalid REACTIONS_MIN/REACTIONS_MAX")
	}
	if cfg.UserBatchSize < 1 {
		cfg.UserBatchSize = 500
	}
	return cfg, nil
}

func (c *Config) AdminDSN() string {
	// template1 always exists; safer than assuming a "postgres" database.
	return c.dsn("template1")
}

func (c *Config) DSN() string {
	return c.dsn(c.DBName)
}

func (c *Config) dsn(dbName string) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		c.DBHost, c.DBUser, c.DBPassword, dbName, c.DBPort, c.DBSSLMode,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}
