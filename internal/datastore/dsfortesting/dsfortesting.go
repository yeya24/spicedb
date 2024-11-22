package dsfortesting

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/authzed/spicedb/internal/datastore/common"
	"github.com/authzed/spicedb/internal/datastore/memdb"
	"github.com/authzed/spicedb/pkg/datastore"
	"github.com/authzed/spicedb/pkg/datastore/options"
	"github.com/authzed/spicedb/pkg/tuple"
)

// NewMemDBDatastoreForTesting creates a new in-memory datastore for testing.
// This is a convenience function that wraps the creation of a new MemDB datastore,
// and injects additional proxies for validation at test time.
// NOTE: These additional proxies are not performant for use in production (but then,
// neither is memdb)
func NewMemDBDatastoreForTesting(
	watchBufferLength uint16,
	revisionQuantization,
	gcWindow time.Duration,
) (datastore.Datastore, error) {
	ds, err := memdb.NewMemdbDatastore(watchBufferLength, revisionQuantization, gcWindow)
	if err != nil {
		return nil, err
	}

	return validatingDatastore{ds}, nil
}

type validatingDatastore struct {
	datastore.Datastore
}

func (vds validatingDatastore) SnapshotReader(rev datastore.Revision) datastore.Reader {
	return validatingReader{vds.Datastore.SnapshotReader(rev)}
}

type validatingReader struct {
	datastore.Reader
}

func (vr validatingReader) QueryRelationships(
	ctx context.Context,
	filter datastore.RelationshipsFilter,
	options ...options.QueryOptionsOption,
) (datastore.RelationshipIterator, error) {
	schema := common.NewSchemaInformation(
		"relationtuples",
		"ns",
		"object_id",
		"relation",
		"subject_ns",
		"subject_object_id",
		"subject_relation",
		"caveat",
		"caveat_context",
		"expiration",
		common.TupleComparison,
		sq.Question,
		"NOW",
		common.ColumnOptimizationOptionStaticValues,
	)

	qBuilder, err := common.NewSchemaQueryFiltererForRelationshipsSelect(schema, 100).
		FilterWithRelationshipsFilter(filter)
	if err != nil {
		return nil, err
	}

	// Run the filter through the common SQL ellison system and ensure that any
	// relationships return have values matching the static fields, if applicable.
	var queryInfo *common.QueryInfo
	executor := common.QueryExecutor{
		Executor: func(ctx context.Context, qi common.QueryInfo, sql string, args []any) (datastore.RelationshipIterator, error) {
			queryInfo = &qi
			return nil, nil
		},
	}

	_, _ = executor.ExecuteQuery(ctx, qBuilder, options...)
	if queryInfo == nil {
		return nil, fmt.Errorf("no query info returned")
	}

	checkStaticField := func(returnedValue string, fieldName string) error {
		if found, ok := queryInfo.FilteringValues[fieldName]; ok && found.SingleValue != nil {
			if returnedValue != *found.SingleValue {
				return fmt.Errorf("static field `%s` does not match expected value `%s`: `%s", fieldName, returnedValue, *found.SingleValue)
			}
		}

		return nil
	}

	// Run the actual query on the memdb instance.
	iter, err := vr.Reader.QueryRelationships(ctx, filter, options...)
	if err != nil {
		return nil, err
	}

	return func(yield func(tuple.Relationship, error) bool) {
		for rel, err := range iter {
			if err != nil {
				if !yield(rel, err) {
					return
				}
				continue
			}

			if err := checkStaticField(rel.Resource.ObjectType, "ns"); err != nil {
				if !yield(rel, err) {
					return
				}
			}

			if err := checkStaticField(rel.Resource.ObjectID, "object_id"); err != nil {
				if !yield(rel, err) {
					return
				}
			}

			if err := checkStaticField(rel.Resource.Relation, "relation"); err != nil {
				if !yield(rel, err) {
					return
				}
			}

			if err := checkStaticField(rel.Subject.ObjectType, "subject_ns"); err != nil {
				if !yield(rel, err) {
					return
				}
			}

			if err := checkStaticField(rel.Subject.ObjectID, "subject_object_id"); err != nil {
				if !yield(rel, err) {
					return
				}
			}

			if err := checkStaticField(rel.Subject.Relation, "subject_relation"); err != nil {
				if !yield(rel, err) {
					return
				}
			}

			if !yield(rel, err) {
				return
			}
		}
	}, nil
}
