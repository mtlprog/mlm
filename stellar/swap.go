package stellar

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/txnbuild"
)

// SwappableToken represents a token that can be swapped to LABR
type SwappableToken struct {
	Code   string
	Issuer string
}

// SwappableTokens is a list of tokens that can be swapped to LABR
// Add new tokens here as needed
var SwappableTokens = []SwappableToken{
	{Code: EURMTLAsset, Issuer: EURMTLIssuer},
}

// SwapResult represents the result of a single swap operation
type SwapResult struct {
	FromAsset    string
	FromAmount   float64
	ToAsset      string
	ToAmount     float64
	TxHash       string
	PricePerLABR float64
}

// SwapSummary represents the summary of all swap operations
type SwapSummary struct {
	Swaps          []SwapResult
	PriceExceeded  []PriceExceededAlert
	Errors         []SwapError
	TotalFromEUR   float64
	TotalToLABR    float64
}

// SwapError represents an error during swap
type SwapError struct {
	Asset string
	Stage string
	Error string
}

// PriceExceededAlert represents an alert when price threshold is exceeded
type PriceExceededAlert struct {
	FromAsset    string
	FromAmount   float64
	PricePerLABR float64
	Threshold    float64
}

// TokenBalance represents a balance of a specific token
type TokenBalance struct {
	Code    string
	Issuer  string
	Balance float64
}

// GetSwappableBalances returns balances of all swappable tokens for the given account
func (c *Client) GetSwappableBalances(ctx context.Context, accountID string) ([]TokenBalance, error) {
	acc, err := c.cl.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get account detail: %w", err)
	}

	var balances []TokenBalance
	for _, token := range SwappableTokens {
		balStr := acc.GetCreditBalance(token.Code, token.Issuer)
		if balStr == "" {
			continue // no trustline or zero balance
		}
		bal, err := strconv.ParseFloat(balStr, 64)
		if err != nil {
			continue
		}
		if bal > 0 {
			balances = append(balances, TokenBalance{
				Code:    token.Code,
				Issuer:  token.Issuer,
				Balance: bal,
			})
		}
	}

	return balances, nil
}

// GetSwapPrice returns the price of 1 unit of source asset in destination asset terms
// Returns destAmount for 1 unit of sourceAsset
func (c *Client) GetSwapPrice(
	ctx context.Context,
	sourceCode, sourceIssuer string,
	destCode, destIssuer string,
) (float64, error) {
	pr := horizonclient.StrictSendPathsRequest{
		SourceAmount:      "1",
		SourceAssetCode:   sourceCode,
		SourceAssetIssuer: sourceIssuer,
		SourceAssetType:   horizonclient.AssetType4,
		DestinationAssets: fmt.Sprintf("%s:%s", destCode, destIssuer),
	}

	// Use public net client directly for path finding
	paths, err := horizonclient.DefaultPublicNetClient.StrictSendPaths(pr)
	if err != nil {
		return 0, err
	}

	if len(paths.Embedded.Records) == 0 {
		return 0, fmt.Errorf("no path found for %s -> %s", sourceCode, destCode)
	}

	// Get the best path (first one)
	bestPath := paths.Embedded.Records[0]
	destAmount, err := strconv.ParseFloat(bestPath.DestinationAmount, 64)
	if err != nil {
		return 0, err
	}

	return destAmount, nil
}

// SwapToLABR swaps the given amount of source asset to LABR
func (c *Client) SwapToLABR(
	ctx context.Context,
	seed string,
	sourceCode, sourceIssuer string,
	amount float64,
	minDestAmount float64,
) (string, float64, error) {
	pair, err := keypair.ParseFull(seed)
	if err != nil {
		return "", 0, err
	}

	accountDetail, err := c.cl.AccountDetail(horizonclient.AccountRequest{
		AccountID: pair.Address(),
	})
	if err != nil {
		return "", 0, err
	}

	// Get the best path first to determine intermediate assets
	pr := horizonclient.StrictSendPathsRequest{
		SourceAmount:      fmt.Sprintf("%.7f", amount),
		SourceAssetCode:   sourceCode,
		SourceAssetIssuer: sourceIssuer,
		SourceAssetType:   horizonclient.AssetType4,
		DestinationAssets: fmt.Sprintf("%s:%s", LABRAsset, LABRIssuer),
	}

	// Use public net client directly for path finding
	paths, err := horizonclient.DefaultPublicNetClient.StrictSendPaths(pr)
	if err != nil {
		return "", 0, err
	}

	if len(paths.Embedded.Records) == 0 {
		return "", 0, fmt.Errorf("no path found for swap")
	}

	bestPath := paths.Embedded.Records[0]
	destAmount, err := strconv.ParseFloat(bestPath.DestinationAmount, 64)
	if err != nil {
		return "", 0, err
	}

	// Build path assets
	var pathAssets []txnbuild.Asset
	for _, pa := range bestPath.Path {
		if pa.Type == "native" {
			pathAssets = append(pathAssets, txnbuild.NativeAsset{})
		} else {
			pathAssets = append(pathAssets, txnbuild.CreditAsset{
				Code:   pa.Code,
				Issuer: pa.Issuer,
			})
		}
	}

	op := &txnbuild.PathPaymentStrictSend{
		SendAsset:   txnbuild.CreditAsset{Code: sourceCode, Issuer: sourceIssuer},
		SendAmount:  fmt.Sprintf("%.7f", amount),
		Destination: pair.Address(),
		DestAsset:   txnbuild.CreditAsset{Code: LABRAsset, Issuer: LABRIssuer},
		DestMin:     fmt.Sprintf("%.7f", minDestAmount),
		Path:        pathAssets,
	}

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &accountDetail,
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{op},
		BaseFee:              1000,
		Memo:                 txnbuild.MemoText(fmt.Sprintf("swap %s->LABR %s", sourceCode, time.Now().Format(time.DateOnly))),
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewTimeout(300),
		},
	})
	if err != nil {
		return "", 0, err
	}

	tx, err = tx.Sign(network.PublicNetworkPassphrase, pair)
	if err != nil {
		return "", 0, err
	}

	res, err := c.cl.SubmitTransaction(tx)
	if err != nil {
		return "", 0, err
	}

	return res.Hash, destAmount, nil
}

// ExecuteSwaps executes swaps for all swappable tokens to LABR
func (c *Client) ExecuteSwaps(
	ctx context.Context,
	accountID, seed string,
	priceThreshold float64,
) (*SwapSummary, error) {
	summary := &SwapSummary{
		Swaps:         make([]SwapResult, 0),
		PriceExceeded: make([]PriceExceededAlert, 0),
		Errors:        make([]SwapError, 0),
	}

	balances, err := c.GetSwappableBalances(ctx, accountID)
	if err != nil {
		return nil, err
	}

	for _, bal := range balances {
		// Get price: how much LABR for 1 unit of source asset
		labrPerUnit, err := c.GetSwapPrice(ctx, bal.Code, bal.Issuer, LABRAsset, LABRIssuer)
		if err != nil {
			summary.Errors = append(summary.Errors, SwapError{
				Asset: bal.Code,
				Stage: "get_price",
				Error: err.Error(),
			})
			continue
		}

		// Price per LABR in source asset terms
		// If 1 EURMTL = 0.05 LABR, then 1 LABR = 20 EURMTL
		pricePerLABR := 1 / labrPerUnit

		if pricePerLABR > priceThreshold {
			summary.PriceExceeded = append(summary.PriceExceeded, PriceExceededAlert{
				FromAsset:    bal.Code,
				FromAmount:   bal.Balance,
				PricePerLABR: pricePerLABR,
				Threshold:    priceThreshold,
			})
			continue
		}

		// Calculate minimum destination amount (with 1% slippage)
		expectedLABR := bal.Balance * labrPerUnit
		minDestAmount := expectedLABR * 0.99

		hash, actualAmount, err := c.SwapToLABR(ctx, seed, bal.Code, bal.Issuer, bal.Balance, minDestAmount)
		if err != nil {
			summary.Errors = append(summary.Errors, SwapError{
				Asset: bal.Code,
				Stage: "swap",
				Error: err.Error(),
			})
			continue
		}

		result := SwapResult{
			FromAsset:    bal.Code,
			FromAmount:   bal.Balance,
			ToAsset:      LABRAsset,
			ToAmount:     actualAmount,
			TxHash:       hash,
			PricePerLABR: pricePerLABR,
		}

		summary.Swaps = append(summary.Swaps, result)
		summary.TotalFromEUR += bal.Balance
		summary.TotalToLABR += actualAmount
	}

	return summary, nil
}

// FormatSwapReport formats the swap summary for Telegram notification
func FormatSwapReport(summary *SwapSummary) string {
	if len(summary.Swaps) == 0 {
		return ""
	}

	report := "<b>Token Swap Report</b>\n\n"

	for _, swap := range summary.Swaps {
		report += fmt.Sprintf("%.2f %s -> %.2f %s\n", swap.FromAmount, swap.FromAsset, swap.ToAmount, swap.ToAsset)
		report += fmt.Sprintf("Price: 1 LABR = %.2f %s\n", swap.PricePerLABR, swap.FromAsset)
		report += fmt.Sprintf("TX: %s\n\n", swap.TxHash[:8]+"..."+swap.TxHash[len(swap.TxHash)-8:])
	}

	report += fmt.Sprintf("<b>Total:</b> %.2f EURMTL -> %.2f LABR", summary.TotalFromEUR, summary.TotalToLABR)

	return report
}

// FormatPriceAlert formats the price exceeded alert for Telegram notification
func FormatPriceAlert(alerts []PriceExceededAlert, mentionUsername string) string {
	if len(alerts) == 0 {
		return ""
	}

	report := fmt.Sprintf("<b>Price Alert</b> @%s\n\n", mentionUsername)

	for _, alert := range alerts {
		report += fmt.Sprintf("Cannot swap %.2f %s\n", alert.FromAmount, alert.FromAsset)
		report += fmt.Sprintf("Current price: 1 LABR = %.2f %s\n", alert.PricePerLABR, alert.FromAsset)
		report += fmt.Sprintf("Threshold: %.2f %s\n\n", alert.Threshold, alert.FromAsset)
	}

	return strings.TrimRight(report, "\n")
}
