package service_test

import (
	"nightlight/internal/service"
	"testing"
	"time"
)

// Making sure the Time and Theme are returned correctly
//
// During DST - Light between [8-18], Dark (18-8[+1d])
// After DST - Light between [6-21], Dark (21-6[+1d])
// Notes:
// - First day after DST Light Theme starts at 6:00 -> Means NextTime after 18:00 on 28/29th Feb should be 6am 1st Mar
// - First day during DST Theme starts at 8:00 -> Means NextTime after 21:00 on 31st Oct should be 8am 1st Nov
func TestDefaultThemeAndTime(t *testing.T) {
	londonTz, _ := time.LoadLocation("Europe/London")
	testTimezone(t, londonTz, time.August, 6, 21, 8)
	testTimezone(t, londonTz, time.December, 8, 18, 6)

}

func testTimezone(t *testing.T, loc *time.Location, m time.Month, sunriseHour, sunsetHour, sunriseInNextZone int) {
	now := time.Now()

	// 1st $m this year. at 5:00, 12:00 and 22:00
	beforeSunrise := time.Date(now.Year(), m, 1, 5, 0, 0, 0, loc)
	midDay := time.Date(now.Year(), m, 1, 12, 0, 0, 0, loc)
	afterSunset := time.Date(now.Year(), m, 1, 22, 0, 0, 0, loc)

	tAndT := service.NextDefaultTime(beforeSunrise)
	if tAndT.NextStart.Hour() != sunriseHour {
		t.Fatalf("Expected %d, got %d", sunriseHour, tAndT.NextStart.Hour())
	}
	if tAndT.Theme != service.Dark {
		t.Fatal("Expected dark theme, got light")
	}

	tAndT = service.NextDefaultTime(midDay)
	if tAndT.NextStart.Hour() != sunsetHour {
		t.Fatalf("Expected %d, got %d", sunriseHour, tAndT.NextStart.Hour())
	}
	if tAndT.Theme != service.Light {
		t.Fatal("Expected light theme, got dark")
	}

	tAndT = service.NextDefaultTime(afterSunset)
	if tAndT.NextStart.Hour() != sunriseHour && tAndT.NextStart.Day() == 2 {
		t.Fatalf("Expected %d:00 on %d 2nd, %s", sunriseHour, m, tAndT.NextStart)
	}
	if tAndT.Theme != service.Dark {
		t.Fatal("Expected dark theme, got light")
	}

	_, zoneEnd := midDay.ZoneBounds()
	zoneEnd = zoneEnd.Add(time.Hour * 22)
	tAndT = service.NextDefaultTime(zoneEnd)

	if tAndT.NextStart.Before(zoneEnd) || tAndT.NextStart.Hour() != sunriseInNextZone {
		zoneName, _ := zoneEnd.Zone()
		t.Fatalf("Next start time %s is less than 1 day after %s end", tAndT.NextStart, zoneName)
	}

	if tAndT.NextStart.Hour() != sunriseInNextZone {
		t.Fatalf("Next start hour should be %d, actually got %d", sunriseInNextZone, tAndT.NextStart.Hour())
	}
}
