package main

import (
	"github.com/go-co-op/gocron"
	"time"
)

func (c *nicruDNSProviderSolver) cronUpdateToken() {

	s := gocron.NewScheduler(time.UTC)

	s.Every(3).Hours().Do(func() {
		c.getNewTokens()
	})

	s.StartBlocking()

}
