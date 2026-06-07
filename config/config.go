package config

type Config struct {
    RedisAddr  string
    ServerPort string
    Algorithm  string // "sliding_window" or "gcra"
}

func Load() Config {
    return Config{
        RedisAddr:  "localhost:6379",
        ServerPort: ":8080",
        Algorithm:  "sliding_window",
    }
}