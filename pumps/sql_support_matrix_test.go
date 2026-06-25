package pumps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Verifies: SW-REQ-040
// SW-REQ-040:support_matrix_enforced:negative
// Verifies: SW-REQ-041
// SW-REQ-041:support_matrix_enforced:negative
// Verifies: SW-REQ-042
// SW-REQ-042:support_matrix_enforced:negative
// Verifies: SW-REQ-043
// SW-REQ-043:support_matrix_enforced:negative
// Verifies: SW-REQ-044
// SW-REQ-044:support_matrix_enforced:negative
// Verifies: SW-REQ-045
// SW-REQ-045:support_matrix_enforced:negative
func TestSQLFamilyRejectsSQLiteSupportType(t *testing.T) {
	tests := []struct {
		name string
		init func() error
	}{
		{
			name: "standard_sql",
			init: func() error {
				return (&SQLPump{}).Init(SQLConf{Type: "sqlite", ConnectionString: ":memory:"})
			},
		},
		{
			name: "sql_aggregate",
			init: func() error {
				return (&SQLAggregatePump{}).Init(SQLAggregatePumpConf{
					SQLConf: SQLConf{Type: "sqlite", ConnectionString: ":memory:"},
				})
			},
		},
		{
			name: "graph_sql",
			init: func() error {
				return (&GraphSQLPump{}).Init(GraphSQLConf{
					SQLConf: SQLConf{Type: "sqlite", ConnectionString: ":memory:"},
				})
			},
		},
		{
			name: "graph_sql_aggregate",
			init: func() error {
				return (&GraphSQLAggregatePump{}).Init(SQLAggregatePumpConf{
					SQLConf: SQLConf{Type: "sqlite", ConnectionString: ":memory:"},
				})
			},
		},
		{
			name: "mcp_sql",
			init: func() error {
				return (&MCPSQLPump{}).Init(MCPSQLConf{
					SQLConf: SQLConf{Type: "sqlite", ConnectionString: ":memory:"},
				})
			},
		},
		{
			name: "mcp_sql_aggregate",
			init: func() error {
				return (&MCPSQLAggregatePump{}).Init(SQLAggregatePumpConf{
					SQLConf: SQLConf{Type: "sqlite", ConnectionString: ":memory:"},
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorContains(t, tc.init(), "Unsupported `config_storage.type` value:sqlite")
		})
	}
}
