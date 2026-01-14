package distributor_test

import (
	"testing"

	"github.com/mtlprog/mlm"
	"github.com/mtlprog/mlm/distributor"
	"github.com/stretchr/testify/require"
)

func TestCalculateParts(t *testing.T) {
	d := &distributor.Distributor{}

	tests := []struct {
		name             string
		lastDistribute   map[string]map[string]int64
		distributeAmount float64
		recs             *mlm.RecommendersFetchResult
		want             *mlm.DistributeResult
	}{
		{
			name:             "новые MTLAP",
			lastDistribute:   map[string]map[string]int64{},
			distributeAmount: 100,
			recs: &mlm.RecommendersFetchResult{
				Recommenders: []mlm.Recommender{
					{
						AccountID: "rec1",
						Recommended: []mlm.Recommended{
							{AccountID: "user1", MTLAP: 10},
							{AccountID: "user2", MTLAP: 20},
						},
					},
				},
				Conflict: map[string][]string{},
			},
			want: &mlm.DistributeResult{
				AmountPerTag:            100.0 / 30.0, // 100 / (10 + 20)
				RecommendedNewCount:     2,
				RecommendedLevelUpCount: 2,
				RecommendDeltas: []mlm.RecommendDelta{
					{Recommender: "rec1", Recommended: "user1", Delta: 10},
					{Recommender: "rec1", Recommended: "user2", Delta: 20},
				},
			},
		},
		{
			name: "измененные MTLAP",
			lastDistribute: map[string]map[string]int64{
				"rec1": {
					"user1": 5,
					"user2": 10,
				},
			},
			distributeAmount: 100,
			recs: &mlm.RecommendersFetchResult{
				Recommenders: []mlm.Recommender{
					{
						AccountID: "rec1",
						Recommended: []mlm.Recommended{
							{AccountID: "user1", MTLAP: 10}, // +5
							{AccountID: "user2", MTLAP: 20}, // +10
						},
					},
				},
				Conflict: map[string][]string{},
			},
			want: &mlm.DistributeResult{
				AmountPerTag:            100.0 / 15.0, // 100 / (5 + 10)
				RecommendedNewCount:     0,
				RecommendedLevelUpCount: 2,
				RecommendDeltas: []mlm.RecommendDelta{
					{Recommender: "rec1", Recommended: "user1", Delta: 5},
					{Recommender: "rec1", Recommended: "user2", Delta: 10},
				},
			},
		},
		{
			name:             "игнорирование конфликтов",
			lastDistribute:   map[string]map[string]int64{},
			distributeAmount: 100,
			recs: &mlm.RecommendersFetchResult{
				Recommenders: []mlm.Recommender{
					{
						AccountID: "rec1",
						Recommended: []mlm.Recommended{
							{AccountID: "user1", MTLAP: 10},
							{AccountID: "user2", MTLAP: 20},
						},
					},
				},
				Conflict: map[string][]string{
					"user1": {"rec1", "rec2"},
				},
			},
			want: &mlm.DistributeResult{
				AmountPerTag:            100.0 / 20.0, // 100 / 20 (user1 в конфликте)
				RecommendedNewCount:     1,
				RecommendedLevelUpCount: 1,
				RecommendDeltas: []mlm.RecommendDelta{
					{Recommender: "rec1", Recommended: "user2", Delta: 20},
				},
			},
		},
		{
			name: "игнорирование нулевых сумм",
			lastDistribute: map[string]map[string]int64{
				"rec1": {
					"user1": 10,
					"user2": 20,
				},
			},
			distributeAmount: 100,
			recs: &mlm.RecommendersFetchResult{
				Recommenders: []mlm.Recommender{
					{
						AccountID: "rec1",
						Recommended: []mlm.Recommended{
							{AccountID: "user1", MTLAP: 10}, // нет изменений
							{AccountID: "user2", MTLAP: 20}, // нет изменений
						},
					},
				},
				Conflict: map[string][]string{},
			},
			want: &mlm.DistributeResult{
				AmountPerTag:            0,
				RecommendedNewCount:     0,
				RecommendedLevelUpCount: 0,
				RecommendDeltas:         []mlm.RecommendDelta{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.CalculateParts(tt.lastDistribute, tt.distributeAmount, tt.recs)
			require.NoError(t, err)
			require.InDelta(t, tt.want.AmountPerTag, got.AmountPerTag, 0.0001)
			require.Equal(t, tt.want.RecommendedNewCount, got.RecommendedNewCount)
			require.Equal(t, tt.want.RecommendedLevelUpCount, got.RecommendedLevelUpCount)
			require.Equal(t, tt.want.RecommendDeltas, got.RecommendDeltas)
		})
	}
}
