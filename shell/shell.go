package shell

import (
	"context"
	"encoding/json"
	"io"
	"strings"

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
	s.fmtMod = safeFmt(&s.stdout)
	return s
}

func (s *Shell) AllowImportFrom(dir string) {
	s.importsDir = dir
}

func (s *Shell) Parse(ctx context.Context, code string) (string, error) {
	code = strings.TrimSpace(code)
	fileSet := parser.NewFileSet()
	srcFile := fileSet.AddFile("(main)", -1, len(code))
	p := parser.NewParser(srcFile, []byte(code), nil)
	_, err := p.ParseFile()
	return code, err
}

func (s *Shell) Eval(ctx context.Context, sout, serr io.Writer, code string, sin io.Reader) error {
	defer func(ctx context.Context) func() {
		old := s.ctx
		s.ctx = ctx
		return func() { s.ctx = old }
	}(ctx)

	script := tengo.NewScript([]byte(code))
	for k, v := range s.globals {
		script.Add(k, v)
	}

	s.stdout.w = sout
	s.stderr.w = serr
	s.stdin.r = sin

	script.SetImports(s.modules())
	if s.importsDir != "" {
		script.EnableFileImport(true)
		script.SetImportDir(s.importsDir)
	} else {
		script.EnableFileImport(false)
	}
	output, err := script.RunContext(ctx)
	if err != nil {
		return err
	}
	for _, v := range output.GetAll() {
		s.globals[v.Name()] = v.Object()
	}
	return nil
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
