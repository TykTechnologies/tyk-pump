package analytics

import "github.com/TykTechnologies/storage/persistent/model"

// MCPSQLTableName should be defined before SQL migration is called on the MCPRecord.
var MCPSQLTableName string

// MCPRecord is the SQL/MongoDB representation of an MCP analytics record.
// It promotes the identity fields from MCPStats to top-level columns for
// efficient querying while embedding the full AnalyticsRecord for all
// standard analytics dimensions.
type MCPRecord struct {
	JSONRPCMethod            string `json:"jsonrpc_method" bson:"jsonrpc_method" gorm:"column:jsonrpc_method"`
	PrimitiveType            string `json:"primitive_type" bson:"primitive_type" gorm:"column:primitive_type"`
	PrimitiveName            string `json:"primitive_name" bson:"primitive_name" gorm:"column:primitive_name"`
	EffectiveProtocolVersion string `json:"effective_protocol_version" bson:"effective_protocol_version" gorm:"column:effective_protocol_version"`
	DeclaredProtocolVersion  string `json:"declared_protocol_version" bson:"declared_protocol_version" gorm:"column:declared_protocol_version"`
	ProtocolVersionSource    string `json:"protocol_version_source" bson:"protocol_version_source" gorm:"column:protocol_version_source"`

	AnalyticsRecord AnalyticsRecord `bson:",inline" gorm:"embedded;embeddedPrefix:analytics_"`
}

// TableName returns the table/collection name for MCPRecord.
func (m *MCPRecord) TableName() string {
	if MCPSQLTableName == "" {
		return m.AnalyticsRecord.TableName()
	}
	return MCPSQLTableName
}

func (*MCPRecord) GetObjectID() model.ObjectID {
	return ""
}

func (*MCPRecord) SetObjectID(model.ObjectID) {}

// ToMCPRecord converts an AnalyticsRecord to an MCPRecord.
// Returns a zero-value MCPRecord if the record is not an MCP record.
func (a *AnalyticsRecord) ToMCPRecord() MCPRecord {
	if !a.IsMCPRecord() {
		return MCPRecord{}
	}
	return MCPRecord{
		AnalyticsRecord:          *a,
		JSONRPCMethod:            a.MCPStats.JSONRPCMethod,
		PrimitiveType:            a.MCPStats.PrimitiveType,
		PrimitiveName:            a.MCPStats.PrimitiveName,
		EffectiveProtocolVersion: a.MCPStats.EffectiveProtocolVersion,
		DeclaredProtocolVersion:  a.MCPStats.DeclaredProtocolVersion,
		ProtocolVersionSource:    a.MCPStats.ProtocolVersionSource,
	}
}
