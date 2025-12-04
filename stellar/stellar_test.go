package stellar_test

import (
	"context"
	"testing"

	"github.com/mtlprog/mlm/stellar"
	"github.com/davecgh/go-spew/spew"
	"github.com/stellar/go/clients/horizonclient"
	"github.com/stretchr/testify/require"
)

func TestClient_Balance(t *testing.T) {
	t.Skip(t)

	ctx := context.Background()
	cl := stellar.NewClient(horizonclient.DefaultPublicNetClient) // TODO: use mock

	balance, err := cl.Balance(ctx, stellar.MTLAPIssuer, stellar.MTLAPAsset, stellar.MTLAPIssuer)
	require.NoError(t, err)
	require.NotEmpty(t, balance)
}

func TestClient_Fetch(t *testing.T) {
	t.Skip(t)

	ctx := context.Background()
	cl := stellar.NewClient(horizonclient.DefaultPublicNetClient) // TODO: use mock

	res, err := cl.Recommenders(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, res)
	require.NotEmpty(t, res.Recommenders)
	require.NotEmpty(t, res.TotalRecommendedMTLAP)
	require.NotEmpty(t, res.Conflict)

	spew.Dump(res.Conflict)
}
