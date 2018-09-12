package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
)

type ExplorerSvc struct {
	db          *sql.DB
	Tables      []*DBTable
	TablesNames map[string]*DBTable
}

type DBTable struct {
	Name string
}

func (s *ExplorerSvc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		s.ListAllTables(w, r)
		return
	}
	s.TableRouter(w, r)
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {
	svc := &ExplorerSvc{
		db: db,
	}
	svc.FillDBMeta()
	http.Handle("/", svc)
	return svc, nil
}

func (s *ExplorerSvc) FillDBMeta() {
	tables := make([]*DBTable, 0)
	tablesNames := make(map[string]*DBTable)
	rows, _ := s.db.Query("SHOW TABLES")
	for rows.Next() {
		tableName := ""
		err := rows.Scan(&tableName)
		if err != nil {
			panic(err)
		}
		t := &DBTable{Name: tableName}
		tables = append(tables, t)
		tablesNames[tableName] = t
		s.Tables = tables
		s.TablesNames = tablesNames
	}
}

func (s *ExplorerSvc) ListAllTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	res := make(map[string]map[string]interface{})
	tables := make([]string, 0)
	for _, el := range s.Tables {
		tables = append(tables, el.Name)
	}
	res["response"] = make(map[string]interface{})
	res["response"]["tables"] = tables
	body, _ := json.Marshal(res)
	w.Write(body)
}

func (s *ExplorerSvc) TableRouter(w http.ResponseWriter, r *http.Request) {
	tableName := regexp.MustCompile("^/([a-zA-Z0-9]+)")
	m := tableName.FindStringSubmatch(r.URL.Path)
	if _, ok := s.TablesNames[m[1]]; !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		body, _ := json.Marshal(map[string]string{"error": "unknown table"})
		w.Write(body)
		return
	} else {
		tn := m[1]
		tableName = regexp.MustCompile("^/([a-zA-Z0-9]+)/([a-zA-Z0-9]+)")
		m = tableName.FindStringSubmatch(r.URL.Path)
		if m == nil {
			// only table name
			s.HandleTableRequest(w, r, tn)
			return
		} else {
			// additional path part
		}
	}
}

func (s *ExplorerSvc) HandleTableRequest(w http.ResponseWriter, r *http.Request, tn string) {
	if r.Method == http.MethodGet {
		sqlExp := fmt.Sprintf("SELECT * FROM %s", tn)
		limit, ok := r.URL.Query()["limit"]
		if ok {
			// possible SQL injection
			sqlExp = fmt.Sprintf("%s LIMIT %s", sqlExp, limit[0])
		}
		offset, ok := r.URL.Query()["offset"]
		if ok {
			// possible SQL injection
			sqlExp = fmt.Sprintf("%s OFFSET %s", sqlExp, offset[0])
		}
		rows, _ := s.db.Query(sqlExp)
		cols, _ := rows.Columns()
		colsTypes, _ := rows.ColumnTypes()
		res := make(map[string]map[string]interface{})
		res["response"] = make(map[string]interface{})
		records := make([]interface{}, 0)
		for rows.Next() {
			data := CreateBlankData(len(cols), colsTypes)
			rows.Scan(data...)
			records = append(records, CreateRecord(data, colsTypes))
		}
		res["response"]["records"] = records
		w.Header().Set("Content-Type", "application/json")
		body, _ := json.Marshal(res)
		w.Write(body)
	}
}
func CreateRecord(data []interface{}, types []*sql.ColumnType) map[string]interface{} {
	record := make(map[string]interface{})
	for ix, el := range types {
		if reflect.TypeOf(data[ix]).Elem() == reflect.TypeOf(sql.RawBytes{}) {
			if len(*data[ix].(*sql.RawBytes)) == 0 {
				record[el.Name()] = nil
			} else {
				record[el.Name()] = string(*data[ix].(*sql.RawBytes))
			}
		} else {
			record[el.Name()] = data[ix]
		}
	}
	return record
}

func CreateBlankData(n int, types []*sql.ColumnType) []interface{} {
	data := make([]interface{}, n)
	for i, el := range types {
		t := el.ScanType()
		data[i] = reflect.New(t).Interface()
	}
	return data
}
