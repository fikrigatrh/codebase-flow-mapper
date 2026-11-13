package routes

import (
	"go/ast"
	"go/types"
	"strings"
)

var fiberFramework = &framework{
	name:     "fiber",
	pkg:      "github.com/gofiber/fiber/v2",
	root:     "",
	types:    []string{"App", "Router", "Group"},
	classify: fiberClassify,
	join:     fiberJoin,
}

var fiberVerbs = map[string]string{"Get": "GET", "Head": "HEAD", "Post": "POST", "Put": "PUT", "Delete": "DELETE",
	"Connect": "CONNECT", "Options": "OPTIONS", "Trace": "TRACE", "Patch": "PATCH"}

func fiberClassify(fn *types.Func, call *ast.CallExpr) (rcall, bool) {
	args := call.Args
	recv := fn.Type().(*types.Signature).Recv()
	if recv == nil {
		if fn.Name() == "New" {
			return rcall{kind: opRoot, methodArg: -1, pathArg: -1}, true
		}
		return rcall{}, false
	}
	if routerType(recv.Type()) == nil {
		return rcall{}, false
	}
	switch name := fn.Name(); {
	case name == "Group":
		return rcall{kind: opGroup, methodArg: -1, pathArg: 0, handlers: argsFrom(args, 1)}, true
	case name == "Route":
		return rcall{kind: opGroup, methodArg: -1, pathArg: 0, route: len(args) > 1}, true
	case name == "Use":
		return rcall{kind: opUse, methodArg: -1, pathArg: -1, handlers: args, usePrefix: true}, true
	case name == "All":
		return rcall{kind: opRegister, methods: []string{"ANY"}, methodArg: -1, pathArg: 0, handlers: argsFrom(args, 1)}, true
	case name == "Add":
		return rcall{kind: opRegister, methodArg: 0, pathArg: 1, handlers: argsFrom(args, 2)}, true
	case fiberVerbs[name] != "":
		return rcall{kind: opRegister, methods: []string{fiberVerbs[name]}, methodArg: -1, pathArg: 0, handlers: argsFrom(args, 1)}, true
	}
	return rcall{}, false
}

// fiberJoin mirrors fiber's getGroupPath; an empty result is the root "/".
func fiberJoin(prefix, rel string) string {
	p := prefix
	if rel != "" {
		if rel[0] != '/' {
			rel = "/" + rel
		}
		p = strings.TrimRight(prefix, "/") + rel
	}
	if p == "" {
		return "/"
	}
	return p
}
