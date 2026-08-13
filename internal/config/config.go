package config

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Host           string
	Port           int
	DBPath         string
	MasterKey      []byte
	ListenHostSet  bool
	ListenHostFrom string
}

var validListenHosts = map[string]struct{}{
	"all":       {},
	"0.0.0.0":   {},
	"::":        {},
	"127.0.0.1": {},
	"::1":       {},
}

func Load() (Config, error) {
	return LoadWithArgs(os.Args[1:])
}

func LoadWithArgs(args []string) (Config, error) {
	cfg := Config{
		Port:   8067,
		DBPath: "./carryapi.db",
	}

	fs := flag.NewFlagSet("carryapi", flag.ContinueOnError)
	flagHost := fs.String("host", "", "listen host")
	flagPort := fs.Int("port", 0, "listen port")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if *flagPort != 0 {
		cfg.Port = *flagPort
	}
	if *flagHost != "" {
		if _, ok := validListenHosts[*flagHost]; !ok {
			return Config{}, fmt.Errorf("invalid --host %q", *flagHost)
		}
		cfg.Host = *flagHost
		cfg.ListenHostSet = true
		cfg.ListenHostFrom = "flag"
	} else if v := os.Getenv("CARRYAPI_HOST"); v != "" {
		if _, ok := validListenHosts[v]; !ok {
			return Config{}, fmt.Errorf("invalid CARRYAPI_HOST %q", v)
		}
		cfg.Host = v
		cfg.ListenHostSet = true
		cfg.ListenHostFrom = "env"
	}
	if v := os.Getenv("CARRYAPI_PORT"); v != "" && *flagPort == 0 {
		p, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid CARRYAPI_PORT: %w", err)
		}
		cfg.Port = p
	}
	if v := os.Getenv("CARRYAPI_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	key, err := loadMasterKey()
	if err != nil {
		return Config{}, err
	}
	cfg.MasterKey = key
	return cfg, nil
}

func loadMasterKey() ([]byte, error) {
	if v := os.Getenv("CARRYAPI_MASTER_KEY"); v != "" {
		if len(v) != 32 {
			return nil, errors.New("CARRYAPI_MASTER_KEY must be 32 bytes")
		}
		return []byte(v), nil
	}
	keyFile := os.Getenv("CARRYAPI_KEY_FILE")
	if keyFile == "" {
		keyFile = "carryapi.key"
	}
	if data, err := os.ReadFile(keyFile); err == nil {
		if len(data) != 32 {
			return nil, fmt.Errorf("%s must be 32 bytes", keyFile)
		}
		return data, nil
	}
	// 生成新密钥
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate master key: %w", err)
	}
	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		return nil, fmt.Errorf("write %s: %w", keyFile, err)
	}
	fmt.Printf("generated new master key at %s (keep it safe)\n", keyFile)
	return key, nil
}
