package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment string
	HTTP        HTTP
	Database    Database
	Modules     map[string]bool
	Security    Security
}

type Security struct {
	MasterKey, AuditKey []byte
	CookieSecure        bool
	SessionTTL, IdleTTL time.Duration
}

type HTTP struct {
	Address         string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
	MaxHeaderBytes  int
}

type Database struct {
	URL            string
	MaxConnections int32
	MinConnections int32
	ConnectTimeout time.Duration
	MigrateOnStart bool
}

func Load() (Config, error) {
	c := Config{
		Environment: env("CONNECTME_ENV", "development"),
		HTTP:        HTTP{Address: env("CONNECTME_HTTP_ADDRESS", ":8080"), ReadTimeout: duration("CONNECTME_HTTP_READ_TIMEOUT", 10*time.Second), WriteTimeout: duration("CONNECTME_HTTP_WRITE_TIMEOUT", 30*time.Second), IdleTimeout: duration("CONNECTME_HTTP_IDLE_TIMEOUT", 60*time.Second), ShutdownTimeout: duration("CONNECTME_SHUTDOWN_TIMEOUT", 20*time.Second), MaxHeaderBytes: integer("CONNECTME_HTTP_MAX_HEADER_BYTES", 1<<20)},
		Database:    Database{URL: os.Getenv("CONNECTME_DATABASE_URL"), MaxConnections: int32(integer("CONNECTME_DATABASE_MAX_CONNECTIONS", 20)), MinConnections: int32(integer("CONNECTME_DATABASE_MIN_CONNECTIONS", 2)), ConnectTimeout: duration("CONNECTME_DATABASE_CONNECT_TIMEOUT", 5*time.Second), MigrateOnStart: boolean("CONNECTME_DATABASE_MIGRATE_ON_START", true)},
		Modules:     map[string]bool{"system": true},
		Security:    Security{CookieSecure: boolean("CONNECTME_COOKIE_SECURE", false), SessionTTL: duration("CONNECTME_SESSION_TTL", 12*time.Hour), IdleTTL: duration("CONNECTME_SESSION_IDLE_TTL", 30*time.Minute)},
	}
	var err error
	c.Security.MasterKey, err = key("CONNECTME_MASTER_KEY")
	if err != nil {
		return Config{}, err
	}
	c.Security.AuditKey, err = key("CONNECTME_AUDIT_HMAC_KEY")
	if err != nil {
		return Config{}, err
	}
	for _, item := range strings.Split(os.Getenv("CONNECTME_MODULES"), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 {
			return Config{}, fmt.Errorf("CONNECTME_MODULES: entrada inválida %q", item)
		}
		on, err := strconv.ParseBool(parts[1])
		if err != nil {
			return Config{}, fmt.Errorf("CONNECTME_MODULES %q: %w", item, err)
		}
		c.Modules[parts[0]] = on
	}
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func key(name string) ([]byte, error) {
	v := os.Getenv(name)
	if v == "" {
		return nil, fmt.Errorf("%s é obrigatório", name)
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("%s deve ser base64: %w", name, err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("%s deve conter exatamente 32 bytes", name)
	}
	return b, nil
}

func (c Config) Validate() error {
	if c.HTTP.Address == "" {
		return errors.New("endereço HTTP vazio")
	}
	if c.Database.URL == "" {
		return errors.New("CONNECTME_DATABASE_URL é obrigatório")
	}
	if c.Database.MinConnections < 0 || c.Database.MaxConnections < 1 || c.Database.MinConnections > c.Database.MaxConnections {
		return errors.New("limites de conexões PostgreSQL inválidos")
	}
	if enabled, ok := c.Modules["system"]; ok && !enabled {
		return errors.New("módulo crítico system não pode ser desativado")
	}
	return nil
}

func env(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
func integer(k string, fallback int) int {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
func boolean(k string, fallback bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return n
}
func duration(k string, fallback time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
