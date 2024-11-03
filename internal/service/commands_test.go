package service_test

import (
	"nightlight/internal/service"
	"sync"
	"testing"
	"time"
)

func mockThemeAndTime(_ time.Time, _ *service.Coordinates, th service.Theme) func(time.Time, *service.Coordinates) service.TimeAndTheme {
	return func(_ time.Time, _ *service.Coordinates) service.TimeAndTheme {
		return service.TimeAndTheme{time.Now().Add(time.Second * 2), th}
	}
}

// Returns a func to replace service.ThemeApplicator
// Verifies that the expected theme is applied
func themeVerifier(expectedTheme string, t *testing.T) func(string) error {
	return func(th string) error {
		if th != expectedTheme {
			t.Fatalf("expected theme %s, instead applied %s", expectedTheme, th)
		}

		return nil
	}
}

// Making sure the correct Theme is applied at the correct time
func TestServiceLoop(t *testing.T) {
	stopCh := make(chan struct{}, 1)
	var wg sync.WaitGroup
	go service.ServiceLoop("", stopCh, &wg)
}

// Making sure the Time and Theme are returned correctly
//
// (1st Nov - 28th/29th Feb) - Light between [8-18], Dark (18-8[+1d])
// (1st March - 31st Oct) - Light between [6-21], Dark (21-6[+1d])
// Notes:
// - 1st Mar Light Theme starts at 6:00 -> Means NextTime after 18:00 on 28/29th Feb should be 6am 1st Mar
// - 1st Nov Light Theme starts at 8:00 -> Means NextTime after 21:00 on 31st Oct should be 8am 1st Nov
func TestDefaultThemeAndTime(t *testing.T) {

}
