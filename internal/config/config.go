// Package config
package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/henrywhitaker3/windowframe/config"
)

type URL struct {
	Host   string `env:"HOST"`
	Scheme string `env:"SCHEME,default=https"`
	Group  *int   `env:"GROUP"`
	UUID   string `env:"UUID"`
}

func (u URL) URL() (*url.URL, error) {
	url, err := url.Parse(fmt.Sprintf("%s://%s", u.Scheme, u.Host))
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	url.Path = "/api/v2/auth"
	if u.Group != nil {
		query := url.Query()
		query.Add("group", fmt.Sprintf("%d", *u.Group))
		url.RawQuery = query.Encode()
	}
	return url, nil
}

type Config struct {
	LogLevel       string        `env:"LOG_LEVEL,default=info"`
	Port           int           `env:"PORT, default=12345"`
	CacheResponses bool          `env:"CACHE_RESPONSES,default=true"`
	CacheDuration  time.Duration `env:"CACHE_DURATION,default=5m"`

	Organizr URL `env:",prefix=ORGANIZR_"`
}

func Parse() (*Config, error) {
	conf, err := config.NewParser[Config]().WithExtractors(
		config.NewEnvExtractor[Config](),
	).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &conf, nil
}
