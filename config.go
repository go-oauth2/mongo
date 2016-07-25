package mongo

// Config MongoDB Config
type Config struct {
	URL string
	DB  string
}

// NewConfig Create MongoDB Config
func NewConfig(url, db string) *Config {
	return &Config{
		URL: url,
		DB:  db,
	}
}
