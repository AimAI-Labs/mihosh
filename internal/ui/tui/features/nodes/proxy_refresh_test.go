package nodes

import (
	"testing"

	"github.com/AimAI-Labs/mihosh/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestProxyRefreshReappliesDelayAndAvailabilitySorting(t *testing.T) {
	for _, trial := range []struct {
		sort ProxySortOrder
		want []string
	}{
		{SortOrderDelayAsc, []string{"node-B", "node-A", "node-C"}},
		{SortOrderAvailable, []string{"node-B", "node-A"}},
	} {
		groups := map[string]model.Group{"fixture": {All: []string{"node-A", "node-B", "node-C"}}}
		s := State{ProxySortOrder: trial.sort, NodeFilter: "node-", Proxies: map[string]model.Proxy{
			"node-A": {History: []model.Delay{{Delay: 40}}},
		}}
		s = s.ApplyGroups(groups, []string{"fixture"})
		s = s.ApplyProxies(map[string]model.Proxy{
			"node-A": {History: []model.Delay{{Delay: 80}}},
			"node-B": {History: []model.Delay{{Delay: 10}}},
		})
		require.Equal(t, trial.want, s.displayProxies())
		require.Equal(t, "node-A", s.displayProxies()[s.SelectedProxy], "preserve selection by name")
	}
}
