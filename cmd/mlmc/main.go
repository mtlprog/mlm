package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/Montelibero/mlm"
	"github.com/Montelibero/mlm/config"
	"github.com/Montelibero/mlm/db"
	"github.com/Montelibero/mlm/distributor"
	"github.com/Montelibero/mlm/report"
	"github.com/Montelibero/mlm/stellar"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/jackc/pgx/v5"
	"github.com/stellar/go/clients/horizonclient"
)

func main() {
	ctx := context.Background()

	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})).WithGroup("mlmc")

	cfg := config.Get()

	cl := horizonclient.DefaultPublicNetClient

	pg, err := pgx.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		l.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}
	defer pg.Close(ctx)

	q := db.New(pg)

	stell := stellar.NewClient(cl)
	distrib := distributor.New(cfg, stell, q, pg)

	distribOpts := []mlm.DistributeOption{}

	if cfg.WithoutReport {
		distribOpts = append(distribOpts, mlm.WithoutReport())
	}

	res, err := distrib.Distribute(ctx, distribOpts...)
	if err != nil {
		l.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}

	l = l.With(slog.Int64("report_id", res.ReportID))

	l.InfoContext(ctx, "report done",
		slog.Int("conflicts", len(res.Conflicts)),
		slog.Int("distributes", len(res.Distributes)),
		slog.Int("recommends", len(res.Recommends)),
	)

	b, err := bot.New(cfg.TelegramToken)
	if err != nil {
		l.ErrorContext(ctx, err.Error())
		os.Exit(1)
	}

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		Text:            report.FromDistributeResult(*res),
		ChatID:          cfg.ReportToChatID,
		MessageThreadID: int(cfg.ReportToMessageThreadID),
		ParseMode:       models.ParseModeHTML,
	})
	if err != nil {
		l.ErrorContext(ctx, err.Error(),
			slog.Int64("chat_id", cfg.ReportToChatID),
			slog.Int64("message_thread_id", cfg.ReportToMessageThreadID),
		)
		os.Exit(1)
	}

	if !cfg.Submit {
		return
	}

	hash, err := stell.SubmitXDR(ctx, cfg.Seed, res.XDR)
	if err != nil {
		l.ErrorContext(ctx, "failed to submit xdr")
		os.Exit(1)
	}

	l.InfoContext(ctx, "report xdr submitted",
		slog.String("hash", hash))
}
