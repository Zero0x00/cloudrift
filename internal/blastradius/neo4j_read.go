package blastradius

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// readSingle runs a read query and returns the first record or nil when empty.
func readSingle(
	ctx context.Context,
	driver neo4j.DriverWithContext,
	database, cypher string,
	params map[string]any,
) (*neo4j.Record, error) {
	if driver == nil {
		return nil, fmt.Errorf("nil driver")
	}
	session := driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: database,
	})
	defer session.Close(ctx)

	res, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		r, err := tx.Run(ctx, strings.TrimSpace(cypher), params)
		if err != nil {
			return nil, err
		}
		if !r.Next(ctx) {
			if err := r.Err(); err != nil {
				return nil, err
			}
			return nil, nil
		}
		return r.Record(), nil
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	rec, ok := res.(*neo4j.Record)
	if !ok {
		return nil, fmt.Errorf("unexpected record type %T", res)
	}
	return rec, nil
}

// readList runs a read query and returns all records.
func readList(
	ctx context.Context,
	driver neo4j.DriverWithContext,
	database, cypher string,
	params map[string]any,
) ([]*neo4j.Record, error) {
	if driver == nil {
		return nil, fmt.Errorf("nil driver")
	}
	session := driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: database,
	})
	defer session.Close(ctx)

	records, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		r, err := tx.Run(ctx, strings.TrimSpace(cypher), params)
		if err != nil {
			return nil, err
		}
		var out []*neo4j.Record
		for r.Next(ctx) {
			rec := r.Record()
			if rec == nil {
				continue
			}
			out = append(out, rec)
		}
		if err := r.Err(); err != nil {
			return nil, err
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	return records.([]*neo4j.Record), nil
}
