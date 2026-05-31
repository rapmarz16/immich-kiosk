package zmanim

import (
	"strings"
	"testing"
	"time"
)

func TestParseTodayHTMLExtractsSunriseAndSunset(t *testing.T) {
	html := `<html><head><title>Zmanim for Toronto - MyZmanim.com</title></head><body>
		Yesterday Talis 4:35 Sunrise 5:41 Shema 9:27 Shchrs 10:43 Midday 1:15 Sunset 8:50 Nightfall 9:44
		Today Talis 4:34 Sunrise 5:40 Shema 9:27 Shchrs 10:43 Midday 1:15 Sunset 8:51 Nightfall 9:45
		Tomorrow Talis 4:33 Sunrise 5:40 Shema 9:27 Shchrs 10:43 Midday 1:15 Sunset 8:52 Nightfall 9:46
	</body></html>`

	got, err := ParseTodayHTML(html, DefaultSourceURL, time.Date(2026, 5, 31, 1, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("ParseTodayHTML returned error: %v", err)
	}

	if got.Location != "Toronto" {
		t.Fatalf("Location = %q, want Toronto", got.Location)
	}
	if got.Sunrise != "5:40 AM" {
		t.Fatalf("Sunrise = %q, want 5:40 AM", got.Sunrise)
	}
	if got.Sunset != "8:51 PM" {
		t.Fatalf("Sunset = %q, want 8:51 PM", got.Sunset)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	path := t.TempDir() + "/zmanim-cache.json"
	want := Times{Date: "2026-05-31", Source: DefaultSourceURL, Sunrise: "5:40 AM", Sunset: "8:51 PM", FetchedAt: "2026-05-31T01:00:00-04:00"}

	if err := SaveCache(path, want); err != nil {
		t.Fatalf("SaveCache returned error: %v", err)
	}
	got, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache returned error: %v", err)
	}
	if got != want {
		t.Fatalf("LoadCache = %#v, want %#v", got, want)
	}
}

func TestDurationUntilNextRefresh(t *testing.T) {
	loc := time.FixedZone("test", -4*60*60)
	now := time.Date(2026, 5, 31, 0, 30, 0, 0, loc)
	if got := DurationUntilNextRefresh(now, "01:00"); got != 30*time.Minute {
		t.Fatalf("DurationUntilNextRefresh before refresh = %s, want 30m", got)
	}

	now = time.Date(2026, 5, 31, 1, 30, 0, 0, loc)
	if got := DurationUntilNextRefresh(now, "01:00"); got != 23*time.Hour+30*time.Minute {
		t.Fatalf("DurationUntilNextRefresh after refresh = %s, want 23h30m", got)
	}
}

func TestParseTodayHTMLErrorsWhenLayoutChanges(t *testing.T) {
	_, err := ParseTodayHTML("<html>no today section</html>", DefaultSourceURL, time.Now())
	if err == nil {
		t.Fatal("ParseTodayHTML expected error for missing today section")
	}
	if !strings.Contains(err.Error(), "could not parse") {
		t.Fatalf("ParseTodayHTML error = %q, want parse error", err.Error())
	}
}
