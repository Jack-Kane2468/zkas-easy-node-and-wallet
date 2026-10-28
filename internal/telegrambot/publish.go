package telegrambot

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type apiError struct {
	method             string
	status, code       int
	unchanged, missing bool
}

func (e *apiError) Error() string {
	return fmt.Sprintf("Telegram API rejected %s (HTTP %d, code %d)", e.method, e.status, e.code)
}
func (a *API) Publish(ctx context.Context, id, mid int64, reply Reply, k [][]Button) (int64, error) {
	text := []rune(reply.Text)
	if len(text) > 3500 {
		text = append(text[:3450], []rune("\n…More detail is available in the manager.")...)
	}
	k = append(copyButtons(string(text)), k...)
	if k == nil {
		k = [][]Button{}
	}
	body := map[string]any{"chat_id": id, "text": string(text), "reply_markup": map[string]any{"inline_keyboard": k}}
	if mid > 0 {
		body["message_id"] = mid
		err := a.call(ctx, "editMessageText", body, nil)
		if err == nil {
			return mid, nil
		}
		var ae *apiError
		if !errors.As(err, &ae) {
			return mid, err
		}
		if ae.unchanged {
			return mid, nil
		}
		if !ae.missing {
			return mid, err
		}
		delete(body, "message_id")
	}
	var result struct {
		ID int64 `json:"message_id"`
	}
	err := a.call(ctx, "sendMessage", body, &result)
	return result.ID, err
}

var copyValue = regexp.MustCompile(`(?:kaspa|zkas):[a-zA-Z0-9]+|\b[0-9a-fA-F]{64}\b`)

func copyButtons(text string) [][]Button {
	var rows [][]Button
	seen := map[string]bool{}
	for _, v := range copyValue.FindAllString(text, -1) {
		if seen[v] || len(v) > 256 {
			continue
		}
		seen[v] = true
		label := "Copy transaction ID"
		if strings.Contains(v, ":") {
			label = "Copy address"
		}
		rows = append(rows, []Button{{Text: label, CopyText: &CopyText{Text: v}}})
		if len(rows) == 6 {
			break
		}
	}
	return rows
}
