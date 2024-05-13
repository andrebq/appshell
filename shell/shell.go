package shell

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/parser"
	"github.com/d5/tengo/v2/stdlib"
)

type (
	Shell struct {
		ctx     context.Context
		globals map[string]tengo.Object

		fmtMod     map[string]tengo.Object
		jsonrpcMod map[string]tengo.Object

		stdout, stderr proxyWriter
		stdin          proxyReader
		importsDir     string

		initRepl func()

		repl struct {
			constants []tengo.Object
			globals   []tengo.Object
			symbols   *tengo.SymbolTable
			fileset   *parser.SourceFileSet
		}
	}

	snapshotFormat struct {
		Data   map[string]json.RawMessage `json:"data"`
		Failed map[string]struct{}        `json:"failed"`
	}

	proxyWriter struct {
		w io.Writer
	}

	proxyReader struct {
		r io.Reader
	}

	emptyBuffer struct{}
)

func (p *proxyWriter) Write(buf []byte) (int, error) {
	return p.w.Write(buf)
}

func (p *proxyReader) Read(buf []byte) (int, error) {
	return p.r.Read(buf)
}

func (emptyBuffer) Read(buf []byte) (int, error) { return 0, io.EOF }

func New() *Shell {
	s := &Shell{
		ctx:     context.Background(),
		globals: make(map[string]tengo.Object),
		stdout:  proxyWriter{w: io.Discard},
		stderr:  proxyWriter{w: io.Discard},
		stdin:   proxyReader{r: emptyBuffer{}},
	}
	s.initRepl = sync.OnceFunc(s.prepareREPL)
	s.fmtMod = safeFmt(&s.stdout)
	return s
}

func (s *Shell) AllowImportFrom(dir string) {
	s.importsDir = dir
}

func (s *Shell) Parse(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	_, _, err := s.parseAST(parser.NewFileSet(), code)
	return code, err
}

func (s *Shell) parseAST(fileSet *parser.SourceFileSet, code string) (*parser.SourceFile, *parser.File, error) {
	srcFile := fileSet.AddFile("(repl)", -1, len(code))
	p := parser.NewParser(srcFile, []byte(code), nil)
	parsed, err := p.ParseFile()
	return srcFile, parsed, err
}

func (s *Shell) Eval(ctx context.Context, sout, serr io.Writer, code string, sin io.Reader) error {
	s.initRepl()

	updateShell := func(ctx context.Context) func() {
		// update shell sets the context and redirects io
		// to the given writers/readers
		//
		// then, returns an function that should be called to
		// undo, those changes
		old := s.ctx
		s.ctx = ctx

		oldsout := s.stdout.w
		oldserr := s.stdout.w
		oldsin := s.stdin.r

		s.stdout.w = sout
		s.stderr.w = serr
		s.stdin.r = sin

		return func() {
			s.ctx = old
			s.stdout.w = oldsout
			s.stderr.w = oldserr
			s.stdin.r = oldsin
		}
	}
	defer updateShell(ctx)()

	srcFile, file, err := s.parseAST(s.repl.fileset, code)
	if err != nil {
		return err
	}

	file = s.addPrints(file)
	c := tengo.NewCompiler(srcFile, s.repl.symbols, s.repl.constants, s.modules(), nil)
	if s.importsDir != "" {
		c.EnableFileImport(true)
		c.SetImportDir(s.importsDir)
	}
	if err := c.Compile(file); err != nil {
		return fmt.Errorf("tengo: compilation error %v", err)
	}

	bytecode := c.Bytecode()
	machine := tengo.NewVM(bytecode, s.repl.globals, -1)
	if err := machine.Run(); err != nil {
		return fmt.Errorf("tengo: eval error: %v", err)
	}
	s.repl.constants = bytecode.Constants
	return nil
}

func (s *Shell) addPrints(file *parser.File) *parser.File {
	var stmts []parser.Stmt
	for _, s := range file.Stmts {
		switch s := s.(type) {
		case *parser.ExprStmt:
			stmts = append(stmts, &parser.ExprStmt{
				Expr: &parser.CallExpr{
					Func: &parser.Ident{Name: "__repl_println__"},
					Args: []parser.Expr{s.Expr},
				},
			})
		case *parser.AssignStmt:
			stmts = append(stmts, s)

			stmts = append(stmts, &parser.ExprStmt{
				Expr: &parser.CallExpr{
					Func: &parser.Ident{
						Name: "__repl_println__",
					},
					Args: s.LHS,
				},
			})
		default:
			stmts = append(stmts, s)
		}
	}
	return &parser.File{
		InputFile: file.InputFile,
		Stmts:     stmts,
	}
}

func (s *Shell) prepareREPL() {
	globals := make([]tengo.Object, tengo.GlobalsSize)
	symbolTable := tengo.NewSymbolTable()
	for idx, fn := range tengo.GetAllBuiltinFunctions() {
		symbolTable.DefineBuiltin(idx, fn.Name)
	}

	// embed println function
	symbol := symbolTable.Define("__repl_println__")
	globals[symbol.Index] = &tengo.UserFunction{
		Name: "println",
		Value: func(args ...tengo.Object) (ret tengo.Object, err error) {
			var printArgs []interface{}
			for _, arg := range args {
				if _, isUndefined := arg.(*tengo.Undefined); isUndefined {
					// avoid printing undefined values
					continue
				} else {
					s, _ := tengo.ToString(arg)
					printArgs = append(printArgs, s)
				}
			}
			printArgs = append(printArgs, "\n")
			_, _ = fmt.Fprint(&s.stdout, printArgs...)
			return
		},
	}

	var constants []tengo.Object
	s.repl.constants = constants
	s.repl.globals = globals
	s.repl.symbols = symbolTable
	s.repl.fileset = parser.NewFileSet()
}

func (s *Shell) modules() tengo.ModuleGetter {
	mods := stdlib.GetModuleMap(safeModules...)
	mods.AddBuiltinModule("fmt", s.fmtMod)
	if s.jsonrpcMod != nil {
		mods.AddBuiltinModule("jsonrpc", s.jsonrpcMod)
	}
	return mods
}

func (s *Shell) Snapshot(ctx context.Context, out io.Writer) error {
	sp := snapshot{}
	sp.from(s.globals)
	output := snapshotFormat{
		Data:   make(map[string]json.RawMessage),
		Failed: make(map[string]struct{}),
	}
	for k, v := range sp.items {
		buf, err := json.Marshal(v)
		if err != nil {
			output.Failed[k] = struct{}{}
			continue
		}
		output.Data[k] = json.RawMessage(buf)
	}

	return json.NewEncoder(out).Encode(output)
}

func (s *Shell) RestoreSnapshot(ctx context.Context, in io.Reader) error {
	var input snapshotFormat
	err := json.NewDecoder(in).Decode(&input)
	if err != nil {
		return err
	}
	for k, v := range input.Data {
		var val any
		err = json.Unmarshal(v, &val)
		if err != nil {
			continue
		}
		s.globals[k], err = tengo.FromInterface(val)
		if err != nil {
			return err
		}
	}
	return nil
}
