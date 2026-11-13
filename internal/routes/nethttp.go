package routes

import (
	"go/ast"
	"go/types"
	"strings"
)

var netHTTPFramework = &framework{
	name:     "nethttp",
	pkg:      "net/http",
	root:     "",
	types:    []string{"ServeMux"},
	classify: netHTTPClassify,
	join:     func(prefix, rel string) string { return prefix + rel },
	pattern:  netHTTPPattern,
	unwrap:   true,
}

func netHTTPClassify(fn *types.Func, call *ast.CallExpr) (rcall, bool) {
	recv := fn.Type().(*types.Signature).Recv()
	switch name := fn.Name(); {
	case recv == nil && name == "NewServeMux":
		return rcall{kind: opRoot, methodArg: -1, pathArg: -1}, true
	case name == "Handle" || name == "HandleFunc":
		if recv != nil && routerType(recv.Type()) == nil {
			return rcall{}, false
		}
		return rcall{kind: opRegister, methods: []string{"ANY"}, methodArg: -1, pathArg: 0,
			handlers: argsFrom(call.Args, 1), implicit: recv == nil}, true
	}
	return rcall{}, false
}

// netHTTPPattern splits a Go 1.22 pattern "[METHOD ][HOST]/[PATH]"; no method means ANY.
// The host, if any, stays in the path.
func netHTTPPattern(p string) ([]string, string) {
	if i := strings.IndexAny(p, " \t"); i > 0 && !strings.Contains(p[:i], "/") {
		return []string{p[:i]}, strings.TrimLeft(p[i:], " \t")
	}
	return []string{"ANY"}, p
}
