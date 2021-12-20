package fafnir

type Config struct {
	MaxConcurrentDownloads int
	DBPath                 string
}

func NewConfig(maxConcurrentDownloads int, DBPath string) *Config {
	return &Config{
		MaxConcurrentDownloads: maxConcurrentDownloads,
		DBPath:                 DBPath,
	}
}
