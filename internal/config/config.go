package config

import (
	"fmt"
	"strconv"
	"time"
)

type Config struct {
	Addr         string
	DBPath       string
	BackupDir    string
	BackupKeep   int
	Location     *time.Location
	CookieSecure bool
	Dev          bool
}

// Load lê a configuração de getenv (os.Getenv em produção) e falha cedo em valores inválidos.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:      or(getenv("GASTOS_ADDR"), ":8000"),
		DBPath:    or(getenv("GASTOS_DB_PATH"), "./data/gastos.db"),
		BackupDir: or(getenv("GASTOS_BACKUP_DIR"), "./data/backups"),
	}

	keep, err := strconv.Atoi(or(getenv("GASTOS_BACKUP_KEEP"), "14"))
	if err != nil || keep < 1 {
		return Config{}, fmt.Errorf("GASTOS_BACKUP_KEEP inválido: %q", getenv("GASTOS_BACKUP_KEEP"))
	}
	c.BackupKeep = keep

	c.Location, err = time.LoadLocation(or(getenv("GASTOS_TZ"), "America/Sao_Paulo"))
	if err != nil {
		return Config{}, fmt.Errorf("GASTOS_TZ inválido: %w", err)
	}
	if c.CookieSecure, err = parseBool("GASTOS_COOKIE_SECURE", getenv, true); err != nil {
		return Config{}, err
	}
	if c.Dev, err = parseBool("GASTOS_DEV", getenv, false); err != nil {
		return Config{}, err
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
		return false, fmt.Errorf("%s inválido: %q", key, v)
	}
	return b, nil
}
