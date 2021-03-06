package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	typesstr = flag.String("types", "", "comma-separated list of type names")
	output   = flag.String("output", "", "output filename; default stdout")
)

// Usage for flag.Usage
func Usage() {
	fmt.Fprintf(os.Stderr, "")
	flag.PrintDefaults()
}

type generator struct {
	buf         bytes.Buffer
	dir         string
	targetTypes []string
	packageName string
	astFiles    []*ast.File
}

func main() { os.Exit(exec()) }

func exec() int {
	flag.Usage = Usage
	flag.Parse()
	if len(*typesstr) == 0 {
		flag.Usage()
		return 1
	}
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"."}
	}
	g := generator{
		targetTypes: strings.Split(*typesstr, ","),
	}
	var filenames []string
	if len(args) == 1 && isDirectory(args[0]) {
		g.dir = args[0]
		g.packageName, filenames = parseDir(args[0])
	} else {
		filenames = args
	}
	if err := g.parse(filenames); err != nil {
		log.Fatal(err)
	}
	out := getOutput(*output)
	defer out.Close()
	g.buf.WriteTo(out)

	return 0
}

func getOutput(output string) *os.File {
	if output == "" {
		return os.Stdout
	}
	var err error
	f, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	return f
}

func isDirectory(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		log.Fatal(err)
	}
	return fi.IsDir()
}

func contains(list []string, key string) bool {
	for _, v := range list {
		if v == key {
			return true
		}
	}
	return false
}

func mergemap(m1, m2 map[string]*ast.StructType) map[string]*ast.StructType {
	ret := map[string]*ast.StructType{}
	for k, v := range m1 {
		ret[k] = v
	}
	for k, v := range m2 {
		ret[k] = v
	}
	return ret
}

func (g *generator) Printf(format string, args ...interface{}) (int, error) {
	return fmt.Fprintf(&g.buf, format, args...)
}

func parseDir(dir string) (string, []string) {
	p, err := build.Default.ImportDir(dir, 0)
	if err != nil {
		log.Fatal(err)
	}
	return p.Name, p.GoFiles
}

func (g *generator) parse(filenames []string) error {
	fs := token.NewFileSet()
	astFiles := []*ast.File{}
	for _, name := range filenames {
		parsedFile, err := parser.ParseFile(fs, filepath.Join(g.dir, name), nil, 0)
		if err != nil {
			log.Fatal(err)
		}
		astFiles = append(astFiles, parsedFile)
		var structMap map[string]*ast.StructType

		for _, decl := range parsedFile.Decls {
			structMap = mergemap(structMap, g.extractStruct(decl))
		}
		if err := g.generate(structMap); err != nil {
			log.Fatal(err)
		}
	}

	return nil
}

func (g *generator) extractStruct(decl ast.Decl) map[string]*ast.StructType {
	genDecl, ok := decl.(*ast.GenDecl)
	if !ok {
		return nil
	}
	if genDecl.Tok != token.TYPE {
		return nil
	}
	structMap := map[string]*ast.StructType{}
	for _, spec := range genDecl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		name := typeSpec.Name.Name
		if !contains(g.targetTypes, name) {
			continue
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		structMap[name] = structType
	}
	return structMap
}

func variablize(name string) string {
	vn := strings.ToLower(name[:1]) + name[1:]
	if vn == name {
		vn += "_"
	}
	return vn
}

func getFields(st *ast.StructType) []string {
	fields := []string{}
	for _, field := range st.Fields.List {
		fields = append(fields, fmt.Sprintf("&%s", field.Names[0].Name))
	}
	return fields
}

func (g *generator) generate(structMap map[string]*ast.StructType) error {
	g.Printf("package %s\n\n", g.packageName)
	g.Printf("import \"database/sql\"\n\n")
	for name, st := range structMap {
		g.Printf("func (%s *%s) Scan(sc interface{\n", variablize(name), name)
		g.Printf("\tScan func(...interface{}) error\n")
		g.Printf("}) error {\n")
		g.Printf("\treturn sc.Scan(%s)\n", strings.Join(getFields(st), ", "))
		g.Printf("}\n\n")
	}
	return nil
}
