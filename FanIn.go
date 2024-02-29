package main

import (
	"context"
	"sync"
)

func FanIn[T any](ctx context.Context, channels ...<-chan T) <-chan T {
	out := make(chan T)

	var wg sync.WaitGroup
	output := func(c <-chan T) {
		defer wg.Done()
		for v := range c {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(channels))
	for _, c := range channels {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
