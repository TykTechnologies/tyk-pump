package analytics

import "github.com/TykTechnologies/storage/persistent/model"

// MCPSQLTableName should be defined before SQL migration is called on the MCPRecord.
var MCPSQLTableName string

// MCPRecord is the SQL/MongoDB representation of an MCP analytics record.
// It promotes the identity fields from MCPStats to top-level columns for
// efficient querying while embedding the full AnalyticsRecord for all
// standard analytics dimensions.
type MCPRecord struct {
	JSONRPCMethod string `gorm:"column:jsonrpc_method"`
	PrimitiveType string `gorm:"column:primitive_type"`
	PrimitiveName string `gorm:"column:primitive_name"`

	AnalyticsRecord AnalyticsRecord `bson:",inline" gorm:"embedded;embeddedPrefix:analytics_"`
}

// TableName returns the table/collection name for MCPRecord.
func (m *MCPRecord) TableName() string {
	if MCPSQLTableName == "" {
		return m.AnalyticsRecord.TableName()
	}
	return MCPSQLTableName
}

// GetObjectID satisfies the model.DBObject interface.
func (*MCPRecord) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID satisfies the model.DBObject interface.
func (*MCPRecord) SetObjectID(model.ObjectID) {}

// ToMCPRecord converts an AnalyticsRecord to an MCPRecord.
// Returns a zero-value MCPRecord if the record is not an MCP record.
func (a *AnalyticsRecord) ToMCPRecord() MCPRecord {
	if !a.IsMCPRecord() {
		return MCPRecord{}
	}
	return MCPRecord{
		AnalyticsRecord: *a,
		JSONRPCMethod:   a.MCPStats.JSONRPCMethod,
		PrimitiveType:   a.MCPStats.PrimitiveType,
		PrimitiveName:   a.MCPStats.PrimitiveName,
	}
}
