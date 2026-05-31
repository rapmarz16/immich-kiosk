package zmanim

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"charm.land/log/v2"
)

const (
	DefaultSourceURL      = "https://www.myzmanim.com/day.aspx?vars=75405214"
	DefaultRefreshTime    = "01:00"
	DefaultFallbackNeitz  = "5:31 AM"
	DefaultFallbackShkiah = "8:51 PM"
)

type Times struct {
	Date      string `json:"date"`
	Source    string `json:"source"`
	Location  string `json:"location,omitempty"`
	Sunrise   string `json:"sunrise"`
	Sunset    string `json:"sunset"`
	FetchedAt string `json:"fetched_at"`
}

type Config struct {
	SourceURL    string
	CacheFile    string
	RefreshTime  string
	HTTPTimeout  time.Duration
	FallbackData Times
}

type Manager struct {
	mu     sync.RWMutex
	client *http.Client
	config Config
	data   Times
}

func NewManager(config Config) *Manager {
	if config.SourceURL == "" {
		config.SourceURL = DefaultSourceURL
	}
	if config.CacheFile == "" {
		config.CacheFile = DefaultCacheFile()
	}
	if config.RefreshTime == "" {
		config.RefreshTime = DefaultRefreshTime
	}
	if config.HTTPTimeout <= 0 {
		config.HTTPTimeout = 20 * time.Second
	}
	if config.FallbackData.Sunrise == "" {
		config.FallbackData.Sunrise = DefaultFallbackNeitz
	}
	if config.FallbackData.Sunset == "" {
		config.FallbackData.Sunset = DefaultFallbackShkiah
	}
	if config.FallbackData.Date == "" {
		config.FallbackData.Date = time.Now().Local().Format(time.DateOnly)
	}
	config.FallbackData.Source = config.SourceURL

	return &Manager{
		client: &http.Client{Timeout: config.HTTPTimeout},
		config: config,
		data:   config.FallbackData,
	}
}

func DefaultCacheFile() string {
	if info, err := os.Stat("/config"); err == nil && info.IsDir() {
		return "/config/zmanim-cache.json"
	}
	return filepath.Join("config", "zmanim-cache.json")
}

func (m *Manager) Start(ctx context.Context) {
	if cached, err := LoadCache(m.config.CacheFile); err == nil {
		m.set(cached)
	} else if !os.IsNotExist(err) {
		log.Warn("loading zmanim cache", "err", err, "file", m.config.CacheFile)
	}

	go m.refresh(ctx, "startup")
	go m.schedule(ctx)
}

func (m *Manager) Current() Times {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data
}

func (m *Manager) refresh(ctx context.Context, reason string) bool {
	data, err := ScrapeToday(ctx, m.client, m.config.SourceURL)
	if err != nil {
		log.Warn("refreshing zmanim", "reason", reason, "err", err)
		return false
	}
	if data.Date == "" {
		data.Date = time.Now().Local().Format(time.DateOnly)
	}
	data.Source = m.config.SourceURL
	data.FetchedAt = time.Now().Local().Format(time.RFC3339)

	if err := SaveCache(m.config.CacheFile, data); err != nil {
		log.Warn("saving zmanim cache", "err", err, "file", m.config.CacheFile)
	} else {
		log.Info("Updated zmanim cache", "reason", reason, "sunrise", data.Sunrise, "sunset", data.Sunset)
	}
	m.set(data)
	return true
}

func (m *Manager) schedule(ctx context.Context) {
	for {
		delay := DurationUntilNextRefresh(time.Now().Local(), m.config.RefreshTime)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			if !m.refresh(ctx, "scheduled") {
				m.retryUntilSuccess(ctx)
			}
		}
	}
}

func (m *Manager) retryUntilSuccess(ctx context.Context) {
	retryTimer := time.NewTimer(time.Hour)
	defer retryTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-retryTimer.C:
			if m.refresh(ctx, "retry") {
				return
			}
			retryTimer.Reset(time.Hour)
		}
	}
}

func (m *Manager) set(data Times) {
	if data.Sunrise == "" {
		data.Sunrise = m.config.FallbackData.Sunrise
	}
	if data.Sunset == "" {
		data.Sunset = m.config.FallbackData.Sunset
	}
	m.mu.Lock()
	m.data = data
	m.mu.Unlock()
}

func DurationUntilNextRefresh(now time.Time, refreshTime string) time.Duration {
	hour, min, err := parseRefreshTime(refreshTime)
	if err != nil {
		hour, min = 1, 0
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(now)
}

func parseRefreshTime(value string) (int, int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid refresh time %q", value)
	}
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, err
	}
	return t.Hour(), t.Minute(), nil
}

func LoadCache(path string) (Times, error) {
	var data Times
	contents, err := os.ReadFile(path)
	if err != nil {
		return data, err
	}
	if err := json.Unmarshal(contents, &data); err != nil {
		return data, err
	}
	return data, nil
}

func SaveCache(path string, data Times) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, contents, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ScrapeToday(ctx context.Context, client *http.Client, sourceURL string) (Times, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return Times{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return Times{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Times{}, fmt.Errorf("unexpected status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Times{}, err
	}
	return ParseTodayHTML(string(body), sourceURL, time.Now().Local())
}

func ParseTodayHTML(page, sourceURL string, now time.Time) (Times, error) {
	text := htmlToText(page)
	location := parseTitle(page)

	pattern := regexp.MustCompile(`(?is)Today\s+Talis\s+([0-9:]+)\s+Sunrise\s+([0-9:]+).*?Sunset\s+([0-9:]+)`)
	match := pattern.FindStringSubmatch(text)
	if len(match) != 4 {
		return Times{}, fmt.Errorf("could not parse today's sunrise/sunset")
	}

	return Times{
		Date:      now.Format(time.DateOnly),
		Source:    sourceURL,
		Location:  location,
		Sunrise:   addAM(match[2]),
		Sunset:    addPM(match[3]),
		FetchedAt: now.Format(time.RFC3339),
	}, nil
}

func htmlToText(page string) string {
	replacer := regexp.MustCompile(`(?s)<[^>]+>`)
	text := replacer.ReplaceAllString(page, "\n")
	text = html.UnescapeString(text)
	text = regexp.MustCompile(`\n\s*\n+`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func parseTitle(page string) string {
	match := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`).FindStringSubmatch(page)
	if len(match) != 2 {
		return ""
	}
	title := strings.TrimSpace(html.UnescapeString(regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(match[1], "")))
	title = strings.TrimPrefix(title, "Zmanim for ")
	title = strings.TrimSuffix(title, " - MyZmanim.com")
	return title
}

func addAM(value string) string {
	if strings.Contains(strings.ToUpper(value), "AM") || strings.Contains(strings.ToUpper(value), "PM") {
		return value
	}
	return value + " AM"
}

func addPM(value string) string {
	if strings.Contains(strings.ToUpper(value), "AM") || strings.Contains(strings.ToUpper(value), "PM") {
		return value
	}
	return value + " PM"
}
