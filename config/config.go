package config

type Config struct {
    RedisAddr  string
    ServerPort string
}

func Load() Config {
    return Config{
        RedisAddr:  "localhost:6379",
        ServerPort: ":8080",
    }
}