package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/demouth/orenoagent-go"
	"github.com/tectiv3/websearch"
	"github.com/tectiv3/websearch/provider"
)

var Tools = []orenoagent.Tool{
	{
		Name:        "currentTime",
		Description: "Get the current date and time with timezone in a human-readable format.",
		Function: func(_ string) string {
			return time.Now().Format(time.RFC3339)
		},
	},
	{
		// NOTE: This is a sample function. Do not use it in production environments.

		Name:        "webSearch",
		Description: "Get the current date and time with timezone in a human-readable format.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"keyword": map[string]string{
					"type":        "string",
					"description": "web search keyword.",
				},
			},
			"required": []string{"keyword"},
		},
		Function: func(args string) string {
			var param struct {
				Keyword string
			}
			err := json.Unmarshal([]byte(args), &param)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}

			type result struct {
				Title   string
				Link    string
				Snippet string
			}
			results := []result{}
			web := websearch.New(provider.NewUnofficialDuckDuckGo())
			res, err := web.Search(param.Keyword, 10)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}
			for _, ddgor := range res {
				r := result{
					Title:   ddgor.Title,
					Link:    ddgor.Link.String(),
					Snippet: ddgor.Description,
				}
				results = append(results, r)
			}
			v, _ := json.Marshal(results)

			return string(v)
		},
	},
	{
		Name:        "WebReader",
		Description: "Reads and returns the content from the specified URL",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]string{
					"type":        "string",
					"description": "URL of the page to retrieve",
				},
			},
			"required": []string{"url"},
		},
		Function: func(args string) string {
			var param struct {
				Url string
			}
			err := json.Unmarshal([]byte(args), &param)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}

			req, _ := http.NewRequest("GET", param.Url, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}
			defer resp.Body.Close()
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Sprintf("%v", err)
			}

			return string(bodyBytes)
		},
	},
}
