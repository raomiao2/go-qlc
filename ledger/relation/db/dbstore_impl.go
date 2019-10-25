package db

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/qlcchain/go-qlc/config"
	"github.com/qlcchain/go-qlc/log"
)

type DBSQL struct {
	db     *sqlx.DB
	logger *zap.SugaredLogger
}

func NewSQLDB(cfg *config.Config) (*DBSQL, error) {
	dbsql := new(DBSQL)
	dbStr := cfg.DB.Driver
	switch dbStr {
	case "sqlite", "sqlite3":
		db, err := openSqlite(cfg)
		if err != nil {
			return nil, err
		}
		dbsql = &DBSQL{db: db, logger: log.NewLogger("relation/dbsql")}
	}
	return dbsql, nil
}

func (s *DBSQL) NewTransaction() *sqlx.Tx {
	return s.db.MustBegin()
}

func (s *DBSQL) Create(table TableName, condition map[Column]interface{}) error {
	sql := createSql(table, condition)
	s.logger.Debug(sql)
	if _, err := s.db.Exec(sql); err != nil {
		s.logger.Errorf("create error, sql: %s, err: %s", sql, err.Error())
		return err
	}
	return nil
}

func (s *DBSQL) BatchCreate(table TableName, cols []Column, vals [][]interface{}) error {
	sql := createBatchSql(table, cols, vals)
	s.logger.Debug(sql)
	if _, err := s.db.Exec(sql); err != nil {
		s.logger.Errorf("create error, sql: %s, err: %s", sql, err.Error())
		return err
	}
	return nil
}

func (s *DBSQL) Read(table TableName, condition map[Column]interface{}, offset int, limit int, order map[Column]bool, dest interface{}) error {
	sql := readSql(table, condition, offset, limit, order)
	s.logger.Debug(sql)
	err := s.db.Select(dest, sql)
	if err != nil {
		s.logger.Errorf("read error, sql: %s, err: %s", sql, err.Error())
		return err
	}
	return nil
}

func (s *DBSQL) Update(table TableName, condition map[Column]interface{}) error {
	panic("implement me")
}

func (s *DBSQL) Delete(table TableName, condition map[Column]interface{}) error {
	sql := deleteSql(table, condition)
	s.logger.Debug(sql)
	_, err := s.db.Exec(sql)
	if err != nil {
		s.logger.Errorf("delete error, sql: %s, err: %s", sql, err.Error())
		return err
	}
	return nil
}

func (s *DBSQL) Count(table TableName, dest interface{}) error {
	sql := fmt.Sprintf("select count (*) as total from %s", string(table))
	s.logger.Debug(sql)
	err := s.db.Get(dest, sql)
	if err != nil {
		s.logger.Errorf("count error, sql: %s, err: %s", sql, err.Error())
		return err
	}
	return nil
}

func (s *DBSQL) Group(table TableName, column Column, dest interface{}) error {
	sql := fmt.Sprintf("select %s, count(*) as count from %s  group by %s", string(column), string(table), string(column))
	s.logger.Debug(sql)
	err := s.db.Select(dest, sql)
	if err != nil {
		s.logger.Errorf("group error, sql: %s, err: %s", sql, err.Error())
		return err
	}
	return nil
}

func (s *DBSQL) Close() error {
	return s.db.Close()
}

func createSql(table TableName, condition map[Column]interface{}) string {
	var key []string
	var value []string
	for k, v := range condition {
		key = append(key, string(k))
		switch v.(type) {
		case string:
			value = append(value, fmt.Sprintf("'%s'", v.(string)))
		case int64:
			value = append(value, strconv.FormatInt(v.(int64), 10))
		}
	}
	sql := fmt.Sprintf("insert into %s (%s) values (%s)", string(table), strings.Join(key, ","), strings.Join(value, ","))
	return sql
}

func createBatchSql(table TableName, cols []Column, vals [][]interface{}) string {
	var keys []string
	var values []string

	for _, col := range cols {
		keys = append(keys, string(col))
	}

	for _, val := range vals {
		var t []string
		for _, v := range val {
			switch v.(type) {
			case string:
				t = append(t, fmt.Sprintf("'%s'", v.(string)))
			case int64:
				t = append(t, strconv.FormatInt(v.(int64), 10))
			}
		}
		values = append(values, fmt.Sprintf("(%s)", strings.Join(t, ",")))
	}

	sql := fmt.Sprintf("insert into %s (%s) values %s", string(table), strings.Join(keys, ","), strings.Join(values, ","))

	//for _, condition := range conditions {
	//	for k, v := range condition {
	//		key = append(key, string(k))
	//		switch v.(type) {
	//		case string:
	//			value = append(value, fmt.Sprintf("'%s'", v.(string)))
	//		case int64:
	//			value = append(value, strconv.FormatInt(v.(int64), 10))
	//		}
	//	}
	//}
	//sql := fmt.Sprintf("insert into %s (%s) values (%s)", string(table), strings.Join(key, ","), strings.Join(value, ","))
	return sql
}

func readSql(table TableName, condition map[Column]interface{}, offset int, limit int, order map[Column]bool) string {
	var sql string
	var para []string
	if condition != nil {
		for k, v := range condition {
			switch v.(type) {
			case string:
				s := v.(string)
				if strings.HasPrefix(s, LikeSign) {
					para = append(para, string(k)+" like '"+strings.TrimLeft(s, LikeSign)+"' ")
				} else {
					para = append(para, string(k)+" = '"+s+"' ")
				}
			case int64:
				para = append(para, string(k)+" = "+strconv.FormatInt(v.(int64), 10))
			}
		}
	}
	if len(para) != 0 {
		sql = fmt.Sprintf("select * from %s  where %s ", string(table), strings.Join(para, " and "))
	} else {
		sql = fmt.Sprintf("select * from %s ", string(table))
	}
	if order != nil {
		oStr := ""
		for k, v := range order {
			if v { // if v is true , ascending order
				oStr = oStr + string(k) + " asc ,"
			} else {
				oStr = oStr + string(k) + " desc ,"
			}
		}
		sql = sql + " order by  " + strings.TrimRight(oStr, ",")
	}

	if limit != -1 {
		sql = sql + " limit " + strconv.Itoa(limit)
	}
	if offset != -1 {
		sql = sql + " offset " + strconv.Itoa(offset)
	}
	return sql
}

func deleteSql(table TableName, condition map[Column]interface{}) string {
	var sql string
	var para []string
	if condition != nil {
		for k, v := range condition {
			switch v.(type) {
			case string:
				para = append(para, string(k)+" = '"+v.(string)+"' ")
			case int64:
				para = append(para, string(k)+" = "+strconv.FormatInt(v.(int64), 10))
			}
		}
	}
	if len(para) != 0 {
		sql = fmt.Sprintf("delete from %s  where %s ", string(table), strings.Join(para, " and "))
	} else {
		sql = fmt.Sprintf("delete from %s ", string(table))
	}
	return sql
}
