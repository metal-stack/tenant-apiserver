package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	v1 "github.com/metal-stack/tenant-api/go/api/v1"
	"github.com/metal-stack/tenant-apiserver/pkg/api"
	"github.com/metal-stack/tenant-apiserver/pkg/errorutil"

	// import for sqlx to use postgres driver
	_ "github.com/lib/pq"
)

// exchangeable for testing
var now = time.Now

// datastore is the adapter to talk to the database
type datastore[E api.Entity] struct {
	log              *slog.Logger
	db               *sqlx.DB
	sb               squirrel.StatementBuilderType
	jsonField        string
	tableName        string
	historyTableName string
}

type Op string

const (
	opCreate Op = "C"
	opUpdate Op = "U"
	opDelete Op = "D"
)

func InitTables(logger *slog.Logger, db *sqlx.DB, ves ...api.Entity) error {
	for _, ve := range ves {
		jsonField := ve.JSONField()
		logger.Info("creating schema", "entity", jsonField)
		_, err := db.Exec(ve.Schema())
		if err != nil {
			logger.Error("unable to create schema", "entity", jsonField, "error", err)
			return err
		}
	}
	return nil
}

// NewPostgresStorage creates a new Storage which uses postgres.
func NewPostgresDB(logger *slog.Logger, host, port, user, password, dbname, sslmode string, ves ...api.Entity) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=%s", host, port, user, dbname, password, sslmode))
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	err = InitTables(logger, db, ves...)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// New creates a new Storage which uses the given database abstraction.
func New[E api.Entity](logger *slog.Logger, db *sqlx.DB, e E) api.Storage[E] {
	ds := &datastore[E]{
		log:              logger,
		db:               db,
		sb:               squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar).RunWith(db),
		jsonField:        e.JSONField(),
		tableName:        e.TableName(),
		historyTableName: fmt.Sprintf("%s_history", e.TableName()),
	}
	return ds
}

// Create a entity
func (ds *datastore[E]) Create(ctx context.Context, ve E) error {
	ds.log.Debug("create", "entity", ds.jsonField, "value", ve)
	meta := ve.GetMeta()
	if meta == nil {
		return errorutil.InvalidArgument("create of type:%s failed, meta is nil", ds.jsonField)
	}
	id := meta.GetId()
	if id == "" {
		meta.Id = uuid.NewV7().String()
	}
	kind := meta.GetKind()
	if kind == "" {
		meta.Kind = ve.Kind()
	} else if kind != ve.Kind() {
		return errorutil.InvalidArgument("create of type:%s failed, kind is set to:%s but must be:%s", ds.jsonField, kind, ve.Kind())
	}
	apiVersion := meta.GetApiversion()
	if apiVersion == "" {
		meta.Apiversion = ve.APIVersion()
	} else if apiVersion != ve.APIVersion() {
		return errorutil.InvalidArgument("create of type:%s failed, apiversion must be set to:%s", ds.jsonField, ve.APIVersion())
	}

	createdAtPb, createdAt := pbNow()
	meta.Version = 0
	meta.CreatedTime = createdAtPb

	q := ds.sb.Insert(
		ds.tableName,
	).SetMap(map[string]any{
		"id":         id,
		ds.jsonField: ve,
	}).Suffix(
		"RETURNING " + ds.jsonField,
	)

	if ds.log.Enabled(ctx, slog.LevelDebug) {
		sql, vals, _ := q.ToSql()
		ds.log.Debug("create", "entity", ds.jsonField, "sql", sql, "values", vals)
	}

	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer ds.rollback(tx)

	err = q.RunWith(tx).QueryRowContext(ctx).Scan(ve)
	if err != nil {
		if IsErrorCode(err, UniqueViolationError) {
			return errorutil.Conflict("an entity of type:%s with the id:%s already exists", ds.jsonField, meta.Id)
		}
		return err
	}
	err = ds.insertHistory(ve, opCreate, createdAt, tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Update the entity
func (ds *datastore[E]) Update(ctx context.Context, ve E) error {
	ds.log.Debug("update", "entity", ds.jsonField)
	meta := ve.GetMeta()
	if meta == nil {
		return errorutil.InvalidArgument("update of type:%s failed, meta is nil", ds.jsonField)
	}
	id := meta.GetId()
	if id == "" {
		return errorutil.InvalidArgument("entity of type:%s has no id, cannot update: %v", ds.jsonField, ve)
	}
	kind := meta.GetKind()
	if kind == "" {
		meta.Kind = ve.Kind()
	} else if kind != ve.Kind() {
		return errorutil.InvalidArgument("update of type:%s failed, kind is set to:%s but must be:%s", ds.jsonField, kind, ve.Kind())
	}
	apiVersion := meta.GetApiversion()
	if apiVersion == "" {
		meta.Apiversion = ve.APIVersion()
	} else if apiVersion != ve.APIVersion() {
		return errorutil.InvalidArgument("update of type:%s failed, apiversion must be set to:%s", ds.jsonField, ve.APIVersion())
	}

	existingVE, err := ds.Get(ctx, id)
	if err != nil {
		return errorutil.NotFound("update - no entity of type:%s with id:%s found", ds.jsonField, id)
	}

	if ve.GetMeta().GetVersion() < existingVE.GetMeta().GetVersion() {
		return errorutil.Conflict("optimistic lock error updating %s with id %s, existing version %d mismatches entity version %d",
			ds.jsonField, id, existingVE.GetMeta().GetVersion(), ve.GetMeta().GetVersion(),
		)
	}

	pbNow, now := pbNow()

	ve.GetMeta().Version = ve.GetMeta().GetVersion() + 1
	ve.GetMeta().UpdatedTime = pbNow

	// handle non updatable fields like created_time
	// simple strategy: copy unmodifiable fields from existing before update
	ve.GetMeta().CreatedTime = existingVE.GetMeta().GetCreatedTime()

	q := ds.sb.Update(ds.tableName).
		SetMap(map[string]any{
			ds.jsonField: ve,
		}).
		Where(squirrel.Eq{
			"id": id,
		}).
		Suffix(
			"RETURNING " + ds.jsonField,
		)

	if ds.log.Enabled(ctx, slog.LevelDebug) {
		sql, vals, _ := q.ToSql()
		ds.log.Debug("update", "entity", ds.jsonField, "sql", sql, "values", vals)
	}

	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer ds.rollback(tx)

	err = q.RunWith(tx).QueryRowContext(ctx).Scan(ve)
	if err != nil {
		return err
	}

	// insert dataset in history table
	err = ds.insertHistory(ve, opUpdate, now, tx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Get the entity for given id
// returns NotFoundError if no entity can be found
func (ds *datastore[E]) Get(ctx context.Context, id string) (E, error) {
	ds.log.Debug("get", "entity", ds.jsonField, "id", id)
	var zero E
	q := ds.sb.Select(
		ds.jsonField,
	).From(
		ds.tableName,
	).Where(squirrel.Eq{
		"id": id,
	})

	row := q.QueryRowContext(ctx)
	e := new(E)
	err := row.Scan(e)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return zero, errorutil.NotFound("%s with id:%s not found %v", ds.jsonField, id, err)
		}
		return zero, err
	}
	return *e, nil
}

// Delete deletes the entity
func (ds *datastore[E]) Delete(ctx context.Context, id string) error {
	ds.log.Debug("delete", "entity", ds.jsonField, "id", id)
	ve, err := ds.Get(ctx, id)
	if err != nil {
		return err
	}

	// delete dataset in table
	q := ds.sb.Delete(ds.tableName).
		Where(squirrel.Eq{"id": id})
	// in tx
	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer ds.rollback(tx)

	result, err := q.RunWith(tx).ExecContext(ctx)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected > 1 {
		return errorutil.Internal("data corruption: delete of %s with id %s affected %d rows", ds.jsonField, id, rowsAffected)
	}
	if rowsAffected < 1 {
		return errorutil.NotFound("not found: delete of %s with id %s affected %d rows", ds.jsonField, id, rowsAffected)
	}

	// insert dataset in history table
	err = ds.insertHistory(ve, opDelete, now(), tx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteAll deletes the entities with the given ids
func (ds *datastore[E]) DeleteAll(ctx context.Context, ids ...string) error {
	ds.log.Debug("delete", "entities", ds.jsonField, "ids", ids)

	if len(ids) == 0 {
		return nil
	}

	var ves []E
	for _, id := range ids {
		ve, err := ds.Get(ctx, id)
		if err != nil {
			return err
		}
		ves = append(ves, ve)
	}

	// delete datasets in table
	q := ds.sb.Delete(ds.tableName).
		Where(squirrel.Eq{"id": ids})
	// in tx
	tx, err := ds.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer ds.rollback(tx)

	result, err := q.RunWith(tx).ExecContext(ctx)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected != int64(len(ids)) {
		return errorutil.Internal("data corruption: delete of %s with ids %s affected %d rows", ds.jsonField, ids, rowsAffected)
	}
	if rowsAffected < 1 {
		return errorutil.NotFound("not found: delete of %s with id %s affected %d rows", ds.jsonField, ids, rowsAffected)
	}

	// insert dataset in history table
	for _, ve := range ves {
		err = ds.insertHistory(ve, opDelete, now(), tx)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Find returns matching elements from the database
func (ds *datastore[E]) Find(ctx context.Context, paging *v1.Paging, filters ...any) ([]E, *uint64, error) {
	ds.log.Debug("find", "entity", ds.jsonField, "filters", filters)
	q := ds.sb.Select(ds.jsonField).
		From(ds.tableName)

	for _, filter := range filters {
		q = q.Where(filter)
	}

	q = q.OrderBy("id")

	// Add paging query if paging is defined
	q, nextPage := addPaging(q, paging)

	if ds.log.Enabled(ctx, slog.LevelDebug) {
		sql, vals, _ := q.ToSql()
		ds.log.Debug("find", "entity", ds.jsonField, "sql", sql, "values", vals)
	}

	rows, err := q.QueryContext(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	ves := []E{}
	for rows.Next() {
		e := new(E)
		err = rows.Scan(e)
		if err != nil {
			return nil, nil, err
		}
		ves = append(ves, *e)
	}

	err = rows.Err()
	if err != nil {
		return nil, nil, err
	}
	if paging != nil && paging.Count != nil && uint64(len(ves)) == *paging.Count {
		return ves, nextPage, err
	}
	return ves, nil, nil
}

// Get the history entity for given id and latest before or equal the given point in time
// returns NotFoundError if no entity can be found
func (ds *datastore[E]) GetHistory(ctx context.Context, id string, at time.Time, ve E) error {
	return ds.getHistoryWithPredicate(ctx, squirrel.And{
		squirrel.Eq{
			"id": id,
		},
		squirrel.LtOrEq{
			"created_at": at,
		},
	}, ve)
}

// Get the first history entity for given id, returns NotFoundError if no entity can be found
func (ds *datastore[E]) GetHistoryCreated(ctx context.Context, id string, ve E) error {
	return ds.getHistoryWithPredicate(ctx, squirrel.And{
		squirrel.Eq{
			"id": id,
		},
		squirrel.Eq{
			"op": opCreate,
		},
	}, ve)
}

// Get the top matching history entity for given filter criteria,
// returns NotFoundError if no entity can be found
func (ds *datastore[E]) getHistoryWithPredicate(ctx context.Context, pred any, ve E) error {
	q := ds.sb.Select(ds.jsonField).From(ds.historyTableName).Where(pred).OrderByClause("created_at DESC").Limit(1)

	sql, _, _ := q.ToSql()
	ds.log.Info("get", "entity", ds.jsonField, "sql", sql, "predicate", pred)
	rows, err := q.QueryContext(ctx)
	if err != nil {
		return err
	}

	defer func() {
		_ = rows.Close()
		_ = rows.Err()
	}()
	if rows.Next() {
		return rows.Scan(ve)
	}
	// we have no row
	return errorutil.NotFound("entity of type:%s with predicate:%s not found", ds.jsonField, pred)
}

// insertHistory inserts the given entity in the history table of the entity using the runner, which may be a Tx.
func (ds *datastore[E]) insertHistory(ve E, op Op, createdAt time.Time, runner squirrel.BaseRunner) error {
	qh := ds.sb.Insert(ds.historyTableName).
		SetMap(map[string]any{
			"id":         ve.GetMeta().Id,
			"op":         op,
			"created_at": createdAt,
			ds.jsonField: ve,
		})
	_, err := qh.RunWith(runner).Exec()
	if err != nil {
		return err
	}
	return nil
}

// pbNow returns the current time as Protobuf and time
func pbNow() (*timestamppb.Timestamp, time.Time) {
	now := now()
	nowPb := timestamppb.New(now)
	return nowPb, now
}

// rollback tries to rollback the given transaction and logs an eventual rollback error
func (ds *datastore[E]) rollback(tx *sql.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		ds.log.Error("error rolling back", "error", err)
	}
}

const defaultPagingLimit = uint64(100)

func addPaging(q squirrel.SelectBuilder, paging *v1.Paging) (squirrel.SelectBuilder, *uint64) {
	if paging == nil {
		return q, nil
	}

	limit := defaultPagingLimit
	if paging.Count != nil {
		limit = *paging.Count
	}
	offset := uint64(0)
	nextpage := uint64(1)
	if paging.Page != nil {
		offset = *paging.Page * limit
		nextpage = *paging.Page + 1
	}
	q = q.Limit(limit).Offset(offset)
	return q, &nextpage
}
