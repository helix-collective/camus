// Common helpers to both the client and server

package main

import (
	"fmt"
	"math/rand"
	"time"
)

func NewDeployId() string {
	t := time.Now().UTC()

	return fmt.Sprintf("%s-%s-%d-%02d-%02d-%02d-%02d-%02d",
		ADJECTIVES[rand.Int63()%int64(len(ADJECTIVES))],
		CITIES[rand.Int63()%int64(len(CITIES))],
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute(),
		t.Second(),
	)
}
