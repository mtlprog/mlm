package main

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/Montelibero/mlm"
	"github.com/Montelibero/mlm/config"
	"github.com/Montelibero/mlm/db"
	"github.com/Montelibero/mlm/distributor"
	"github.com/Montelibero/mlm/report"
	"github.com/Montelibero/mlm/stellar"
	"github.com/go-telegram/bot"
	"github.com/samber/lo"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/urfave/cli/v3"
)

type app struct {
	cfg     *config.Config
	log     *slog.Logger
	pg      *pgx.Conn
	q       *db.Queries
	stellar *stellar.Client
	distrib *distributor.Distributor
}

func main() {
	ctx := context.Background()

	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).WithGroup("mlmc")

	cfg := config.Get()

	pg, err := pgx.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		l.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}
	defer pg.Close(ctx)

	q := db.New(pg)
	cl := horizonclient.DefaultPublicNetClient
	stell := stellar.NewClient(cl)
	distrib := distributor.New(cfg, stell, q, pg)

	a := &app{
		cfg:     cfg,
		log:     l,
		pg:      pg,
		q:       q,
		stellar: stell,
		distrib: distrib,
	}

	cmd := &cli.Command{
		Name:  "mlmc",
		Usage: "MLM distribution CLI",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "notify-tg",
				Usage: "Send notification to Telegram",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "report",
				Usage: "Report management",
				Commands: []*cli.Command{
					{
						Name:   "dry",
						Usage:  "Generate dry-run report without saving to database",
						Action: a.reportDry,
					},
					{
						Name:   "create",
						Usage:  "Generate and save report to database",
						Action: a.reportCreate,
					},
				},
			},
			{
				Name:   "distribute",
				Usage:  "Submit pending report or create new one and submit",
				Action: a.distribute,
			},
		},
	}

	if err := cmd.Run(ctx, os.Args); err != nil {
		l.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}
}

func (a *app) reportDry(ctx context.Context, cmd *cli.Command) error {
	res, err := a.distrib.Distribute(ctx, mlm.WithoutReport())
	if err != nil {
		return err
	}

	a.log.InfoContext(ctx, "dry report done",
		slog.Int("conflicts", len(res.Conflicts)),
		slog.Int("distributes", len(res.Distributes)),
		slog.Int("recommends", len(res.Recommends)),
		slog.Float64("amount", res.Amount),
		slog.Float64("amount_per_tag", res.AmountPerTag),
	)

	if cmd.Root().Bool("notify-tg") {
		return a.sendTelegramNotification(ctx, res)
	}

	return nil
}

func (a *app) reportCreate(ctx context.Context, cmd *cli.Command) error {
	res, err := a.distrib.Distribute(ctx)
	if err != nil {
		return err
	}

	a.log.InfoContext(ctx, "report created",
		slog.Int64("report_id", res.ReportID),
		slog.Int("conflicts", len(res.Conflicts)),
		slog.Int("distributes", len(res.Distributes)),
		slog.Int("recommends", len(res.Recommends)),
	)

	if cmd.Root().Bool("notify-tg") {
		return a.sendTelegramNotification(ctx, res)
	}

	return nil
}

func (a *app) distribute(ctx context.Context, cmd *cli.Command) error {
	pendingReport, err := a.q.GetPendingReport(ctx)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	var res *mlm.DistributeResult

	if pendingReport.ID != 0 {
		a.log.InfoContext(ctx, "found pending report",
			slog.Int64("report_id", pendingReport.ID),
		)

		res, err = a.buildResultFromReport(ctx, pendingReport)
		if err != nil {
			return err
		}
	} else {
		a.log.InfoContext(ctx, "no pending report found, creating new one")

		res, err = a.distrib.Distribute(ctx)
		if err != nil {
			return err
		}
	}

	hash, err := a.stellar.SubmitXDR(ctx, a.cfg.Seed, res.XDR)
	if err != nil {
		return err
	}

	if err := a.q.SetReportHash(ctx, db.SetReportHashParams{
		Hash:     pgtype.Text{String: hash, Valid: true},
		ReportID: res.ReportID,
	}); err != nil {
		return err
	}

	a.log.InfoContext(ctx, "transaction submitted",
		slog.Int64("report_id", res.ReportID),
		slog.String("hash", hash),
	)

	if cmd.Root().Bool("notify-tg") {
		return a.sendTelegramNotification(ctx, res)
	}

	return nil
}

func (a *app) buildResultFromReport(ctx context.Context, rep db.Report) (*mlm.DistributeResult, error) {
	recommends, err := a.q.GetReportRecommends(ctx, rep.ID)
	if err != nil {
		return nil, err
	}

	distributes, err := a.q.GetReportDistributes(ctx, rep.ID)
	if err != nil {
		return nil, err
	}

	conflicts, err := a.q.GetReportConflicts(ctx, rep.ID)
	if err != nil {
		return nil, err
	}

	return &mlm.DistributeResult{
		ReportID:    rep.ID,
		XDR:         rep.Xdr,
		CreatedAt:   rep.CreatedAt.Time,
		Recommends:  recommends,
		Distributes: distributes,
		Conflicts:   conflicts,
	}, nil
}

func (a *app) sendTelegramNotification(ctx context.Context, res *mlm.DistributeResult) error {
	b, err := bot.New(a.cfg.TelegramToken)
	if err != nil {
		return err
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		Text:            report.FromDistributeResult(lo.FromPtr(res)),
		ChatID:          a.cfg.ReportToChatID,
		MessageThreadID: int(a.cfg.ReportToMessageThreadID),
		ParseMode:       models.ParseModeHTML,
	})

	return err
}
