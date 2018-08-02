package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
)

const (
	API_MARKER = "apigen:api"
)

func main() {
	args := os.Args
	if len(args) < 3 {
		fmt.Println("Usage: codegen input.go output.go.")
		log.Fatalln("To few arguments.")
	}
	inputFile := args[1]
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Fatalf("File %s does not exists.", inputFile)
	}
	outputFile := args[2]
	genapi(inputFile, outputFile)
}

func genapi(in, out string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, in, nil, parser.ParseComments)
	if err != nil {
		log.Fatalln("Can not parse go source.")
	}
	//funcDecl := findFuncDecl(node)
	findFuncDecl(node)
	genOutput(out, node)
}

func findFuncDecl(node *ast.File) map[string][]*ast.FuncDecl {
	res := make(map[string][]*ast.FuncDecl)
	for _, el := range node.Decls {
		if v, ok := el.(*ast.FuncDecl); ok {
			if isGenApi(v) {
				exp := v.Recv.List[0].Type.(*ast.StarExpr)
				name := exp.X.(*ast.Ident).Name
				if fList, ok := res[name]; ok {
					res[name] = append(fList, v)
				} else {
					res[name] = []*ast.FuncDecl{v}
				}
			}
		}
	}
	return res
}

func isGenApi(decl *ast.FuncDecl) bool {
	if decl.Doc == nil {
		return false
	}
	for _, el := range decl.Doc.List {
		if strings.Contains(el.Text, API_MARKER) {
			return true
		}
	}
	return false
}

func genOutput(outputFile string, node *ast.File) {
	out, _ := os.Create(os.Args[2])
	fmt.Fprintln(out, `package `+node.Name.Name)
	fmt.Fprintln(out)
}
