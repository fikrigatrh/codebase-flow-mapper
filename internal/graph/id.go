package graph

import (
	"fmt"
	"go/token"
	"path/filepath"
	"strconv"
)

type NodeID string

// Sym names a Go function or method: Recv is "" for plain functions, else e.g. "*txService".
type Sym struct {
	Pkg, Recv, Name string
}

func (s Sym) qualified() string {
	if s.Recv == "" {
		return s.Pkg + "." + s.Name
	}
	return s.Pkg + ".(" + s.Recv + ")." + s.Name
}

func FuncID(s Sym) NodeID     { return NodeID("fn:" + s.qualified()) }
func ExternalID(s Sym) NodeID { return NodeID("external:" + s.qualified()) }

// LiteralID numbers func literals inside a parent function like SSA does ($1, $2, …).
func LiteralID(parent NodeID, n int) NodeID { return NodeID(string(parent) + "$" + strconv.Itoa(n)) }

func EndpointID(framework, method, path string, reg Loc) NodeID {
	return NodeID(fmt.Sprintf("endpoint:%s:%s:%s:%s:%d", framework, method, path, reg.File, reg.Line))
}

func UnresolvedID(at Loc) NodeID {
	return NodeID(fmt.Sprintf("unresolved:%s:%d:%d", at.File, at.Line, at.Col))
}

// SiteKey joins a call edge to its flow step. Both stages must derive the position the same way:
// ssa.Call → CallExpr.Lparen, ssa.Go/ssa.Defer → the go/defer keyword (= ssa CallInstruction.Pos()).
func SiteKey(at Loc) string { return fmt.Sprintf("%s:%d:%d", at.File, at.Line, at.Col) }

// LocOf converts a token position to a Loc relative to root. Files outside root keep their
// absolute path; Validate rejects those unless an editor root is set, so a bug can't leak them silently.
func LocOf(p token.Position, root string) Loc {
	file := p.Filename
	if rel, err := filepath.Rel(root, p.Filename); err == nil && filepath.IsLocal(rel) {
		file = filepath.ToSlash(rel)
	}
	return Loc{File: file, Line: p.Line, Col: p.Column}
}
