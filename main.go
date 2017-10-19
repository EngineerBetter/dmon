package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var url = flag.String("url", "", "url to check")
var method = flag.String("method", "GET", "http method")
var token = flag.String("token", "", "authentication token")
var insecure = flag.Bool("k", false, "allow insecure requests")
var timeout = flag.Duration("timeout", 5*time.Second, "connection timeout")
var interval = flag.Duration("interval", 1*time.Second, "interval duration")

type event struct {
	status     bool
	statusCode int
	ts         time.Time
}

func (e event) String() string {
	status := "up"
	if e.status == false {
		status = "down"
	}
	return fmt.Sprintf("Endpoint is %v. Status code: %v. Time %v", status, e.statusCode, e.ts.Format(time.StampMilli))
}

func dmon(ctx context.Context, req *http.Request, interval time.Duration) chan event {
	req = req.WithContext(ctx)
	ticker := time.NewTicker(interval)
	events := make(chan event)
	client := http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: *insecure,
			},
		},
	}
	go func() {
		for {
			select {
			case t := <-ticker.C:
				resp, err := client.Do(req)
				statusCode := 0
				if err == nil {
					statusCode = resp.StatusCode
					resp.Body.Close()
				}
				events <- event{
					err == nil && resp.StatusCode < 400,
					statusCode,
					t,
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return events
}

func main() {
	flag.Parse()
	req, err := http.NewRequest(*method, *url, nil)
	if err != nil {
		log.Fatal(err)
	}
	if *token != "" {
		req.Header.Add("Authorization", "bearer "+*token)
	}
	events := dmon(context.Background(), req, *interval)

	event := <-events
	lastEvent := event
	fmt.Println(event)
	for event := range events {
		if event.status != lastEvent.status {
			if event.status == true {
				fmt.Printf("Experienced %v downtime\n", event.ts.Sub(lastEvent.ts))
			}
			fmt.Println(event)
			lastEvent = event
		}
	}
}
