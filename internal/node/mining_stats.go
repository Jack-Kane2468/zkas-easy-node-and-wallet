package node

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const MiningStatsPort = 18888

type MiningWorker struct {
	Worker   string  `json:"worker"`
	Wallet   string  `json:"wallet"`
	Status   string  `json:"status"`
	Hashrate float64 `json:"hashrate"` // Upstream reports GH/s, estimated from shares.
	Shares   uint64  `json:"shares"`
	Stale    uint64  `json:"stale"`
	Invalid  uint64  `json:"invalid"`
	Blocks   uint64  `json:"blocks"`
}
type MiningBlock struct {
	Worker    string `json:"worker"`
	Hash      string `json:"hash"`
	Timestamp string `json:"timestamp"`
}
type MiningStats struct {
	ActiveWorkers int            `json:"activeWorkers"`
	TotalBlocks   uint64         `json:"totalBlocks"`
	TotalShares   uint64         `json:"totalShares"`
	Workers       []MiningWorker `json:"workers"`
	Blocks        []MiningBlock  `json:"blocks"`
	BridgeUptime  uint64         `json:"bridgeUptime"`
}

func ReadMiningStats(ctx context.Context) (MiningStats, error) {
	var stats MiningStats
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/api/stats", MiningStatsPort), nil)
	if err != nil {
		return stats, err
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return stats, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return stats, fmt.Errorf("statistics returned HTTP %d", res.StatusCode)
	}
	return DecodeMiningStats(res.Body)
}

func DecodeMiningStats(reader io.Reader) (MiningStats, error) {
	var stats MiningStats
	// Required keys distinguish an actual statistics response from unrelated JSON.
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(io.LimitReader(reader, 4<<20)).Decode(&raw); err != nil {
		return stats, err
	}
	for _, key := range []string{"activeWorkers", "totalBlocks", "totalShares", "workers", "blocks"} {
		if _, ok := raw[key]; !ok {
			return stats, fmt.Errorf("bridge statistics missing %s", key)
		}
	}
	b, _ := json.Marshal(raw)
	err := json.Unmarshal(b, &stats)
	return stats, err
}
func (s MiningStats) Display() string {
	rate := 0.0
	for _, w := range s.Workers {
		rate += w.Hashrate
	}
	out := fmt.Sprintf("Active workers: %d    Estimated: %.3f GH/s\r\nShares: %d    Blocks reported: %d    Bridge uptime: %s\r\n", s.ActiveWorkers, rate, s.TotalShares, s.TotalBlocks, (time.Duration(s.BridgeUptime) * time.Second).String())
	for _, w := range s.Workers {
		out += fmt.Sprintf("\r\n%s · %s · %.3f GH/s\r\n  Shares %d | stale %d | invalid %d | blocks %d\r\n  Address: %s\r\n", w.Worker, w.Status, w.Hashrate, w.Shares, w.Stale, w.Invalid, w.Blocks, w.Wallet)
	}
	if len(s.Workers) == 0 {
		out += "\r\nNo workers with recent share activity.\r\n"
	}
	if len(s.Blocks) > 0 {
		out += "\r\nRecent reported blocks:\r\n"
	}
	for i, b := range s.Blocks {
		if i == 10 {
			break
		}
		out += fmt.Sprintf("%s · %s\r\n%s\r\n", b.Timestamp, b.Worker, b.Hash)
	}
	return out
}
