package jsonstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"gopkg.in/yaml.v2"
)

type Config struct {
	host     string
	user     string
	password string
	port     string
	dbname   string
	sslmode  string
}

func NewConfig(host, user, password, port, dbname, sslmode string) Config {
	return Config{
		host:     host,
		user:     user,
		password: password,
		port:     port,
		dbname:   dbname,
		sslmode:  sslmode,
	}
}

func LoadConfiguration(ctx context.Context) Config {
	return Config{
		host:     env.GetVariableOrDefault(ctx, "POSTGRES_HOST", ""),
		user:     env.GetVariableOrDefault(ctx, "POSTGRES_USER", ""),
		password: env.GetVariableOrDefault(ctx, "POSTGRES_PASSWORD", ""),
		port:     env.GetVariableOrDefault(ctx, "POSTGRES_PORT", "5432"),
		dbname:   env.GetVariableOrDefault(ctx, "POSTGRES_DBNAME", "diwise"),
		sslmode:  env.GetVariableOrDefault(ctx, "POSTGRES_SSLMODE", "disable"),
	}
}

func (c Config) ConnStr() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.user, c.password, c.host, c.port, c.dbname, c.sslmode)
}

type EntityConfig struct {
	IDPattern string `yaml:"idPattern"`
	Type      string `yaml:"type"`
	TableName string `yaml:"tableName"`
}

type StorageConfig struct {
	ServiceName string         `yaml:"serviceName"`
	Entities    []EntityConfig `yaml:"entities"`
}

func loadStorageConfig(data io.Reader) (*StorageConfig, error) {
	buf, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	cfg := &StorageConfig{}
	err = yaml.Unmarshal(buf, &cfg)

	return cfg, err
}

type JsonStorage struct {
	db           *pgxpool.Pool
	entityConfig map[string]EntityConfig
}

func New(ctx context.Context, config Config, r io.Reader) (JsonStorage, error) {
	db, err := NewPool(ctx, config)
	if err != nil {
		return JsonStorage{}, err
	}

	return NewWithPool(ctx, db, r)
}

func NewPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	p, err := pgxpool.New(ctx, config.ConnStr())
	if err != nil {
		return nil, err
	}

	err = p.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}

func NewWithPool(ctx context.Context, pool *pgxpool.Pool, r io.Reader) (JsonStorage, error) {
	cfg, err := loadStorageConfig(r)
	if err != nil {
		return JsonStorage{}, err
	}

	m := make(map[string]EntityConfig)

	for _, c := range cfg.Entities {
		if c.TableName == "" {
			c.TableName = fmt.Sprintf("%s_%s", cfg.ServiceName, c.Type)
		}
		m[c.Type] = c
	}

	return JsonStorage{
		db:           pool,
		entityConfig: m,
	}, nil
}

func (s JsonStorage) Initialize(ctx context.Context, sql ...string) error {
	var errs []error

	if len(s.entityConfig) == 0 {
		errs = append(errs, fmt.Errorf("configuration error"))
	}

	ddl := `CREATE TABLE IF NOT EXISTS %s (					
		id			TEXT 	NOT NULL,			
		type 		TEXT 	NOT NULL,			
		data 		JSONB	NULL,	
		tenant		TEXT 	NOT NULL,	
		created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,			
		modified_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,	
		deleted     BOOLEAN DEFAULT FALSE,
		deleted_on  timestamp with time zone NULL,
		CONSTRAINT pkey_%s_unique PRIMARY KEY (id, type, deleted));
		CREATE INDEX IF NOT EXISTS idx_%s_deleted ON %s (id, type) WHERE deleted = FALSE;
		`

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}

	for _, v := range s.entityConfig {
		query := fmt.Sprintf(ddl, v.TableName, v.TableName, v.TableName, v.TableName)

		_, err := tx.Exec(ctx, query)
		if err != nil {
			errs = append(errs, err)
		}
	}

	for _, s := range sql {
		_, err := tx.Exec(ctx, s)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		err = tx.Rollback(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		return errors.Join(errs...)
	}

	return tx.Commit(ctx)
}

func (s JsonStorage) Close() {
	s.db.Close()
}

var (
	ErrNoRows        = errors.New("no rows in result set")
	ErrTooManyRows   = errors.New("too many rows in result set")
	ErrQueryRow      = errors.New("could not execute query")
	ErrStoreFailed   = errors.New("could not store data")
	ErrNoID          = errors.New("data contains no id")
	ErrMissingTenant = errors.New("missing tenant information")
)

func (s JsonStorage) Delete(ctx context.Context, id, typeName string, tenants []string) error {
	args := pgx.NamedArgs{
		"id":     id,
		"type":   typeName,
		"tenant": tenants,
	}

	tableName := s.entityConfig[typeName].TableName

	sql := fmt.Sprintf(`UPDATE %s 
	        			SET deleted=TRUE, deleted_on=NOW()
						WHERE id=@id AND type=@type AND tenant=any(@tenant)`, tableName)

	_, err := s.db.Exec(ctx, sql, args)

	return err
}

func (s JsonStorage) FindByID(ctx context.Context, id, typeName string, tenants []string) ([]byte, error) {
	err := validateID(id, s.entityConfig[typeName].IDPattern)
	if err != nil {
		return nil, err
	}

	args := pgx.NamedArgs{
		"id":     id,
		"type":   typeName,
		"tenant": tenants,
	}

	tableName := s.entityConfig[typeName].TableName

	sql := fmt.Sprintf(`SELECT data 
	        			FROM %s
						WHERE id=@id AND type=@type AND tenant=any(@tenant) AND deleted = FALSE`, tableName)

	var data json.RawMessage
	err = s.db.QueryRow(ctx, sql, args).Scan(&data)
	if err != nil {
		logging.GetFromContext(ctx).Debug("could not FindByID", "id", id, "table_name", tableName, "err", err.Error())

		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoRows
		}
		return nil, ErrQueryRow
	}

	return data, nil
}

func FindByID[T any](ctx context.Context, s JsonStorage, id string, tenants []string) (T, error) {
	typeName := getTypeName[T]()

	data, err := s.FindByID(ctx, id, typeName, tenants)
	if err != nil {
		return *new(T), err
	}

	t := *new(T)
	err = json.Unmarshal(data, &t)
	if err != nil {
		return *new(T), err
	}

	return t, nil
}

func (s JsonStorage) Store(ctx context.Context, id, typeName string, data []byte, tenant string) error {
	if tenant == "" {
		return ErrMissingTenant
	}

	err := validateID(id, s.entityConfig[typeName].IDPattern)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"id":     id,
		"type":   typeName,
		"data":   string(data),
		"tenant": tenant,
	}

	tableName := s.entityConfig[typeName].TableName

	upsert := fmt.Sprintf(`INSERT INTO %s (id, type, data, tenant) VALUES (@id, @type, @data, @tenant)			   
			   			   ON CONFLICT ON CONSTRAINT pkey_%s_unique
			   			   DO UPDATE SET data=EXCLUDED.data, modified_on=NOW();`, tableName, tableName)

	_, err = s.db.Exec(ctx, upsert, args)
	if err != nil {
		return ErrStoreFailed
	}

	return nil
}

func Store[T any](ctx context.Context, s JsonStorage, t T, tenant string) error {
	valueOf := func(n string, v any) (string, bool) {
		val := reflect.ValueOf(v)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return "", false
		}
		typ := val.Type()
		for i := 0; i < val.NumField(); i++ {
			tag := typ.Field(i).Tag.Get("jsonstore")
			if strings.EqualFold(n, typ.Field(i).Name) || strings.EqualFold(n, tag) {
				if val.Field(i).Kind() == reflect.String {
					return fmt.Sprintf("%v", val.Field(i).Interface()), true
				}
			}
		}
		return "", false
	}

	id, idOk := valueOf("id", t)
	if !idOk {
		return ErrNoID
	}

	typeName, typeNameOk := valueOf("type", t)
	if !typeNameOk {
		typeName = getTypeName[T]()
	}

	data, err := json.Marshal(t)
	if err != nil {
		return err
	}

	return s.Store(ctx, id, typeName, data, tenant)
}

type QueryResult struct {
	Data       [][]byte
	Count      uint64
	Offset     uint64
	Limit      uint64
	TotalCount uint64
}

func NewQueryResult(data [][]byte, count int, totalCount uint64, offset, limit int) QueryResult {
	return QueryResult{
		Data:       data,
		Count:      uint64(count),
		TotalCount: totalCount,
		Offset:     uint64(offset),
		Limit:      uint64(limit),
	}
}

type Condition func(map[string]any) map[string]any

func Offset(v int) Condition {
	return func(q map[string]any) map[string]any {
		q["offset"] = v
		return q
	}
}

func Limit(v int) Condition {
	return func(q map[string]any) map[string]any {
		q["limit"] = v
		return q
	}
}

func SortBy(colName string) Condition {
	return func(q map[string]any) map[string]any {
		if colName != "" {
			q["sortBy"] = colName
		}
		return q
	}
}

func (s JsonStorage) Query(ctx context.Context, q string, tenants []string, conditions ...Condition) (QueryResult, error) {
	if len(q) == 0 {
		return QueryResult{}, fmt.Errorf("no query specified")
	}

	args := pgx.NamedArgs{
		"tenant": tenants,
		"offset": 0,
		"limit":  100,
		"sortBy": "id",
	}
	for _, condition := range conditions {
		condition(args)
	}

	sb := strings.Builder{}
	sb.WriteString("SELECT data, count(*) OVER () AS total_count FROM (")

	for _, v := range s.entityConfig {
		sb.WriteString(fmt.Sprintf("SELECT data FROM %s WHERE %s AND tenant=any(@tenant) AND deleted = FALSE\n", v.TableName, q))
		sb.WriteString("UNION\n")
	}
	query := strings.TrimSuffix(sb.String(), "UNION\n")
	query = query + ") all_tables ORDER BY (data->>@sortBy) OFFSET @offset LIMIT @limit"

	rows, err := s.db.Query(ctx, query, args)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	var data [][]byte
	var totalCount uint64

	for rows.Next() {
		var d json.RawMessage
		err := rows.Scan(&d, &totalCount)
		if err != nil {
			return QueryResult{}, err
		}
		data = append(data, d)
	}

	return NewQueryResult(data, len(data), totalCount, args["offset"].(int), args["limit"].(int)), nil
}

func (s JsonStorage) QueryWithinBounds(ctx context.Context, bounds types.Bounds) (QueryResult, error) {
	q := "latitude BETWEEN @minLat AND @maxLat AND longitude BETWEEN @minLon AND @maxLon"

	args := pgx.NamedArgs{
		"tenant": "default",
		"offset": 0,
		"limit":  100,
		"sortBy": "id",
		"minLat": bounds.MinLat,
		"minLon": bounds.MinLon,
		"maxLat": bounds.MaxLat,
		"maxLon": bounds.MaxLon,
	}

	sb := strings.Builder{}
	sb.WriteString("SELECT data, count(*) OVER () AS total_count FROM (")

	for _, v := range s.entityConfig {
		sb.WriteString(fmt.Sprintf("SELECT data FROM %s WHERE %s AND tenant=any(@tenant) AND deleted = FALSE\n", v.TableName, q))
		sb.WriteString("UNION\n")
	}
	query := strings.TrimSuffix(sb.String(), "UNION\n")
	query = query + ") all_tables ORDER BY (data->>@sortBy) OFFSET @offset LIMIT @limit"

	rows, err := s.db.Query(ctx, query, args)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	var data [][]byte
	var totalCount uint64

	for rows.Next() {
		var d json.RawMessage
		err := rows.Scan(&d, &totalCount)
		if err != nil {
			return QueryResult{}, err
		}
		data = append(data, d)
	}

	return NewQueryResult(data, len(data), totalCount, args["offset"].(int), args["limit"].(int)), nil
}

func (s JsonStorage) QueryType(ctx context.Context, typeName, q string, tenants []string, conditions ...Condition) (QueryResult, error) {
	if len(q) == 0 {
		return QueryResult{}, fmt.Errorf("no query specified")
	}

	if _, ok := s.entityConfig[typeName]; !ok {
		return QueryResult{}, fmt.Errorf("invalid type")
	}

	args := pgx.NamedArgs{
		"typeName": typeName,
		"tenant":   tenants,
		"offset":   0,
		"limit":    100,
		"sortBy":   "id",
	}
	for _, condition := range conditions {
		condition(args)
	}

	query := fmt.Sprintf(`SELECT data, count(*) OVER () AS total_count 
	                      FROM %s 
						  WHERE %s AND type=@typeName AND tenant=any(@tenant) AND deleted = FALSE
						  ORDER BY (data->>@sortBy) 
						  OFFSET @offset LIMIT @limit`, s.entityConfig[typeName].TableName, q)
	rows, err := s.db.Query(ctx, query, args)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	var data [][]byte
	var totalCount uint64

	for rows.Next() {
		var d json.RawMessage
		err := rows.Scan(&d, &totalCount)
		if err != nil {
			return QueryResult{}, err
		}
		data = append(data, d)
	}

	return NewQueryResult(data, len(data), totalCount, args["offset"].(int), args["limit"].(int)), nil
}

func (s JsonStorage) FetchType(ctx context.Context, typeName string, tenants []string, conditions ...Condition) (QueryResult, error) {
	args := pgx.NamedArgs{
		"typeName": typeName,
		"tenant":   tenants,
		"offset":   0,
		"limit":    100,
		"sortBy":   "id",
	}
	for _, condition := range conditions {
		condition(args)
	}

	query := fmt.Sprintf(`SELECT data, count(*) OVER () AS total_count 
	                      FROM %s 
						  WHERE type=@typeName AND tenant=any(@tenant) AND deleted = FALSE
						  ORDER BY (data->>@sortBy)
						  OFFSET @offset LIMIT @limit`, s.entityConfig[typeName].TableName)
	rows, err := s.db.Query(ctx, query, args)
	if err != nil {
		return QueryResult{}, err
	}
	defer rows.Close()

	var data [][]byte
	var totalCount uint64

	for rows.Next() {
		var d json.RawMessage
		err := rows.Scan(&d, &totalCount)
		if err != nil {
			return QueryResult{}, err
		}
		data = append(data, d)
	}

	return NewQueryResult(data, len(data), totalCount, args["offset"].(int), args["limit"].(int)), nil
}

func (s JsonStorage) GetTenants(ctx context.Context) []string {
	sb := strings.Builder{}

	sb.WriteString("SELECT DISTINCT tenant FROM (")
	for _, v := range s.entityConfig {
		sb.WriteString(fmt.Sprintf("SELECT DISTINCT tenant FROM %s WHERE deleted = FALSE\n", v.TableName))
		sb.WriteString("UNION\n")
	}
	query := strings.TrimSuffix(sb.String(), "UNION\n")
	query = query + ") all_tables ORDER BY tenant ASC"

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var tenants []string

	for rows.Next() {
		var t string
		err := rows.Scan(&t)
		if err != nil {
			return []string{}
		}
		tenants = append(tenants, t)
	}

	return tenants
}

func validateID(id, idPattern string) error {
	regexpForID, err := regexp.CompilePOSIX(idPattern)
	if err != nil {
		return fmt.Errorf("could not compile regexp")
	}

	if !regexpForID.MatchString(id) {
		return fmt.Errorf("id did not match pattern")
	}

	return nil
}

func getTypeName[T any]() string {
	t := *new(T)
	typeName := fmt.Sprintf("%T", t)
	if strings.Contains(typeName, ".") {
		parts := strings.Split(typeName, ".")
		typeName = parts[len(parts)-1]
	}
	return typeName
}

func MapOne[T any](b []byte) (T, error) {
	t := *new(T)
	err := json.Unmarshal(b, &t)
	return t, err
}

func MapAll[T any](arr [][]byte) ([]T, error) {
	var errs []error
	m := make([]T, 0, len(arr))
	for _, b := range arr {
		t, err := MapOne[T](b)
		if err != nil {
			errs = append(errs, err)
		}
		m = append(m, t)
	}
	return m, errors.Join(errs...)
}
