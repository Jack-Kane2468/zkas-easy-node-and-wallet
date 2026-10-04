package wallet

import (
	"context"
	"fmt"
)

// SelectedHistory reads only the authenticated wallet profile and verifies its
// address before requesting history. It never registers or rescans a wallet.
func (c Client) SelectedHistory(ctx context.Context, expectedAddress string, offset int) (Status, HistoryPage, error) {
	var page HistoryPage
	if offset < 0 {
		return Status{}, page, fmt.Errorf("invalid history page")
	}
	status, err := c.Status(ctx)
	if err != nil {
		return status, page, err
	}
	if status.Loading {
		return status, page, fmt.Errorf("Wallet is opening. Wait a moment, then click Refresh")
	}
	if !status.HasWallet {
		return status, page, fmt.Errorf("This wallet is not registered with the local backend. Close this window and click Refresh / sync first")
	}
	if expectedAddress == "" || status.Address != expectedAddress {
		return status, page, fmt.Errorf("Wallet API identity did not match the selected wallet; history was not loaded")
	}
	page, err = c.HistoryPage(ctx, offset)
	return status, page, err
}
