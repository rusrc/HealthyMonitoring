package main

import (
	"context"
	"io"
	"net/http"
	"time"
)

type PollResult struct {
	Time       time.Time
	StatusCode int
	Body       []byte
	Err        error
}

func StartPoller(ctx context.Context, url string, every time.Duration) <-chan PollResult {
	out := make(chan PollResult, 16) // буфер чтобы не стопорить poller

	go func() {
		defer close(out)

		ticker := time.NewTicker(every)
		defer ticker.Stop()

		client := &http.Client{Timeout: 5 * time.Second}

		for {
			select {
			case <-ctx.Done():
				return

			case t := <-ticker.C:
				res := PollResult{Time: t}

				resp, err := client.Get(url)
				if err != nil {
					res.Err = err
					out <- res
					continue
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()

				if err != nil {
					res.Err = err
					out <- res
					continue
				}

				res.StatusCode = resp.StatusCode
				res.Body = body
				out <- res
			}
		}
	}()

	return out
}
