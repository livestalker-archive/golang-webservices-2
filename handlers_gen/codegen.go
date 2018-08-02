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
	Receiver string
	Method   string
	Json     *JsonApi
}

type JsonApi struct {
	Url    string
	Auth   bool
	Method string
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
	// валидирование параметров
	ctx := context.Background()
	res, err := h.{{ $point.Method }}(ctx, params)
	if err != nil {
		// do something
	}
	body, err := json.Marshal(res)
	if err != nil {
		// do something
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
	// прочие обработки
}
{{- end }}

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
	genOutput(out, node, funcDecl)
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
					Receiver: name,
					Method:   v.Name.Name,
					Json:     getJsonApi(comment),
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

func genOutput(outputFile string, node *ast.File, funcDecl map[string][]ApiPoint) {
	out, _ := os.Create(os.Args[2])
	defer out.Close()
	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out)
	fmt.Fprintln(out, `import "net/http"`)
	fmt.Fprintln(out, `import "encoding/json"`)
	fmt.Fprintln(out, `import "context"`)
	codeTmpl.Execute(out, funcDecl)
}
