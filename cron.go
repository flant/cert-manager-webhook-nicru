package main

import (
	"github.com/go-co-op/gocron"
	"time"
)

func (c *nicruDNSProviderSolver) cronUpdateToken() {
	time.Sleep(1 * time.Minute)
	s := gocron.NewScheduler(time.UTC)

	s.Every(3).Hours().Do(func() {
		c.getNewTokens()
	})

	s.StartBlocking()

}
