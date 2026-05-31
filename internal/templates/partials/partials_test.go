package partials

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/damongolding/immich-kiosk/internal/common"
	"github.com/damongolding/immich-kiosk/internal/config"
	"github.com/damongolding/immich-kiosk/internal/immich"
	"github.com/damongolding/immich-kiosk/internal/zmanim"
)

func TestAssetCameraData(t *testing.T) {
	tests := []struct {
		name string
		exif immich.ExifInfo
		want string
	}{
		{
			name: "Make already in model (whole word)",
			exif: immich.ExifInfo{
				Make:  "Canon",
				Model: "Canon EOS 5D Mark IV",
			},
			want: "Canon EOS 5D Mark IV",
		},
		{
			name: "Make not in model",
			exif: immich.ExifInfo{
				Make:  "Canon",
				Model: "EOS 5D Mark IV",
			},
			want: "Canon EOS 5D Mark IV",
		},
		{
			name: "Make is substring but not whole word",
			exif: immich.ExifInfo{
				Make:  "Canon",
				Model: "Canonic EOS 5D Mark IV",
			},
			want: "Canon Canonic EOS 5D Mark IV",
		},
		{
			name: "Extra whitespace trimmed",
			exif: immich.ExifInfo{
				Make:  " Canon ",
				Model: " EOS 5D Mark IV ",
			},
			want: "Canon EOS 5D Mark IV",
		},
		{
			name: "Empty make",
			exif: immich.ExifInfo{
				Make:  "",
				Model: "EOS 5D Mark IV",
			},
			want: "EOS 5D Mark IV",
		},
		{
			name: "Empty model",
			exif: immich.ExifInfo{
				Make:  "Canon",
				Model: "",
			},
			want: "Canon",
		},
		{
			name: "Both empty",
			exif: immich.ExifInfo{
				Make:  "",
				Model: "",
			},
			want: "",
		},
		{
			name: "Case insensitive whole word match",
			exif: immich.ExifInfo{
				Make:  "canon",
				Model: "Canon EOS 5D Mark IV",
			},
			want: "Canon EOS 5D Mark IV",
		},
		{
			name: "My camera",
			exif: immich.ExifInfo{
				Make:  "NIKON CORPORATION",
				Model: "NIKON D90",
			},
			want: "NIKON CORPORATION NIKON D90",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssetCameraData(tt.exif)
			if got != tt.want {
				t.Errorf("AssetCameraData() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Unit test for trimFloatToString
func TestTrimFloatToString(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{123.456, "123.46"},
		{123.400, "123.4"},
		{123.004, "123"},
		{123.000, "123"},
		{0.004, "0"},
		{0.0, "0"},
		{-123.456, "-123.46"},
		{-123.400, "-123.4"},
		{-123.000, "-123"},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%f", test.input), func(t *testing.T) {
			result := trimFloatToString(test.input)
			if result != test.expected {
				t.Errorf("trimFloatToString(%f) = %s; expected %s", test.input, result, test.expected)
			}
		})
	}
}

func TestDaveningTimesRendersDynamicSunTimesAndHardcodedSchedule(t *testing.T) {
	var rendered bytes.Buffer

	err := DaveningTimes(common.ViewData{
		ZmanimTimes: zmanim.Times{
			Sunrise:  "5:40 AM",
			Sunset:   "8:51 PM",
			Davening: []zmanim.TimeEntry{{Label: "Shachris", Time: "8:00 AM"}, {Label: "Mincha", Time: "8:30 PM"}, {Label: "Maariv", Time: "9:00 PM"}},
		},
		Config: config.Config{Zmanim: config.ZmanimConfig{
			Enabled:            true,
			ShowSunTimes:       true,
			ShowDaveningTimes:  true,
		}},
	}).Render(context.Background(), &rendered)
	if err != nil {
		t.Fatalf("DaveningTimes render failed: %v", err)
	}

	html := rendered.String()
	for _, want := range []string{
		`id="davening-times"`,
		"Today&#39;s Zmanim",
		"🌄",
		"Neitz",
		"5:40 AM",
		"🌅",
		"Shachris",
		"8:00 AM",
		"☀️",
		"Mincha",
		"8:30 PM",
		"🌇",
		"Shkiah",
		"8:51 PM",
		"🌙",
		"Maariv",
		"9:00 PM",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("DaveningTimes() output missing %q in %s", want, html)
		}
	}
}

func TestDaveningTimesCanHideSunTimesIndependently(t *testing.T) {
	var rendered bytes.Buffer

	err := DaveningTimes(common.ViewData{
		ZmanimTimes: zmanim.Times{
			Sunrise:  "5:40 AM",
			Sunset:   "8:51 PM",
			Davening: []zmanim.TimeEntry{{Label: "Shachris", Time: "8:00 AM"}, {Label: "Mincha", Time: "8:30 PM"}, {Label: "Maariv", Time: "9:00 PM"}},
		},
		Config: config.Config{Zmanim: config.ZmanimConfig{
			Enabled:            true,
			ShowSunTimes:       false,
			ShowDaveningTimes:  true,
		}},
	}).Render(context.Background(), &rendered)
	if err != nil {
		t.Fatalf("DaveningTimes render failed: %v", err)
	}

	html := rendered.String()
	for _, hidden := range []string{"Neitz", "Shkiah", "5:40 AM", "8:51 PM"} {
		if strings.Contains(html, hidden) {
			t.Fatalf("DaveningTimes() output unexpectedly contained %q in %s", hidden, html)
		}
	}
	for _, want := range []string{"Shachris", "Mincha", "Maariv"} {
		if !strings.Contains(html, want) {
			t.Fatalf("DaveningTimes() output missing %q in %s", want, html)
		}
	}
}
