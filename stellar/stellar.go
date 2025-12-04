package stellar

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/mtlprog/mlm"
	"github.com/samber/lo"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
)

const (
	DefaultLimit      = 20
	MTLAPAsset        = "MTLAP"
	MTLAPIssuer       = "GCNVDZIHGX473FEI7IXCUAEXUJ4BGCKEMHF36VYP5EMS7PX2QBLAMTLA"
	MTLAPAssetRequest = "MTLAP:GCNVDZIHGX473FEI7IXCUAEXUJ4BGCKEMHF36VYP5EMS7PX2QBLAMTLA"
	EURMTLAsset       = "EURMTL"
	EURMTLIssuer      = "GACKTN5DAZGWXRWB2WLM6OPBDHAMT6SJNGLJZPQMEZBUR4JUGBX2UK7V"
	LABRAsset         = "LABR"
	LABRIssuer        = "GA7I6SGUHQ26ARNCD376WXV5WSE7VJRX6OEFNFCEGRLFGZWQIV73LABR"
	TagRecommend      = "RecommendToMTLA"
)

const (
	minMTLAPApplicableRecommender = 4
)

type Client struct {
	cl mlm.HorizonClient
}

func (c *Client) Balance(ctx context.Context, accountID, asset, issuer string) (string, error) {
	acc, err := c.cl.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
	if err != nil {
		return "", err
	}

	return acc.GetCreditBalance(asset, issuer), nil
}

func (c *Client) Recommenders(ctx context.Context) (*mlm.RecommendersFetchResult, error) {
	var allAccounts []horizon.Account
	accp, err := c.cl.Accounts(horizonclient.AccountsRequest{
		Asset: MTLAPAssetRequest,
		Limit: DefaultLimit,
	})
	if err != nil {
		return nil, err
	}
	if len(accp.Embedded.Records) < DefaultLimit {
		return accountsToResult(accp.Embedded.Records), nil
	}

	for {
		allAccounts = append(allAccounts, accp.Embedded.Records...)
		accp, err = c.cl.NextAccountsPage(accp)
		if err != nil {
			return nil, err
		}
		if len(accp.Embedded.Records) == 0 {
			break
		}
	}

	return accountsToResult(allAccounts), nil
}

func (c *Client) AccountDetail(accountID string) (horizon.Account, error) {
	return c.cl.AccountDetail(horizonclient.AccountRequest{
		AccountID: accountID,
	})
}

func (c *Client) SubmitXDR(ctx context.Context, seed, xdr string) (string, error) {
	pair, err := keypair.ParseFull(seed)
	if err != nil {
		return "", err
	}

	txg, err := txnbuild.TransactionFromXDR(xdr)
	if err != nil {
		return "", err
	}

	tx, _ := txg.Transaction()

	tx, err = tx.Sign(network.PublicNetworkPassphrase, pair)
	if err != nil {
		return "", err
	}

	res, err := c.cl.SubmitTransaction(tx)
	if err != nil {
		return "", err
	}

	return res.Hash, nil
}

func NewClient(cl horizonclient.ClientInterface) *Client {
	return &Client{cl: cl}
}

func accountsToResult(accs []horizon.Account) *mlm.RecommendersFetchResult {
	res := &mlm.RecommendersFetchResult{
		Conflict: make(map[string][]string),
	}
	uniqueRecommendeds := make(map[string]struct{})
	lastRecommendedRecommenders := make(map[string]string)

	accMap := lo.Associate(accs, func(acc horizon.Account) (string, horizon.Account) {
		return acc.AccountID, acc
	})

	for _, recommender := range accs {
		recommendedDataMap := lo.PickBy(recommender.Data, func(k, v string) bool {
			return strings.HasPrefix(k, TagRecommend)
		})

		if len(recommendedDataMap) == 0 { // if no recommendeds
			continue
		}

		if !isRecommenderApplicable(recommender) { // if recommender doesn't have enough MTLAP
			continue
		}

		recommendeds := make([]mlm.Recommended, 0, len(recommendedDataMap))
		for _, v := range recommendedDataMap {
			recommended, ok := accMap[decodeBase64(v)]
			if !ok {
				continue
			}

			if _, ok := uniqueRecommendeds[recommended.AccountID]; ok {
				_, ok := res.Conflict[recommended.AccountID]
				if !ok {
					res.Conflict[recommended.AccountID] = append(res.Conflict[recommended.AccountID], lastRecommendedRecommenders[recommended.AccountID])
				}
				res.Conflict[recommended.AccountID] = append(res.Conflict[recommended.AccountID], recommender.AccountID)
			}

			mtlapBalance := getBalanceInt64(recommended, MTLAPAsset, MTLAPIssuer)

			res.TotalRecommendedMTLAP += mtlapBalance

			recommendeds = append(recommendeds, mlm.Recommended{
				AccountID: recommended.AccountID,
				MTLAP:     mtlapBalance,
			})

			lastRecommendedRecommenders[recommended.AccountID] = recommender.AccountID
			uniqueRecommendeds[recommended.AccountID] = struct{}{}
		}

		res.Recommenders = append(res.Recommenders, mlm.Recommender{
			AccountID:   recommender.AccountID,
			Recommended: recommendeds,
		})
	}

	return res
}

func isRecommenderApplicable(acc horizon.Account) bool {
	mtlapBalance, _ := strconv.ParseFloat(acc.GetCreditBalance(MTLAPAsset, MTLAPIssuer), 64)
	return mtlapBalance >= minMTLAPApplicableRecommender
}

func decodeBase64(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	return string(b)
}

func getBalanceInt64(acc horizon.Account, asset, issuer string) int64 {
	balance, err := strconv.ParseFloat(acc.GetCreditBalance(asset, issuer), 64)
	if err != nil {
		return 0
	}
	return int64(balance)
}

var _ mlm.StellarAgregator = &Client{}
