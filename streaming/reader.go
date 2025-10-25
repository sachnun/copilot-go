package streaming

import (
	"bufio"
	"context"
	"net/http"
	"strings"

	"internal/services/copilot"
)

type Reader struct{}

func (Reader) ReadSSE(ctx context.Context, resp *http.Response) (<-chan copilot.SSEMessage, error) {
	ch := make(chan copilot.SSEMessage)

	go func() {
		defer resp.Body.Close()
		defer close(ch)

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		var (
			currentEvent string
			dataBuilder  strings.Builder
		)

		flush := func() {
			if dataBuilder.Len() == 0 {
				return
			}
			select {
			case <-ctx.Done():
				return
			case ch <- copilot.SSEMessage{
				Event: currentEvent,
				Data:  dataBuilder.String(),
			}:
			}
			dataBuilder.Reset()
			currentEvent = ""
		}

		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line := scanner.Text()
			if line == "" {
				flush()
				continue
			}

			if strings.HasPrefix(line, "event:") {
				currentEvent = strings.TrimSpace(line[len("event:"):])
				continue
			}

			if strings.HasPrefix(line, "data:") {
				if dataBuilder.Len() > 0 {
					dataBuilder.WriteByte('\n')
				}
				dataBuilder.WriteString(strings.TrimSpace(line[len("data:"):]))
			}
		}

		flush()
	}()

	return ch, nil
}
