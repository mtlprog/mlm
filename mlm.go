package mlm

import (
	"context"
	"embed"
	"time"

	"github.com/mtlprog/mlm/db"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/protocols/horizon"
)

//go:embed migrations/*.sql
var EmbedMigrations embed.FS

type Recommended struct {
	AccountID string
	MTLAP     int64
}

type Recommender struct {
	AccountID   string
	Recommended []Recommended
}

type RecommendersFetchResult struct {
	Conflict              map[string][]string // recommended-recommender
	Recommenders          []Recommender
	TotalRecommendedMTLAP int64
}

type StellarAgregator interface {
	Balance(ctx context.Context, accountID, asset, issuer string) (string, error)
	HasTrustline(ctx context.Context, accountID, asset, issuer string) (bool, error)
	Recommenders(ctx context.Context) (*RecommendersFetchResult, error)
	AccountDetail(accountID string) (horizon.Account, error)
}

type HorizonClient interface {
	horizonclient.ClientInterface
}

type MissingTrustline struct {
	AccountID string
	Asset     string
}

type DistributeResult struct {
	CreatedAt               time.Time
	XDR                     string
	Conflicts               []db.ReportConflict
	Recommends              []db.ReportRecommend
	Distributes             []db.ReportDistribute
	MissingTrustlines       []MissingTrustline
	ReportID                int64
	Amount                  float64
	AmountPerTag            float64
	RecommendedNewCount     int64
	RecommendedLevelUpCount int64
	SourceAddress           string
}

type DistributeOptions struct {
	WithoutReport bool
}

type DistributeOption func(*DistributeOptions)

func WithoutReport() DistributeOption {
	return func(o *DistributeOptions) {
		o.WithoutReport = true
	}
}

type Distributor interface {
	Distribute(ctx context.Context, opts ...DistributeOption) (*DistributeResult, error)
}
