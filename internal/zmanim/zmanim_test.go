package zmanim

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseTodayHTMLExtractsKhalTorasChesedSunAndDaveningTimes(t *testing.T) {
	html := `<html><head><title>K'hal Toras Chesed</title></head><body>
		<section>
			<h2>Today's Calendar</h2>
			<div>Neitz Hachama: 5:31am</div>
			<div>Shacharis 8:00am</div>
			<div>Plag Hamincha 7:36pm</div>
			<div>Mincha 8:30pm</div>
			<div>Shkiah: 8:51pm</div>
			<div>Maariv 9:00pm</div>
		</section>
		<h2>Tomorrow's Calendar</h2>
		<div>Shacharis 7:00am</div>
	</body></html>`

	got, err := ParseTodayHTML(html, DefaultSourceURL, time.Date(2026, 5, 31, 1, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("ParseTodayHTML returned error: %v", err)
	}

	if got.Sunrise != "5:31 AM" {
		t.Fatalf("Sunrise = %q, want 5:31 AM", got.Sunrise)
	}
	if got.Sunset != "8:51 PM" {
		t.Fatalf("Sunset = %q, want 8:51 PM", got.Sunset)
	}
	wantDavening := []TimeEntry{{Label: "Shachris", Time: "8:00 AM"}, {Label: "Mincha", Time: "8:30 PM"}, {Label: "Maariv", Time: "9:00 PM"}}
	if !reflect.DeepEqual(got.Davening, wantDavening) {
		t.Fatalf("Davening = %#v, want %#v", got.Davening, wantDavening)
	}
}

func TestParseTodayHTMLExtractsMyZmanimSunriseAndSunset(t *testing.T) {
	html := `<html><head><title>Zmanim for Toronto - MyZmanim.com</title></head><body>
		Yesterday Talis 4:35 Sunrise 5:41 Shema 9:27 Shchrs 10:43 Midday 1:15 Sunset 8:50 Nightfall 9:44
		Today Talis 4:34 Sunrise 5:40 Shema 9:27 Shchrs 10:43 Midday 1:15 Sunset 8:51 Nightfall 9:45
		Tomorrow Talis 4:33 Sunrise 5:40 Shema 9:27 Shchrs 10:43 Midday 1:15 Sunset 8:52 Nightfall 9:46
	</body></html>`

	got, err := ParseTodayHTML(html, "https://www.myzmanim.com/day.aspx?vars=75405214", time.Date(2026, 5, 31, 1, 0, 0, 0, time.Local))
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
	want := Times{
		Date:      "2026-05-31",
		Source:    DefaultSourceURL,
		Sunrise:   "5:31 AM",
		Sunset:    "8:51 PM",
		Davening:  []TimeEntry{{Label: "Shachris", Time: "8:00 AM"}, {Label: "Mincha", Time: "8:30 PM"}, {Label: "Maariv", Time: "9:00 PM"}},
		FetchedAt: "2026-05-31T01:00:00-04:00",
	}

	if err := SaveCache(path, want); err != nil {
		t.Fatalf("SaveCache returned error: %v", err)
	}
	got, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache returned error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
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
