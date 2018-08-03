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
	Type       string
	Validators map[string]string
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
		http.NotFound(w, r)
	}
}

{{- range $ix, $point := $apiPoints }}
func (h *{{ $receiver }} ) handler{{ $point.Method }}(w http.ResponseWriter, r *http.Request) {
	// заполнение структуры params
	params := {{ $point.InParam }}{
		{{- range $ix, $f :=  $point.InParamFields }}
		{{- if index $f.Validators "paramname" }}
		{{ $f.Name }}: FillValue("{{ $f.Validators.paramname }}", "{{ $f.Type }}", r).({{ $f.Type }}),
		{{- else }}
		{{ $f.Name }}: FillValue("{{ $f.Name }}", "{{ $f.Type }}", r).({{ $f.Type }}),
		{{- end }}
		{{- end }}
	}
	// валидирование параметров
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
	res, err := h.{{ $point.Method }}(ctx, params)
	if err != nil {
		// do something
	}
	body, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	// прочие обработки
}
{{- end }}

{{- end }}
func FillValue(n, t string, r *http.Request) interface{}{
	n = strings.ToLower(n)
	val := r.FormValue(n)
	if t == "int" {
		res, _ := strconv.Atoi(val)
		return res
	}
	return val
}
`))

	validTmpl = template.Must(template.New("validTmpl").Parse(`
{{- range $k, $v := . }}

func Validate{{ $v.Name }}(param {{ $v.Name }}) *ApiError {
	{{- range $ix, $f := $v.ParamFields }}
	// validate {{ $f.Name }} field
	{{- range $namev, $valv := $f.Validators }}
	{{- if eq $namev "required" }}
	e := reflect.ValueOf(param).FieldByName("{{ $f.Name }}")
	if reflect.Zero(e.Type()).Interface() == e.Interface() {
		return &ApiError{
			HTTPStatus: http.StatusBadRequest,
			Err: fmt.Errorf("%s must me not empty", strings.ToLower("{{ $f.Name }}")),
		}
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
		sf := StructField{
			Name:       name,
			Type:       f.Type.(*ast.Ident).Name,
			Validators: parseValidators(tag),
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
		sf := StructField{
			Name:       name,
			Type:       f.Type.(*ast.Ident).Name,
			Validators: parseValidators(tag),
		}
		res[ix] = sf
	}
	return res
}

func parseValidators(s string) map[string]string {
	res := make(map[string]string)
	if s == "" {
		return res
	}
	s = strings.Trim(s, "`")
	s = s[strings.Index(s, "\""):]
	s = strings.Trim(s, "\"")
	for _, el := range strings.Split(s, ",") {
		parts := strings.Split(el, "=")
		parts = append(parts, "")
		res[parts[0]] = parts[1]
	}
	return res
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
