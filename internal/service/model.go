package service

import (
	"context"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	DayTheme   string // as reported by lookandfeeltool
	NightTheme string // as reported by lookandfeeltool
	Location   string
	APIKey     string // OpenWeatherMap API Key
}

func getThemeList() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "lookandfeeltool", "--list").Output()
	if err != nil {
		return []string{}, err
	}

	return slices.DeleteFunc(strings.Split(string(out), "\n"), func(s string) bool {
		return s == ""
	}), nil
}

func FromFile(f string) (*Config, error) {
	b, err := os.ReadFile(f)
	if err != nil {
		return nil, err
	}

	cfg := new(Config)
	err = toml.Unmarshal(b, &cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) ValidateConfig() bool {
	if _, err := getCoordinatesLocation(locationName(c.Location), c.APIKey); err != nil {
		log.Println(err)
		return false
	}

	if themes, err := getThemeList(); err != nil ||
		!slices.Contains(themes, c.DayTheme) || !slices.Contains(themes, c.NightTheme) {
		return false
	}

	return true
}
