package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gastos/internal/secret"
)

const minInviteCodeLen = 8

// DefaultAnthropicURL is the Anthropic API base URL used unless GASTOS_ANTHROPIC_URL overrides it.
const DefaultAnthropicURL = "https://api.anthropic.com"

// DefaultFXURL is the USD-BRL quote endpoint used unless GASTOS_FX_URL overrides it.
const DefaultFXURL = "https://economia.awesomeapi.com.br/last/USD-BRL"

type Config struct {
	Addr         string
	DBPath       string
	BackupDir    string
	BackupKeep   int
	Location     *time.Location
	CookieSecure bool
	Dev          bool
	// InviteCode, when set, is required to sign up once the database has users.
	InviteCode string
	// SecretKey seals stored API keys; nil when GASTOS_SECRET_KEY is unset, which disables the assistant.
	SecretKey []byte
	// AnthropicURL is the Anthropic API base URL; tests point it at a local fake.
	AnthropicURL string
	// FXURL returns the USD-BRL quote (AwesomeAPI shape) used to price assistant usage; tests point it at a fake.
	FXURL string
}

// Load reads the configuration from getenv (os.Getenv in production) and fails early on invalid values.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:         or(getenv("GASTOS_ADDR"), ":8000"),
		DBPath:       or(getenv("GASTOS_DB_PATH"), "./data/gastos.db"),
		BackupDir:    or(getenv("GASTOS_BACKUP_DIR"), "./data/backups"),
		AnthropicURL: or(getenv("GASTOS_ANTHROPIC_URL"), DefaultAnthropicURL),
		FXURL:        or(getenv("GASTOS_FX_URL"), DefaultFXURL),
	}

	keep, err := strconv.Atoi(or(getenv("GASTOS_BACKUP_KEEP"), "14"))
	if err != nil || keep < 1 {
		return Config{}, fmt.Errorf("invalid GASTOS_BACKUP_KEEP: %q", getenv("GASTOS_BACKUP_KEEP"))
	}
	c.BackupKeep = keep

	c.Location, err = time.LoadLocation(or(getenv("GASTOS_TZ"), "America/Sao_Paulo"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid GASTOS_TZ: %w", err)
	}
	if c.CookieSecure, err = parseBool("GASTOS_COOKIE_SECURE", getenv, true); err != nil {
		return Config{}, err
	}
	if c.Dev, err = parseBool("GASTOS_DEV", getenv, false); err != nil {
		return Config{}, err
	}
	c.InviteCode = strings.TrimSpace(getenv("GASTOS_INVITE_CODE"))
	if c.InviteCode != "" && len(c.InviteCode) < minInviteCodeLen {
		return Config{}, fmt.Errorf("invalid GASTOS_INVITE_CODE: use at least %d characters", minInviteCodeLen)
	}
	if raw := strings.TrimSpace(getenv("GASTOS_SECRET_KEY")); raw != "" {
		if c.SecretKey, err = secret.ParseKey(raw); err != nil {
			return Config{}, fmt.Errorf("invalid GASTOS_SECRET_KEY: %w", err)
		}
	}
	return c, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseBool(key string, getenv func(string) string, def bool) (bool, error) {
	v := getenv(key)
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %q", key, v)
	}
	return b, nil
}
