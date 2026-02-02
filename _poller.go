package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func PollEvery10s(ctx context.Context, url string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			fmt.Println("poller stopped")
			return

		case <-ticker.C:
			resp, err := client.Get(url)
			if err != nil {
				fmt.Println("GET error:", err)
				continue
			}

			// обязательно закрывать body
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Println("read error:", err)
				continue
			}

			fmt.Println("status:", resp.Status, "body:", string(body))
		}
	}
}
