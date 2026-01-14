package distributor

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/mtlprog/mlm"
	"github.com/mtlprog/mlm/config"
	"github.com/mtlprog/mlm/db"
	"github.com/mtlprog/mlm/stellar"
	"github.com/jackc/pgx/v5"
	"github.com/samber/lo"
	"github.com/stellar/go/txnbuild"
)

var ErrNoBalance = errors.New("no balance")
var ErrNoDistributes = errors.New("no distributes: nothing to distribute")

type Distributor struct {
	cfg     *config.Config
	stellar mlm.StellarAgregator
	q       *db.Queries
	pg      *pgx.Conn
}

func (d *Distributor) Distribute(ctx context.Context, opts ...mlm.DistributeOption) (*mlm.DistributeResult, error) {
	opt := &mlm.DistributeOptions{}

	for _, o := range opts {
		o(opt)
	}

	if err := d.q.LockReport(ctx); err != nil {
		return nil, err
	}
	defer func() { _ = d.q.UnlockReport(ctx) }()

	lastDistribute, err := d.getLastDistribute(ctx)
	if err != nil {
		return nil, err
	}

	distributeAmount, err := d.getDistributeAmount(ctx)
	if err != nil {
		return nil, err
	}

	recs, err := d.stellar.Recommenders(ctx)
	if err != nil {
		return nil, err
	}

	res, err := d.CalculateParts(lastDistribute, distributeAmount, recs)
	if err != nil {
		return nil, err
	}

	res.SourceAddress = d.cfg.Address

	res.MissingTrustlines, err = d.checkTrustlines(ctx, res.Distributes)
	if err != nil {
		return nil, err
	}

	if opt.WithoutReport {
		return res, nil
	}

	res.XDR, err = d.getXDR(ctx, res.Distributes)
	if err != nil {
		return nil, err
	}

	res.ReportID, err = d.createReport(ctx, res)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (d *Distributor) getLastDistribute(ctx context.Context) (map[string]map[string]int64, error) {
	rr, err := d.q.GetReports(ctx, 1)
	if err != nil {
		return nil, err
	}

	lastDistribute := map[string]map[string]int64{} // recommender-recommended-mtlap

	if len(rr) > 0 {
		ras, err := d.q.GetReportRecommends(ctx, rr[0].ID)
		if err != nil {
			return nil, err
		}

		for _, ra := range ras {
			if _, ok := lastDistribute[ra.Recommender]; !ok {
				lastDistribute[ra.Recommender] = make(map[string]int64)
			}

			lastDistribute[ra.Recommender][ra.Recommended] = ra.RecommendedMtlap
		}
	}

	return lastDistribute, nil
}

func (d *Distributor) getDistributeAmount(ctx context.Context) (float64, error) {
	balstr, err := d.stellar.Balance(ctx, d.cfg.Address, stellar.LABRAsset, stellar.LABRIssuer)
	if err != nil {
		return 0, err
	}

	bal, err := strconv.ParseFloat(balstr, 64)
	if err != nil {
		return 0, err
	}

	if bal == 0 {
		return 0, ErrNoBalance
	}

	return bal / 3 * 10000000 / 10000000, nil
}

func (d *Distributor) CalculateParts(
	lastDistribute map[string]map[string]int64,
	distributeAmount float64,
	recs *mlm.RecommendersFetchResult,
) (*mlm.DistributeResult, error) {
	res := &mlm.DistributeResult{
		Conflicts:       make([]db.ReportConflict, 0),
		Recommends:      make([]db.ReportRecommend, 0),
		Distributes:     make([]db.ReportDistribute, 0),
		RecommendDeltas: make([]mlm.RecommendDelta, 0),
		CreatedAt:       time.Now(),
		Amount:          distributeAmount,
	}

	// Сначала посчитаем общее количество новых/измененных MTLAP
	totalNewMTLAP := int64(0)

	for _, recommender := range recs.Recommenders {
		for _, recommended := range recommender.Recommended {
			if _, ok := recs.Conflict[recommended.AccountID]; ok {
				continue
			}

			lastMTLAP, ok := lastDistribute[recommender.AccountID][recommended.AccountID]
			if !ok {
				totalNewMTLAP += recommended.MTLAP
				continue
			}
			if lastMTLAP < recommended.MTLAP {
				totalNewMTLAP += recommended.MTLAP - lastMTLAP
			}
		}
	}

	if totalNewMTLAP > 0 {
		res.AmountPerTag = distributeAmount / float64(totalNewMTLAP)
	}

	for recommended, recommenders := range recs.Conflict {
		for _, recoomender := range recommenders {
			res.Conflicts = append(res.Conflicts, db.ReportConflict{
				Recommender: recoomender,
				Recommended: recommended,
			})
		}
	}

	for _, recommender := range recs.Recommenders {
		partCount := int64(0)

		for _, recommended := range recommender.Recommended {
			if _, ok := recs.Conflict[recommended.AccountID]; ok {
				continue
			}

			lastMTLAP, ok := lastDistribute[recommender.AccountID][recommended.AccountID]
			if !ok {
				res.RecommendedNewCount++
			}

			delta := int64(0)
			if lastMTLAP < recommended.MTLAP {
				delta = recommended.MTLAP - lastMTLAP
				partCount += delta
				res.RecommendedLevelUpCount++
			}

			// Добавляем в RecommendDeltas только если есть изменение
			if delta > 0 {
				res.RecommendDeltas = append(res.RecommendDeltas, mlm.RecommendDelta{
					Recommender: recommender.AccountID,
					Recommended: recommended.AccountID,
					Delta:       delta,
				})
			}

			res.Recommends = append(res.Recommends, db.ReportRecommend{
				Recommender:      recommender.AccountID,
				Recommended:      recommended.AccountID,
				RecommendedMtlap: recommended.MTLAP,
			})
		}

		amount := math.Floor(float64(partCount)*res.AmountPerTag*10000000) / 10000000
		if amount > 0 {
			res.Distributes = append(res.Distributes, db.ReportDistribute{
				Recommender: recommender.AccountID,
				Asset:       stellar.LABRAsset,
				Amount:      amount,
			})
		}
	}

	return res, nil
}

func (d *Distributor) getXDR(ctx context.Context, distributes []db.ReportDistribute) (string, error) {
	if len(distributes) == 0 {
		return "", ErrNoDistributes
	}

	accountDetail, err := d.stellar.AccountDetail(d.cfg.Address)
	if err != nil {
		return "", err
	}

	ops := lo.Map(distributes, func(d db.ReportDistribute, _ int) txnbuild.Operation {
		return &txnbuild.Payment{
			Destination: d.Recommender,
			Amount:      fmt.Sprintf("%.7f", d.Amount),
			Asset:       txnbuild.CreditAsset{Code: d.Asset, Issuer: stellar.LABRIssuer},
		}
	})

	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &accountDetail,
		IncrementSequenceNum: true,
		Operations:           ops,
		BaseFee:              1000,
		Memo:                 txnbuild.MemoText(fmt.Sprintf("mlta mlm %s", time.Now().Format(time.DateOnly))),
		Preconditions: txnbuild.Preconditions{
			TimeBounds: txnbuild.NewInfiniteTimeout(),
		},
	})
	if err != nil {
		return "", err
	}

	xdr, err := tx.Base64()
	if err != nil {
		return "", err
	}

	return xdr, err
}

func (d *Distributor) createReport(ctx context.Context, res *mlm.DistributeResult) (int64, error) {
	tx, err := d.pg.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	qtx := d.q.WithTx(tx)

	reportID, err := qtx.CreateReport(ctx, res.XDR)
	if err != nil {
		return 0, err
	}

	for _, recommend := range res.Recommends {
		if err := qtx.CreateReportRecommend(ctx, db.CreateReportRecommendParams{
			ReportID:         reportID,
			Recommender:      recommend.Recommender,
			Recommended:      recommend.Recommended,
			RecommendedMtlap: recommend.RecommendedMtlap,
		}); err != nil {
			return 0, err
		}
	}

	for _, distrib := range res.Distributes {
		if err := qtx.CreateReportDistribute(ctx, db.CreateReportDistributeParams{
			ReportID:    reportID,
			Recommender: distrib.Recommender,
			Asset:       distrib.Asset,
			Amount:      distrib.Amount,
		}); err != nil {
			return 0, err
		}
	}

	for _, conflict := range res.Conflicts {
		if err := qtx.CreateReportConflict(ctx, db.CreateReportConflictParams{
			ReportID:    reportID,
			Recommender: conflict.Recommender,
			Recommended: conflict.Recommended,
		}); err != nil {
			return 0, err
		}
	}

	return reportID, tx.Commit(ctx)
}

func (d *Distributor) checkTrustlines(ctx context.Context, distributes []db.ReportDistribute) ([]mlm.MissingTrustline, error) {
	var missing []mlm.MissingTrustline

	for _, dist := range distributes {
		has, err := d.stellar.HasTrustline(ctx, dist.Recommender, stellar.LABRAsset, stellar.LABRIssuer)
		if err != nil {
			return nil, fmt.Errorf("check trustline for %s: %w", dist.Recommender, err)
		}

		if !has {
			missing = append(missing, mlm.MissingTrustline{
				AccountID: dist.Recommender,
				Asset:     stellar.LABRAsset,
			})
		}
	}

	return missing, nil
}

func New(
	cfg *config.Config,
	stellar mlm.StellarAgregator,
	q *db.Queries,
	pg *pgx.Conn,
) *Distributor {
	return &Distributor{
		cfg:     cfg,
		stellar: stellar,
		q:       q,
		pg:      pg,
	}
}

var _ mlm.Distributor = &Distributor{}
