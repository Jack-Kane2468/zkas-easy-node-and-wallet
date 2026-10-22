package node

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var checkpointFailure = regexp.MustCompile(`cannot verify checkpoint cursor against selected chain \(([0-9a-f]{64}) lies in peer-supplied history`)

func FailureCursor(line string) string {
	m := checkpointFailure.FindStringSubmatch(line)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// A cursor can belong to several wallets. Never choose one arbitrarily.
func CheckpointOwners(root string) (map[string][]string, error) {
	tokens, err := RegisteredWalletTokens(root)
	if err != nil {
		return nil, err
	}
	owners := map[string][]string{}
	for _, token := range tokens {
		cursor, e := WalletCheckpoint(root, token)
		if e == nil {
			owners[cursor] = append(owners[cursor], token)
		}
	}
	return owners, nil
}
func UniqueCheckpointOwner(root, cursor string) (string, error) {
	owners, err := CheckpointOwners(root)
	if err != nil {
		return "", err
	}
	if len(owners[cursor]) != 1 {
		return "", fmt.Errorf("This log entry cannot be matched uniquely to one current wallet registration. No wallet was rescanned")
	}
	return owners[cursor][0], nil
}
func VerifyRejectedCheckpoint(ctx context.Context, root, token, cursor string, port int) error {
	current, err := WalletCheckpoint(root, token)
	if err != nil || current != cursor {
		return fmt.Errorf("The wallet checkpoint has changed or was removed; no rescan requested")
	}
	rpc, err := NewViewRPC(port)
	if err != nil {
		return err
	}
	defer rpc.Close()
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	_, err = rpc.call(ctx, 1120, 1121, bytesField(1, []byte(cursor)))
	if err == nil || !strings.Contains(err.Error(), "lies in peer-supplied history") {
		return fmt.Errorf("The node did not confirm the checkpoint failure. No rescan requested; check that the node is running")
	}
	return nil
}

// Only repeated failures on distinct timestamps, with the latest in the last
// ten minutes, qualify for the bulk stuck-wallet operation.
func RecentLoopCursors(log string, now time.Time) map[string]bool {
	times := map[string]map[time.Time]bool{}
	for _, line := range strings.Split(log, "\n") {
		cursor := FailureCursor(line)
		if cursor == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		stamp, err := time.Parse("2006-01-02 15:04:05.000-07:00", fields[0]+" "+fields[1])
		if err != nil || stamp.After(now.Add(time.Minute)) || now.Sub(stamp) > 10*time.Minute {
			continue
		}
		if times[cursor] == nil {
			times[cursor] = map[time.Time]bool{}
		}
		times[cursor][stamp] = true
	}
	out := map[string]bool{}
	for c, t := range times {
		if len(t) > 1 {
			out[c] = true
		}
	}
	return out
}
