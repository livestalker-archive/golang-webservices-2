package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
	"text/template"
)

const (
	API_MARKER = "apigen:api"
)

type ApiPoint struct {
	Receiver      string
	Method        string
	Params        []*ast.Field
	InParam       string
	InParamFields []StructField
	Json          *JsonApi
}

type ApiParam struct {
	Name        string
	ParamFields []StructField
}

type JsonApi struct {
	Url    string
	Auth   bool
	Method string
}

type StructField struct {
	Name       string
	CustomName string
	Type       string
	Validators []Validator
}

type Validator struct {
	Name  string
	Value string
}

var (
	codeTmpl = template.Must(template.New("codeTmpl").Parse(`
{{- range $receiver, $apiPoints := . }}
func (h *{{ $receiver }} ) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
{{- range $ix, $point := $apiPoints }}
	case "{{ $point.Json.Url }}":
		h.handler{{ $point.Method }}(w, r)
{{- end }}
	default:
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "unknown method",}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write(body)
		return
	}
}

{{- range $ix, $point := $apiPoints }}
func (h *{{ $receiver }} ) handler{{ $point.Method }}(w http.ResponseWriter, r *http.Request) {
	{{- if $point.Json.Auth }}
	// 1. проверка авторизации
	if h, ok := r.Header["X-Auth"]; ok {
		if h[0] != "100500" {
			w.Header().Set("Content-Type", "application/json")
			res := map[string]string{"error": "unauthorized",}
			body, _ := json.Marshal(res)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write(body)
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "unauthorized",}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		w.Write(body)
		return
	}
	{{- end }}
	{{- if $point.Json.Method }}
	// 2. проверки метода (GET/POST)
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": "bad method",}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(body)
		return
	}
	{{- end }}
	// 3. заполнение структуры params
	var vErr *ApiError
	{{- range $ix, $f :=  $point.InParamFields }}
	{{- if $f.CustomName }}
	val{{ $f.Name }}, vErr := FillValue("{{ $f.CustomName }}", "{{ $f.Type }}", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error(),}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	{{- else }}
	val{{ $f.Name }}, vErr := FillValue("{{ $f.Name }}", "{{ $f.Type }}", r)
	if vErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": vErr.Error(),}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(vErr.HTTPStatus)
		w.Write(body)
		return
	}
	{{- end }}
	{{- end }}
	params := {{ $point.InParam }}{
		{{- range $ix, $f :=  $point.InParamFields }}
		{{ $f.Name }}: val{{$f.Name}}.({{ $f.Type }}),
		{{- end }}
	}
	// 4. валидирование параметров
	valErr := Validate{{ $point.InParam }}(params)
	if valErr != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": valErr.Error(),}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(valErr.HTTPStatus)
		w.Write(body)
		return
	}
	ctx := context.Background()
	answer, err := h.{{ $point.Method }}(ctx, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		res := map[string]string{"error": err.Error(),}
		body, _ := json.Marshal(res)
		w.Header().Set("Content-Type", "application/json")
		if err, ok := err.(ApiError); ok {
			w.WriteHeader(err.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(body)
		return
	}
	res := map[string]interface{}{
		"error": "",
		"response": answer,
	}	
	body, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	// прочие обработки
}
{{- end }}

{{- end }}
func FillValue(n, t string, r *http.Request) (interface{}, *ApiError){
	n = strings.ToLower(n)
	val := r.FormValue(n)
	if t == "int" {
		res, err := strconv.Atoi(val)
		if err != nil {
			return 0, &ApiError{
				HTTPStatus:http.StatusBadRequest,
				Err:fmt.Errorf("%s must be %s", n, t),
			}
		}
		return res, nil
	}
	return val, nil
}
`))

	validTmpl = template.Must(template.New("validTmpl").Parse(`
{{- range $k, $v := . }}

func Validate{{ $v.Name }}(param {{ $v.Name }}) *ApiError {
	var e reflect.Value
	var isReqErr bool
	var reqErr *ApiError	
	{{- range $ix, $f := $v.ParamFields }}
	// validate {{ $f.Name }} field
	{{- range $ix, $v := $f.Validators }}

	{{- if eq $v.Name "required" }}
	// validate required status
	e = reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	if reflect.Zero(e.Type()).Interface() == e.Interface() {
		reqErr = &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s must me not empty", strings.ToLower("{{ $f.Name }}")),
		}
		isReqErr = true
	}
	{{- end }}

	{{- if eq $v.Name "min" }}
	// validate min value
	{{- if eq $f.Type "string" }}
	e = reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	if len(e.Interface().(string)) < {{ $v.Value }} {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s len must be >= %d", strings.ToLower("{{ $f.Name }}"), {{ $v.Value }}),
		}
	}
	{{- else }}
	e = reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	if e.Interface().(int) < {{ $v.Value }} {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s must be >= %d", strings.ToLower("{{ $f.Name }}"), {{ $v.Value }}),
		}
	}
	{{- end }}
	{{- end }}

	{{- if eq $v.Name "max" }}
	// validate max value
	{{- if eq $f.Type "string" }}
	e = reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	if len(e.Interface().(string)) > {{ $v.Value }} {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s len must be <= %d", strings.ToLower("{{ $f.Name }}"), {{ $v.Value }}),
		}
	}
	{{- else }}
	e = reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	if e.Interface().(int) > {{ $v.Value }} {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s must be <= %d", strings.ToLower("{{ $f.Name }}"), {{ $v.Value }}),
		}
	}
	{{- end }}
	{{- end }}

	{{- if eq $v.Name "enum" }}
	// validate enum value
	enumVal := strings.Split("{{ $v.Value }}", "|")
	e = reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	var findVal bool
	for _, el :=range enumVal {
		if el == e.Interface().(string) {
			findVal = true
			break
		}
	}
	if !findVal {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s must be one of [%v]", strings.ToLower("{{ $f.Name }}"), strings.Join(enumVal, ", ")),
		}
	}
	{{- end }}

	{{- if eq $v.Name "default" }}
	param.{{ $f.Name }} = "{{ $v.Value }}"
	isReqErr = false
	reqErr = nil
	{{- else }}
	if isReqErr {
		return reqErr
	}
	{{- end }}

	{{- end }}
	{{- end }}
	return nil
}

{{- end }}
`))
)

func main() {
	args := os.Args
	if len(args) < 3 {
		fmt.Println("Usage: codegen input.go output.go")
		log.Fatalln("To few arguments")
	}
	inputFile := args[1]
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Fatalf("File %s does not exists", inputFile)
	}
	outputFile := args[2]
	genapi(inputFile, outputFile)
}

func genapi(in, out string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, in, nil, parser.ParseComments)
	if err != nil {
		log.Fatalln("Can not parse go source")
	}
	funcDecl := findFuncDecl(node)
	paramsStructNames := getParamsStructNames(funcDecl)
	structDecl := findStructDecl(node, paramsStructNames)
	genOutput(out, node, funcDecl, structDecl)
}
func getParamsStructNames(funcDecl map[string][]ApiPoint) map[string]int {
	res := make(map[string]int)
	for _, v := range funcDecl {
		for _, el := range v {
			res[el.InParam] = 1
		}
	}
	return res
}

func findStructDecl(node *ast.File, paramsStructNames map[string]int) map[string]ApiParam {
	res := make(map[string]ApiParam)
	for _, el := range node.Decls {
		if d, ok := el.(*ast.GenDecl); ok {
			if structDecl, ok := d.Specs[0].(*ast.TypeSpec); ok {
				if _, ok := paramsStructNames[structDecl.Name.Name]; !ok {
					continue
				}
				p := ApiParam{
					Name:        structDecl.Name.Name,
					ParamFields: getStructFields2(structDecl),
				}
				res[structDecl.Name.Name] = p
			}
		}
	}
	return res
}

func findFuncDecl(node *ast.File) map[string][]ApiPoint {
	res := make(map[string][]ApiPoint)
	for _, el := range node.Decls {
		if v, ok := el.(*ast.FuncDecl); ok {
			if ok, comment := isGenApi(v); ok {
				getJsonApi(comment)
				exp := v.Recv.List[0].Type.(*ast.StarExpr)
				name := exp.X.(*ast.Ident).Name
				apiPoint := ApiPoint{
					Receiver:      name,
					Method:        v.Name.Name,
					Params:        v.Type.Params.List,
					InParam:       getParamType(v.Type.Params.List[1]),
					InParamFields: getStructFields(v.Type.Params.List[1]),
					Json:          getJsonApi(comment),
				}
				if pointList, ok := res[name]; ok {
					res[name] = append(pointList, apiPoint)
				} else {
					res[name] = []ApiPoint{apiPoint}
				}
			}
		}
	}
	return res
}

func getStructFields(s *ast.Field) []StructField {
	fields := s.Type.(*ast.Ident).Obj.Decl.(*ast.TypeSpec).Type.(*ast.StructType).Fields.List
	res := make([]StructField, len(fields))
	for ix, f := range fields {
		name := f.Names[0].Name
		tag := f.Tag.Value
		v, cn := parseValidators(tag)
		sf := StructField{
			Name:       name,
			Type:       f.Type.(*ast.Ident).Name,
			CustomName: cn,
			Validators: v,
		}
		res[ix] = sf
	}
	return res
}

func getStructFields2(s *ast.TypeSpec) []StructField {
	fields := s.Type.(*ast.StructType).Fields.List
	res := make([]StructField, len(fields))
	for ix, f := range fields {
		name := f.Names[0].Name
		var tag string
		if f.Tag != nil {
			tag = f.Tag.Value
		} else {
			tag = ""
		}
		v, cn := parseValidators(tag)
		sf := StructField{
			Name:       name,
			Type:       f.Type.(*ast.Ident).Name,
			CustomName: cn,
			Validators: v,
		}
		res[ix] = sf
	}
	return res
}

func parseValidators(s string) ([]Validator, string) {
	res := make([]Validator, 0)
	customName := ""
	if s == "" {
		return res, customName
	}
	s = strings.Trim(s, "`")
	s = s[strings.Index(s, "\""):]
	s = strings.Trim(s, "\"")
	for _, el := range strings.Split(s, ",") {
		parts := strings.Split(el, "=")
		parts = append(parts, "")
		res = append(res, Validator{Name: parts[0], Value: parts[1]})
		if parts[0] == "paramname" {
			customName = parts[1]
		}
	}
	return res, customName
}

func isGenApi(decl *ast.FuncDecl) (bool, string) {
	if decl.Doc == nil {
		return false, ""
	}
	for _, el := range decl.Doc.List {
		if strings.Contains(el.Text, API_MARKER) {
			return true, el.Text
		}
	}
	return false, ""
}

func getJsonApi(comment string) *JsonApi {
	res := &JsonApi{}
	jsonStr := comment[strings.Index(comment, "{"):]
	err := json.Unmarshal([]byte(jsonStr), res)
	if err != nil {
		log.Fatalln("Wrong json in comments")
	}
	return res
}

func genOutput(outputFile string, node *ast.File, funcDecl map[string][]ApiPoint, structDecl map[string]ApiParam) {
	out, _ := os.Create(os.Args[2])
	defer out.Close()
	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "context"`)
	fmt.Fprintln(out, `import "reflect"`)
	fmt.Fprintln(out, `import "fmt"`)
	fmt.Fprintln(out, `import "strings"`)
	fmt.Fprintln(out, `import "strconv"`)
	codeTmpl.Execute(out, funcDecl)
	validTmpl.Execute(out, structDecl)
}

func getParamType(p *ast.Field) string {
	return p.Type.(*ast.Ident).Name
}
