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
	DefaultSourceURL      = "https://r.jina.ai/http://r.jina.ai/http://https://www.khaltoraschesed.com/"
	DefaultRefreshTime    = "01:00"
	DefaultFallbackNeitz  = "5:31 AM"
	DefaultFallbackShkiah = "8:51 PM"
)

type Times struct {
	Date      string      `json:"date"`
	Source    string      `json:"source"`
	Location  string      `json:"location,omitempty"`
	Sunrise   string      `json:"sunrise"`
	Sunset    string      `json:"sunset"`
	Davening  []TimeEntry `json:"davening,omitempty"`
	FetchedAt string      `json:"fetched_at"`
}

type TimeEntry struct {
	Label string `json:"label"`
	Time  string `json:"time"`
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
	if len(config.FallbackData.Davening) == 0 {
		config.FallbackData.Davening = DefaultDaveningTimes()
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
		log.Info("Updated zmanim cache", "reason", reason, "sunrise", data.Sunrise, "sunset", data.Sunset, "davening_times", len(data.Davening))
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
	if len(data.Davening) == 0 {
		data.Davening = m.config.FallbackData.Davening
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

func DefaultDaveningTimes() []TimeEntry {
	return []TimeEntry{
		{Label: "Shachris", Time: "8:00 AM"},
		{Label: "Mincha", Time: "8:30 PM"},
		{Label: "Maariv", Time: "9:00 PM"},
	}
}

func ScrapeToday(ctx context.Context, client *http.Client, sourceURL string) (Times, error) {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return Times{}, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

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
	if data, err := parseKhalTorasChesedHTML(page, sourceURL, now); err == nil {
		return data, nil
	}
	return parseMyZmanimHTML(page, sourceURL, now)
}

func parseMyZmanimHTML(page, sourceURL string, now time.Time) (Times, error) {
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
		Sunrise:   normalizeClockTime(match[2], "AM"),
		Sunset:    normalizeClockTime(match[3], "PM"),
		Davening:  DefaultDaveningTimes(),
		FetchedAt: now.Format(time.RFC3339),
	}, nil
}

func parseKhalTorasChesedHTML(page, sourceURL string, now time.Time) (Times, error) {
	text := htmlToText(page)
	location := parseTitle(page)

	sunrise := findTimeAfterLabels(text, []string{"Neitz", "Netz", "Sunrise"})
	sunset := findTimeAfterLabels(text, []string{"Shkiah", "Shkia", "Sunset"})
	davening := parseDaveningTimes(text)

	if sunrise == "" || sunset == "" || len(davening) == 0 {
		return Times{}, fmt.Errorf("could not parse Khal Toras Chesed zmanim")
	}

	return Times{
		Date:      now.Format(time.DateOnly),
		Source:    sourceURL,
		Location:  location,
		Sunrise:   normalizeClockTime(sunrise, "AM"),
		Sunset:    normalizeClockTime(sunset, "PM"),
		Davening:  davening,
		FetchedAt: now.Format(time.RFC3339),
	}, nil
}

func findTimeAfterLabels(text string, labels []string) string {
	for _, label := range labels {
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(label) + `\b(?:\s*\([^)]*\))?(?:\s+(?:Hachama|HaChama))?\s*[:\-–]?\s*([0-9]{1,2}:[0-9]{2}\s*(?:[ap]m|[AP]M)?)`),
			regexp.MustCompile(`(?is)([0-9]{1,2}:[0-9]{2}\s*(?:[ap]m|[AP]M)?)\s+\b` + regexp.QuoteMeta(label) + `\b(?:\s*\([^)]*\))?(?:\s+(?:Hachama|HaChama))?`),
		}
		for _, pattern := range patterns {
			if match := pattern.FindStringSubmatch(text); len(match) == 2 {
				return match[1]
			}
		}
	}
	return ""
}

func parseDaveningTimes(text string) []TimeEntry {
	section := todaysCalendarSection(text)
	if section == "" {
		section = text
	}
	if entries := parseCalendarDefinitionList(section); len(entries) > 0 {
		return entries
	}

	entries := []TimeEntry{}
	for _, item := range []struct {
		label    string
		patterns []string
	}{
		{label: "Shachris", patterns: []string{"Shachris", "Shacharit", "Shacharis"}},
		{label: "Mincha", patterns: []string{"Mincha"}},
		{label: "Maariv", patterns: []string{"Maariv", "Ma'ariv", "Marriv"}},
	} {
		if value := findDaveningTime(section, item.patterns); value != "" {
			entries = append(entries, TimeEntry{Label: item.label, Time: value})
		}
	}
	return entries
}

func parseCalendarDefinitionList(section string) []TimeEntry {
	lines := strings.Split(section, "\n")
	entries := []TimeEntry{}
	for i := 0; i < len(lines)-1; i++ {
		label := strings.TrimSpace(strings.TrimPrefix(lines[i], "*"))
		value := strings.TrimSpace(lines[i+1])
		if strings.HasPrefix(value, ":") {
			value = strings.TrimSpace(strings.TrimPrefix(value, ":"))
		} else if strings.HasPrefix(label, "*") || strings.Contains(label, ":") {
			continue
		} else {
			continue
		}
		if !isDaveningLabel(label) || strings.Contains(strings.ToLower(label), "plag") {
			continue
		}
		entries = append(entries, TimeEntry{Label: cleanDaveningLabel(label), Time: normalizeClockTime(value, defaultPeriodForLabel(label))})
		i++
	}
	return entries
}

func isDaveningLabel(label string) bool {
	lower := strings.ToLower(label)
	return strings.Contains(lower, "shach") || strings.Contains(lower, "mincha") || strings.Contains(lower, "maariv") || strings.Contains(lower, "marriv")
}

func cleanDaveningLabel(label string) string {
	label = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(label, " "))
	label = strings.ReplaceAll(label, "Marriv", "Maariv")
	return label
}

func defaultPeriodForLabel(label string) string {
	lower := strings.ToLower(label)
	if strings.Contains(lower, "mincha") || strings.Contains(lower, "maariv") || strings.Contains(lower, "marriv") {
		return "PM"
	}
	return "AM"
}

func todaysCalendarSection(text string) string {
	pattern := regexp.MustCompile(`(?is)Today(?:'s)?\s+Calendar(.*?)(?:\*\s*\*\s*\*|Friday\s+Night|Shabbos\s+Day|Tomorrow(?:'s)?\s+Calendar|Upcoming|Full Calendar|Announcements|$)`)
	if match := pattern.FindStringSubmatch(text); len(match) == 2 {
		return match[1]
	}
	return ""
}

func findDaveningTime(section string, labels []string) string {
	for _, label := range labels {
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`(?is)\b` + regexp.QuoteMeta(label) + `\b(?!\s+Hamincha)(?:\s+(?:Gedolah|Ketana))?\s*[:\-–]?\s*([0-9]{1,2}:[0-9]{2}\s*(?:[ap]m|[AP]M)?)`),
			regexp.MustCompile(`(?is)([0-9]{1,2}:[0-9]{2}\s*(?:[ap]m|[AP]M)?)\s+\b` + regexp.QuoteMeta(label) + `\b(?!\s+Hamincha)`),
		}
		matches := [][]string{}
		for _, pattern := range patterns {
			matches = append(matches, pattern.FindAllStringSubmatch(section, -1)...)
		}
		for _, match := range matches {
			line := strings.ToLower(match[0])
			if strings.Contains(line, "plag") || strings.Contains(line, "hamincha") {
				continue
			}
			defaultPeriod := "AM"
			if strings.EqualFold(label, "Mincha") || strings.EqualFold(label, "Maariv") || strings.EqualFold(label, "Ma'ariv") {
				defaultPeriod = "PM"
			}
			return normalizeClockTime(match[1], defaultPeriod)
		}
	}
	return ""
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

func normalizeClockTime(value, defaultPeriod string) string {
	value = strings.TrimSpace(strings.ToUpper(strings.ReplaceAll(value, ".", "")))
	value = regexp.MustCompile(`\s+`).ReplaceAllString(value, " ")
	if strings.HasSuffix(value, "AM") || strings.HasSuffix(value, "PM") {
		return regexp.MustCompile(`\s*(AM|PM)$`).ReplaceAllString(value, " $1")
	}
	return value + " " + defaultPeriod
}
