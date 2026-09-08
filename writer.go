package main

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
)

type counter struct {
	wr    io.Writer
	n     atomic.Uint64
	start time.Time
}

func NewCounter(wr io.Writer) *counter {
	return &counter{
		wr:    wr,
		start: time.Now(),
	}
}

func (c *counter) Write(p []byte) (int, error) {
	n, err := c.wr.Write(p)

	c.n.Add(uint64(n))

	return n, err
}

func (c *counter) Stats() (uint64, time.Duration) {
	return c.n.Load(), time.Since(c.start)
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

				msg := fmt.Sprintf("Written %s (%s/s)", fmtBytes(now), fmtBytes(delta))

				log.Printf("%s%s\r", msg, strings.Repeat(" ", max(0, length-len(msg))))

				length = len(msg)
			case <-done:
				msg := fmt.Sprintf("Wrote %s", fmtBytes(c.n.Load()))

				log.Printf("%s%s\n", msg, strings.Repeat(" ", max(0, length-len(msg))))

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

func humanSpeed(bytes uint64, duration time.Duration) string {
	if duration == 0 {
		return "∞"
	}

	speed := float64(bytes) / duration.Seconds()

	switch {
	case speed >= GiB:
		return fmt.Sprintf("%.2f GiB/s", speed/GiB)
	case speed >= MiB:
		return fmt.Sprintf("%.2f MiB/s", speed/MiB)
	case speed >= KiB:
		return fmt.Sprintf("%.2f KiB/s", speed/KiB)
	default:
		return fmt.Sprintf("%d B/s", int64(speed))
	}
}

func fmtBytes(n uint64) string {
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

func fmtDuration(d time.Duration) string {
	switch {
	case d >= time.Second:
		return d.Round(100 * time.Millisecond).String()
	case d >= time.Millisecond:
		return d.Round(100 * time.Microsecond).String()
	case d >= time.Microsecond:
		return d.Round(100 * time.Nanosecond).String()
	}

	return d.String()
}
