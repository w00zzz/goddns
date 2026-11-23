package discovery

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"
)

type sourceResult struct {
	ip  net.IP
	err error
}

// Discover queries all sources concurrently and returns an IP only when the
// configured minimum number of sources agree.
func Discover(ctx context.Context, sources []IPSource, minimumConsensus int) (net.IP, error) {
	return DiscoverWithLogger(ctx, sources, minimumConsensus, nil)
}

func DiscoverWithLogger(ctx context.Context, sources []IPSource, minimumConsensus int, log *slog.Logger) (net.IP, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one IP source is required")
	}
	if minimumConsensus < 1 || minimumConsensus > len(sources) {
		return nil, fmt.Errorf("minimum consensus must be between 1 and %d", len(sources))
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := make(chan sourceResult, len(sources))
	var wg sync.WaitGroup
	wg.Add(len(sources))
	for _, source := range sources {
		go func(source IPSource) {
			defer wg.Done()
			started := time.Now()
			ip, err := source.GetIP(ctx)
			if log != nil {
				if err != nil {
					log.Warn("IP source failed", "source", source.Name(), "error", err, "duration", time.Since(started))
				} else {
					log.Debug("IP source response", "source", source.Name(), "ip", ip.String(), "duration", time.Since(started))
				}
			}
			results <- sourceResult{ip: ip, err: err}
		}(source)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	counts := make(map[string]int)
	for result := range results {
		if result.err != nil || result.ip == nil {
			continue
		}
		key := result.ip.String()
		counts[key]++
		if counts[key] >= minimumConsensus {
			// Continue receiving results so all source goroutines can finish cleanly.
			// The result is returned after the channel has been drained below.
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for ip, count := range counts {
		if count >= minimumConsensus {
			if log != nil {
				log.Info("IP consensus reached", "ip", ip, "votes", count, "required", minimumConsensus)
			}
			return net.ParseIP(ip), nil
		}
	}
	if log != nil {
		log.Warn("IP consensus not reached", "responses", counts, "required", minimumConsensus)
	}
	return nil, fmt.Errorf("IP consensus not reached: minimum %d", minimumConsensus)
}
