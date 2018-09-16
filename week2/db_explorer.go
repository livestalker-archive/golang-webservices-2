package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type ExplorerSvc struct {
	db          *sql.DB
	Tables      []*DBTable
	TablesNames map[string]*DBTable
}

type DBTable struct {
	Name   string
	Fields []string
	Null   map[string]bool
	Types  []reflect.Kind
	PK     string
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
	// table name
	tables := make([]*DBTable, 0)
	tablesNames := make(map[string]*DBTable)
	rows, _ := s.db.Query("SHOW TABLES")
	for rows.Next() {
		tableName := ""
		err := rows.Scan(&tableName)
		if err != nil {
			panic(err)
		}
		t := &DBTable{Name: tableName, Null: make(map[string]bool)}
		tables = append(tables, t)
		tablesNames[tableName] = t
		s.Tables = tables
		s.TablesNames = tablesNames
	}
	for _, el := range s.Tables {
		name := el.Name
		sqlExp := fmt.Sprintf("SHOW FULL COLUMNS FROM `%s`", name)
		rows, _ := s.db.Query(sqlExp)
		cols, _ := rows.Columns()
		colsTypes, _ := rows.ColumnTypes()
		for rows.Next() {
			dataRaw := CreateBlankData(len(cols), colsTypes)
			rows.Scan(dataRaw...)
			data := CreateRecord(dataRaw, colsTypes)
			el.Fields = append(el.Fields, data["Field"].(string))
			typeS := data["Type"].(string)
			var typeR reflect.Kind
			if strings.Contains(typeS, "int") {
				typeR = reflect.Int
			} else if strings.Contains(typeS, "float") {
				typeR = reflect.Float64
			} else {
				typeR = reflect.String
			}
			el.Types = append(el.Types, typeR)
			if v, ok := data["Key"].(string); ok {
				if v == "PRI" {
					el.PK = data["Field"].(string)
				}
			}
			if data["Null"].(string) == "NO" {
				el.Null[data["Field"].(string)] = false
			} else {
				el.Null[data["Field"].(string)] = true
			}
		}
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
			id, _ := strconv.Atoi(m[2])
			s.HandleItemRequest(w, r, tn, id)
			return
		}
	}
}

func (s *ExplorerSvc) HandleTableRequest(w http.ResponseWriter, r *http.Request, tn string) {
	if r.Method == http.MethodGet {
		sqlExp := fmt.Sprintf("SELECT * FROM %s", tn)
		limit, ok := r.URL.Query()["limit"]
		if ok {
			l, err := strconv.Atoi(limit[0])
			if err != nil {
				l = 1
			}
			// possible SQL injection
			sqlExp = fmt.Sprintf("%s LIMIT %d", sqlExp, l)
		}
		offset, ok := r.URL.Query()["offset"]
		if ok {
			o, err := strconv.Atoi(offset[0])
			if err != nil {
				o = 1
			}
			// possible SQL injection
			sqlExp = fmt.Sprintf("%s OFFSET %d", sqlExp, o)
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
		return
	} else if r.Method == http.MethodPut {
		s.HandlePutTableRequest(w, r, tn)
		return
	}
}

func (s *ExplorerSvc) HandlePutTableRequest(w http.ResponseWriter, r *http.Request, tn string) {
	data := make(map[string]interface{})
	json.NewDecoder(r.Body).Decode(&data)
	realFields := fieldList(s.TablesNames[tn], data)
	values := valuesList(realFields, data)
	plh := strings.Repeat("?,", len(realFields))
	plh = strings.Trim(plh, ",")
	// possible SQL injection
	sqlExp := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tn, strings.Join(realFields, ","), plh)
	resSql, _ := s.db.Exec(sqlExp, values...)
	lastID, _ := resSql.LastInsertId()
	res := make(map[string]map[string]interface{})
	res["response"] = make(map[string]interface{})
	res["response"][s.TablesNames[tn].PK] = lastID
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(res)
	w.Write(body)
}

func fieldList(table *DBTable, data map[string]interface{}) []string {
	fields := make([]string, 0)
	for _, el := range table.Fields {
		if _, ok := data[el]; ok {
			if el != table.PK {
				fields = append(fields, el)
			}
		} else {
			if !table.Null[el] {
				fields = append(fields, el)
				data[el] = ""
			}
		}
	}
	return fields
}

func fieldList2(table *DBTable, data map[string]interface{}) []string {
	fields := make([]string, 0)
	for _, el := range table.Fields {
		if _, ok := data[el]; ok {
			if el != table.PK {
				fields = append(fields, el)
			}
		}
	}
	return fields
}

func valuesList(fields []string, data map[string]interface{}) []interface{} {
	res := make([]interface{}, 0)
	for _, el := range fields {
		res = append(res, data[el])
	}
	return res
}

func (s *ExplorerSvc) HandleItemRequest(w http.ResponseWriter, r *http.Request, tn string, id int) {
	if r.Method == http.MethodGet {
		s.HandleGetItemRequest(w, r, tn, id)
		return
	} else if r.Method == http.MethodPost {
		s.HandlePostItemRequest(w, r, tn, id)
		return
	} else if r.Method == http.MethodDelete {
		s.HandleDeleteItemRequest(w, r, tn, id)
	}
}

func (s *ExplorerSvc) HandleGetItemRequest(w http.ResponseWriter, r *http.Request, tn string, id int) {
	sqlExp := fmt.Sprintf("SELECT * FROM %s WHERE %s=?", tn, s.TablesNames[tn].PK)
	// We should use QueryRow
	rows, _ := s.db.Query(sqlExp, id)
	defer rows.Close()
	cols, _ := rows.Columns()
	colsTypes, _ := rows.ColumnTypes()
	data := CreateBlankData(len(cols), colsTypes)
	if !rows.Next() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		body, _ := json.Marshal(map[string]string{"error": "record not found"})
		w.Write(body)
		return
	}
	rows.Scan(data...)
	res := make(map[string]map[string]interface{})
	res["response"] = make(map[string]interface{})
	res["response"]["record"] = CreateRecord(data, colsTypes)
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(res)
	w.Write(body)
}

func (s *ExplorerSvc) HandlePostItemRequest(w http.ResponseWriter, r *http.Request, tn string, id int) {
	sqlExp := fmt.Sprintf("UPDATE %s SET ", tn)
	data := make(map[string]interface{})
	decoder := json.NewDecoder(r.Body)
	//decoder.UseNumber()
	decoder.Decode(&data)
	realFields := fieldList2(s.TablesNames[tn], data)
	if _, ok := data[s.TablesNames[tn].PK]; ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("field %s have invalid type", s.TablesNames[tn].PK)})
		w.Write(body)
		return
	}
	values := valuesList(realFields, data)
	err := validateValues(realFields, values, s.TablesNames[tn])
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		body, _ := json.Marshal(map[string]string{"error": err.Error()})
		w.Write(body)
		return
	}
	set := make([]string, 0)
	for _, el := range realFields {
		set = append(set, el+"=?")
	}
	sqlExp = sqlExp + strings.Join(set, ",") + fmt.Sprintf(" WHERE %s=?", s.TablesNames[tn].PK)
	values = append(values, id)
	resSql, _ := s.db.Exec(sqlExp, values...)
	affected, _ := resSql.RowsAffected()
	res := make(map[string]map[string]interface{})
	res["response"] = make(map[string]interface{})
	res["response"]["updated"] = affected
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(res)
	w.Write(body)
}

func (s *ExplorerSvc) HandleDeleteItemRequest(w http.ResponseWriter, r *http.Request, tn string, id int) {
	sqlExp := fmt.Sprintf("DELETE FROM %s WHERE %s=? ", tn, s.TablesNames[tn].PK)
	resSql, _ := s.db.Exec(sqlExp, id)
	affected, _ := resSql.RowsAffected()
	res := make(map[string]map[string]interface{})
	res["response"] = make(map[string]interface{})
	res["response"]["deleted"] = affected
	w.Header().Set("Content-Type", "application/json")
	body, _ := json.Marshal(res)
	w.Write(body)
}

func validateValues(filds []string, values []interface{}, table *DBTable) error {
	for ix, el := range filds {
		ixx := findIndex(el, table)
		if values[ix] != nil {
			if table.Types[ixx] != reflect.Int && reflect.TypeOf(values[ix]).Kind() != table.Types[ixx] {
				return errors.New(fmt.Sprintf("field %s have invalid type", table.Fields[ixx]))
			}
		} else {
			if !table.Null[el] {
				return errors.New(fmt.Sprintf("field %s have invalid type", table.Fields[ixx]))
			}
		}
	}
	return nil
}

func findIndex(name string, table *DBTable) int {
	for ix, el := range table.Fields {
		if name == el {
			return ix
		}
	}
	return 0
}

func CreateRecord(data []interface{}, types []*sql.ColumnType) map[string]interface{} {
	record := make(map[string]interface{})
	for ix, el := range types {
		if reflect.TypeOf(data[ix]).Elem() == reflect.TypeOf(sql.RawBytes{}) {
			// TODO check nulable
			n, _ := el.Nullable()
			if len(*data[ix].(*sql.RawBytes)) == 0 && n {
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
