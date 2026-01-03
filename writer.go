package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type counter struct {
	wr io.Writer
	n  atomic.Uint64
}

func NewCounter(wr io.Writer) *counter {
	return &counter{
		wr: wr,
	}
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.wr.Write(p)

	c.n.Add(uint64(n))

	return n, err
}

func (c *counter) Start() func() {
	var (
		wg     sync.WaitGroup
		done   = make(chan struct{})
		ticker = time.NewTicker(1 * time.Second)
	)

	wg.Go(func() {
		var (
			length int
			last   uint64
		)

		for {
			select {
			case <-ticker.C:
				now := c.n.Load()
				delta := now - last

				last = now

				msg := fmt.Sprintf("Written %s (%s)", fmtBytes(now), fmtRate(delta))

				log.Printf("%s%s\r", msg, strings.Repeat(" ", max(0, length-len(msg))))

				length = len(msg)
			case <-done:
				return
			}
		}
	})

	return func() {
		close(done)

		ticker.Stop()

		wg.Wait()
	}
}

func fmtBytes(n uint64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)

	switch {
	case n >= GiB:
		return fmt.Sprintf("%.2f GiB", float64(n)/GiB)
	case n >= MiB:
		return fmt.Sprintf("%.2f MiB", float64(n)/MiB)
	case n >= KiB:
		return fmt.Sprintf("%.2f KiB", float64(n)/KiB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func fmtRate(bytesPerSec uint64) string {
	return fmtBytes(bytesPerSec) + "/s"
}
