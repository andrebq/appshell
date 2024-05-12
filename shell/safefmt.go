package shell

import (
	"fmt"
	"io"

	"github.com/d5/tengo/v2"
)

var (
	safeModules = []string{
		"math",
		"text",
		"times",
		"rand",
		"json",
		"base64",
		"hex",
	}
)

func safeFmt(out io.Writer) map[string]tengo.Object {
	if out == nil {
		out = io.Discard
	}
	return map[string]tengo.Object{
		"print":   &tengo.UserFunction{Name: "print", Value: fmtPrint(out)},
		"printf":  &tengo.UserFunction{Name: "printf", Value: fmtPrintf(out)},
		"println": &tengo.UserFunction{Name: "println", Value: fmtPrintln(out)},
		"sprintf": &tengo.UserFunction{Name: "sprintf", Value: fmtSprintf},
	}
}

func fmtPrint(out io.Writer) func(args ...tengo.Object) (ret tengo.Object, err error) {
	return func(args ...tengo.Object) (ret tengo.Object, err error) {
		printArgs, err := getPrintArgs(args...)
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprint(out, printArgs...)
		return nil, nil
	}
}

func fmtPrintf(out io.Writer) func(args ...tengo.Object) (ret tengo.Object, err error) {
	return func(args ...tengo.Object) (ret tengo.Object, err error) {
		numArgs := len(args)
		if numArgs == 0 {
			return nil, tengo.ErrWrongNumArguments
		}

		format, ok := args[0].(*tengo.String)
		if !ok {
			return nil, tengo.ErrInvalidArgumentType{
				Name:     "format",
				Expected: "string",
				Found:    args[0].TypeName(),
			}
		}
		if numArgs == 1 {
			fmt.Print(format)
			return nil, nil
		}

		s, err := tengo.Format(format.Value, args[1:]...)
		if err != nil {
			return nil, err
		}
		fmt.Fprint(out, s)
		return nil, nil
	}
}

func fmtPrintln(out io.Writer) func(args ...tengo.Object) (ret tengo.Object, err error) {
	return func(args ...tengo.Object) (ret tengo.Object, err error) {
		printArgs, err := getPrintArgs(args...)
		if err != nil {
			return nil, err
		}
		printArgs = append(printArgs, "\n")
		_, _ = fmt.Fprintln(out, printArgs...)
		return nil, nil
	}
}

func fmtSprintf(args ...tengo.Object) (ret tengo.Object, err error) {
	numArgs := len(args)
	if numArgs == 0 {
		return nil, tengo.ErrWrongNumArguments
	}

	format, ok := args[0].(*tengo.String)
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{
			Name:     "format",
			Expected: "string",
			Found:    args[0].TypeName(),
		}
	}
	if numArgs == 1 {
		// okay to return 'format' directly as String is immutable
		return format, nil
	}
	s, err := tengo.Format(format.Value, args[1:]...)
	if err != nil {
		return nil, err
	}
	return &tengo.String{Value: s}, nil
}

func getPrintArgs(args ...tengo.Object) ([]interface{}, error) {
	var printArgs []interface{}
	l := 0
	for _, arg := range args {
		s, _ := tengo.ToString(arg)
		slen := len(s)
		// make sure length does not exceed the limit
		if l+slen > tengo.MaxStringLen {
			return nil, tengo.ErrStringLimit
		}
		l += slen
		printArgs = append(printArgs, s)
	}
	return printArgs, nil
}
