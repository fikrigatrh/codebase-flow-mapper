package routes

import (
	"go/ast"
	"go/types"
	"path"
	"slices"
	"strings"
)

var ginFramework = &framework{
	name:     "gin",
	pkg:      "github.com/gin-gonic/gin",
	root:     "/",
	types:    []string{"Engine", "RouterGroup", "IRouter", "IRoutes"},
	classify: ginClassify,
	join:     ginJoin,
}

var ginVerbs = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"}

func ginClassify(fn *types.Func, call *ast.CallExpr) (rcall, bool) {
	args := call.Args
	from := func(i int) []ast.Expr { return argsFrom(args, i) }
	recv := fn.Type().(*types.Signature).Recv()
	if recv == nil {
		if fn.Name() == "New" || fn.Name() == "Default" {
			return rcall{kind: opRoot, methodArg: -1, pathArg: -1}, true
		}
		return rcall{}, false
	}
	if routerType(recv.Type()) == nil {
		return rcall{}, false
	}
	switch name := fn.Name(); {
	case name == "Group":
		return rcall{kind: opGroup, methodArg: -1, pathArg: 0, handlers: from(1)}, true
	case name == "Use":
		return rcall{kind: opUse, methodArg: -1, pathArg: -1, handlers: args}, true
	case name == "Any":
		return rcall{kind: opRegister, methods: []string{"ANY"}, methodArg: -1, pathArg: 0, handlers: from(1)}, true
	case name == "Handle" || name == "Match":
		return rcall{kind: opRegister, methodArg: 0, pathArg: 1, handlers: from(2)}, true
	case slices.Contains(ginVerbs, name):
		return rcall{kind: opRegister, methods: []string{name}, methodArg: -1, pathArg: 0, handlers: from(1)}, true
	}
	return rcall{}, false
}

// ginJoin mirrors gin's joinPaths: path.Join, keeping a trailing slash of the relative path.
func ginJoin(prefix, rel string) string {
	if rel == "" {
		return prefix
	}
	final := path.Join(prefix, rel)
	if strings.HasSuffix(rel, "/") && !strings.HasSuffix(final, "/") {
		final += "/"
	}
	return final
}
