package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"text/template"
	"time"

	"github.com/antchfx/jsonquery"
	"github.com/pelletier/go-toml/v2"
)

func applyTheme(t string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Printf("Applying theme '%s'", t)
	p := exec.CommandContext(ctx, "lookandfeeltool", "--apply", t)
	if err := p.Run(); err != nil {
		return err
	}

	return nil
}

func readConfig(cf string) (*Config, error) {
	b, err := os.ReadFile(cf)
	if err != nil {
		return nil, err
	}
	cfg := new(Config)
	if err := toml.Unmarshal(b, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

const (
	geocodingURL     = "https://api.openweathermap.org/geo/1.0/direct?q={{.Location}}&limit=1&appid={{.AppID}}"
	sunriseSunsetAPI = "https://api.sunrise-sunset.org/json?lat={{.Lat}}&lng={{.Long}}&date={{.Date}}&tzid={{.Tzid}}"
)

type coordinates struct {
	latitude  float32
	longitude float32
}

type locationName string

func getCoordinatesLocation(loc locationName, apiKey string) (coordinates, error) {
	tmpl, err := template.New("LocationAPI").Parse(geocodingURL)
	if err != nil {
		return coordinates{0, 0}, err
	}

	url := new(bytes.Buffer)
	err = tmpl.Execute(url, struct {
		Location string
		AppID    string
	}{
		string(loc),
		apiKey,
	})
	if err != nil {
		return coordinates{0, 0}, err
	}

	log.Printf("Trying %s", url.String())

	r, err := http.Get(url.String())
	if err != nil {
		return coordinates{0, 0}, err
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		return coordinates{0, 0}, errors.New("error response from OpenWetherMap")
	}

	doc, err := jsonquery.Parse(r.Body)
	if err != nil {
		return coordinates{0, 0}, err
	}

	lat, ok := jsonquery.FindOne(doc, "*[1]/lat").Value().(float64)
	if !ok {
		return coordinates{0, 0}, errors.New("invalid JSON response from OpenWeatherMap")
	}
	lon, ok := jsonquery.FindOne(doc, "*[1]/lon").Value().(float64)
	if !ok {
		return coordinates{0, 0}, errors.New("invalid JSON response from OpenWeatherMap")
	}

	return coordinates{float32(lat), float32(lon)}, nil
}

type Theme int

const (
	light Theme = 1
	dark  Theme = 2
)

type TimeAndTheme struct {
	Start time.Time
	Theme Theme
}

// Before $sunrise, returns $sunrise
// Between $sunrise and $sunset, returns $sunset
// After $sunset, returns $sunrise the next day
//
// Also returns the theme to be set *UNTIL* the returned time
func NextDefaultTime(t time.Time) TimeAndTheme {
	sunrise := 8
	sunset := 18

	if t.Local().Month() >= time.March && t.Local().Month() < time.October {
		sunrise = 6
		sunset = 21

	}

	th := dark
	next := time.Date(t.Year(), t.Month(), t.Day(), sunrise, 0, 0, 0, t.Location())
	if t.Hour() >= sunrise && t.Hour() < sunset {
		next = next.Add(time.Hour * time.Duration(sunset-sunrise))
		th = light
	}
	if t.Hour() >= sunset {
		next = next.AddDate(0, 0, 1)
	}

	return TimeAndTheme{next, th}
}

// exactly as the func above, but uses the Sunset-sunrise API to get the corect times
func GetNextImportantTime(t time.Time, coord *coordinates) TimeAndTheme {
	tmpl, err := template.New("SunsetSunriseAPI").Parse(sunriseSunsetAPI)
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}

	var forCurrentT, forTPlus1 struct {
		Lat  float32
		Long float32
		Date string
		Tzid string
	}

	forCurrentT.Lat = coord.latitude
	forCurrentT.Long = coord.longitude
	forCurrentT.Date = t.Format(time.DateOnly)
	forCurrentT.Tzid = t.Location().String()

	forTPlus1 = forCurrentT
	forTPlus1.Date = t.AddDate(0, 0, 1).Format(time.DateOnly)

	requestForT := new(bytes.Buffer)
	err = tmpl.Execute(requestForT, forCurrentT)
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}

	requestForTPlus1 := new(bytes.Buffer)
	err = tmpl.Execute(requestForTPlus1, forTPlus1)
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}

	client := http.Client{}
	r, err := client.Get(requestForT.String())
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		log.Println("error response from OpenWetherMap")
		return NextDefaultTime(t)
	}

	doc, err := jsonquery.Parse(r.Body)
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}

	const timeLayout = "3:04:05 PM" // go time formatting is **WILD**
	firstLight, err := time.ParseInLocation(timeLayout, jsonquery.FindOne(doc, "/results/civil_twilight_begin").Value().(string), t.Location())
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}
	firstLight = firstLight.AddDate(t.Year(), int(t.Month())-1, t.Day()-1)

	log.Printf("First light on %s is at %s", t.Format(time.DateOnly), firstLight.Format(time.TimeOnly))

	if t.Before(firstLight) {
		return TimeAndTheme{firstLight, dark}
	}

	lastLight, err := time.ParseInLocation(timeLayout, jsonquery.FindOne(doc, "/results/civil_twilight_end").Value().(string), t.Location())
	if err != nil {
		log.Println(err)
		return NextDefaultTime(t)
	}
	lastLight = lastLight.AddDate(t.Year(), int(t.Month())-1, t.Day()-1)

	log.Printf("Last light on %s is at %s", t.Format(time.DateOnly), lastLight.Format(time.TimeOnly))
	if t.Before(lastLight) {
		return TimeAndTheme{lastLight, light}
	}

	// First light for next day
	r, err = client.Get(requestForTPlus1.String())
	if err != nil {
		log.Println(err)
		return TimeAndTheme{firstLight.AddDate(0, 0, 1), dark}
	}

	if r.StatusCode < 200 || r.StatusCode > 299 {
		log.Println("error response from OpenWetherMap")
		return TimeAndTheme{firstLight.AddDate(0, 0, 1), dark}

	}

	doc, err = jsonquery.Parse(r.Body)
	if err != nil {
		log.Println(err)
		return TimeAndTheme{firstLight.AddDate(0, 0, 1), dark}

	}

	firstNextLight, err := time.ParseInLocation(timeLayout, jsonquery.FindOne(doc, "/results/civil_twilight_begin").Value().(string), t.Location())
	if err != nil {
		log.Println(err)
		return TimeAndTheme{firstLight.AddDate(0, 0, 1), dark}
	}
	firstNextLight = firstNextLight.AddDate(t.Year(), int(t.Month())-1, t.Day())

	return TimeAndTheme{firstNextLight, dark}
}

func serviceLoop(cf string, stopChan chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	prevLoc := ""
	coord := coordinates{}

serviceLoopLabel:
	for {
		cfg, err := readConfig(cf)
		if err != nil {
			log.Fatal("Failed to read config file. Bailing")
		}

		if prevLoc != cfg.Location {
			coord, err = getCoordinatesLocation(locationName(cfg.Location), cfg.APIKey)
			if err != nil {
				log.Printf("Failed to get coordinates for %s", cfg.Location)
			} else {
				prevLoc = cfg.Location
			}
		}

		tt := GetNextImportantTime(time.Now(), &coord)

		if tt.Theme == light {
			applyTheme(cfg.DayTheme)
		} else {
			applyTheme(cfg.NightTheme)
		}

		log.Printf("Next time theme will change %s", tt.Start.Format(time.DateTime))

		sleepCh := make(chan struct{})
		go func() {
			time.Sleep(time.Until(tt.Start))
			sleepCh <- struct{}{}
		}()

		select {
		case <-sleepCh:
			continue
		case <-stopChan:
			fmt.Println("Stopping service")
			break serviceLoopLabel
		}
	}
}
