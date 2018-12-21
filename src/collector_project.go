package main

import (
	"context"
	devopsClient "azure-devops-exporter/src/azure-devops-client"
	"sync"
	"time"
)

type CollectorProject struct {
	CollectorBase

	Processor CollectorProcessorProjectInterface
	Name string
}

func (c *CollectorProject) Run(scrapeTime time.Duration) {
	c.SetScrapeTime(scrapeTime)

	c.Processor.Setup(c)
	go func() {
		for {
			go func() {
				c.Collect()
			}()
			Logger.Verbose("collector[%s]: sleeping %v", c.Name, c.GetScrapeTime().String())
			time.Sleep(*c.GetScrapeTime())
		}
	}()
}

func (c *CollectorProject) Collect() {
	var wg sync.WaitGroup
	var wgCallback sync.WaitGroup

	if c.GetAzureProjects() == nil {
		Logger.Messsage(
			"collector[%s]: no projects found, skipping",
			c.Name,
		)
		return
	}

	ctx := context.Background()

	callbackChannel := make(chan func())

	Logger.Messsage(
		"collector[%s]: starting metrics collection",
		c.Name,
	)

	for _, project := range c.GetAzureProjects().List {
		wg.Add(1)
		go func(ctx context.Context, callback chan<- func(), project devopsClient.Project) {
			defer wg.Done()
			c.Processor.Collect(ctx, callbackChannel, project)
		}(ctx, callbackChannel, project)
	}

	// collect metrics (callbacks) and proceses them
	wgCallback.Add(1)
	go func() {
		defer wgCallback.Done()
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		// reset metric values
		c.Processor.Reset()

		// process callbacks (set metrics)
		for _, callback := range callbackList {
			callback()
		}
	}()

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
	wgCallback.Wait()

	Logger.Verbose(
		"collector[%s]: finished metrics collection",
		c.Name,
	)
}