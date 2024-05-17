package analytics

import (
	"github.com/TykTechnologies/storage/persistent/model"
)

// GraphSQLTableName should be defined before SQL migration is called on the GraphRecord
// the reason this approach is used to define the table name is due to gorm's inability to
// read values from the fields of the GraphRecord/AnalyticsRecord struct when it is migrating, due to that
// a single static value is going to be returned as TableName and it will be used as the prefix for index/relationship creation no matter the
// value passed to db.Table()
var GraphSQLTableName string

type GraphRecord struct {
	Types map[string][]string `gorm:"types"`

	AnalyticsRecord AnalyticsRecord `bson:",inline" gorm:"embedded;embeddedPrefix:analytics_"`

	OperationType string       `gorm:"column:operation_type"`
	Variables     string       `gorm:"variables"`
	RootFields    []string     `gorm:"root_fields"`
	Errors        []GraphError `gorm:"errors"`
	HasErrors     bool         `gorm:"has_errors"`
}

// TableName is used by both the sql orm and mongo driver the table name and collection name used for operations on this model
// the conditional return is to ensure the right value is used for both the sql and mongo operations
func (g *GraphRecord) TableName() string {
	if GraphSQLTableName == "" {
		return g.AnalyticsRecord.TableName()
	}
	return GraphSQLTableName
}

// GetObjectID is a dummy function to satisfy the interface
func (*GraphRecord) GetObjectID() model.ObjectID {
	return ""
}

// SetObjectID is a dummy function to satisfy the interface
func (*GraphRecord) SetObjectID(model.ObjectID) {
	// empty
}

func (a *AnalyticsRecord) ToGraphRecord() GraphRecord {
	if !a.IsGraphRecord() {
		return GraphRecord{}
	}
	opType := ""
	switch a.GraphQLStats.OperationType {
	case OperationQuery:
		opType = "Query"
	case OperationMutation:
		opType = "Mutation"
	case OperationSubscription:
		opType = "Subscription"
	default:
	}
	record := GraphRecord{
		AnalyticsRecord: *a,
		RootFields:      a.GraphQLStats.RootFields,
		Types:           a.GraphQLStats.Types,
		Errors:          a.GraphQLStats.Errors,
		HasErrors:       a.GraphQLStats.HasErrors,
		Variables:       a.GraphQLStats.Variables,
		OperationType:   opType,
	}
	if a.ResponseCode >= 400 {
		record.HasErrors = true
	}
	return record
}
