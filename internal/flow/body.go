package flow

// Request bodies (CFM-401, GRAPH_SCHEMA §8): which functions bind a body and into what type, and
// which conditions compare a field of such a type with a constant. The viewer evaluates the tests
// against a body the user types in; CFM itself never evaluates anything.

import (
	"encoding/json"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strings"

	"github.com/fikrigatrh/codebase-flow-mapper/internal/graph"
)

const maxDepth = 6 // nesting kept in a body schema; deeper objects show as "object" with no fields

// binders are the methods (package path + name) whose first argument receives the request body.
var binders = map[string]bool{
	"github.com/gin-gonic/gin.ShouldBindJSON": true,
	"github.com/gin-gonic/gin.BindJSON":       true,
	"github.com/gin-gonic/gin.ShouldBind":     true,
	"github.com/gin-gonic/gin.Bind":           true,
	"github.com/gofiber/fiber/v2.BodyParser":  true,
}

// body returns the first body bind in list, or nil. Func literals are their own functions.
// ponytail: a decoder kept in a variable (dec := json.NewDecoder(r.Body)) or io.ReadAll +
// json.Unmarshal is not recognized; match those call shapes too if real handlers use them.
func (w *walker) body(list []ast.Stmt) *graph.Body {
	var out *graph.Body
	for _, s := range list {
		ast.Inspect(s, func(n ast.Node) bool {
			if out != nil {
				return false
			}
			if _, lit := n.(*ast.FuncLit); lit {
				return false
			}
			if c, ok := n.(*ast.CallExpr); ok {
				out = w.bind(c)
			}
			return out == nil
		})
	}
	return out
}

func (w *walker) bind(c *ast.CallExpr) *graph.Body {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok || len(c.Args) == 0 {
		return nil
	}
	fn, _ := w.info.Uses[sel.Sel].(*types.Func)
	if fn == nil || fn.Pkg() == nil || fn.Type().(*types.Signature).Recv() == nil {
		return nil
	}
	name := fn.Pkg().Path() + "." + fn.Name()
	if !binders[name] && !(name == "encoding/json.Decode" && w.decodesRequest(sel.X)) {
		return nil
	}
	t := deref(w.info.TypeOf(c.Args[0]))
	named, _ := t.(*types.Named)
	if named == nil {
		return nil // a map or an anonymous struct: nothing a condition can refer to by type
	}
	root := schema(t, map[*types.Named]bool{}, 0)
	if root.Kind != graph.FieldObject {
		return nil
	}
	return &graph.Body{Type: root.Type, Binder: fn.Name(), Line: w.line(c.Lparen), Fields: root.Fields}
}

// decodesRequest: x is json.NewDecoder(r.Body) with r an *http.Request.
func (w *walker) decodesRequest(x ast.Expr) bool {
	c, ok := ast.Unparen(x).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return false
	}
	s, ok := ast.Unparen(c.Args[0]).(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "Body" {
		return false
	}
	return qualified(deref(w.info.TypeOf(s.X))) == "net/http.Request"
}

func deref(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// qualified is "pkgpath.Name" for a named type, "" otherwise.
func qualified(t types.Type) string {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return ""
	}
	return n.Obj().Pkg().Path() + "." + n.Obj().Name()
}

var unmarshaler = types.NewInterfaceType([]*types.Func{types.NewFunc(token.NoPos, nil, "UnmarshalJSON",
	types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewParam(token.NoPos, nil, "", types.NewSlice(types.Typ[types.Byte]))),
		types.NewTuple(types.NewParam(token.NoPos, nil, "", types.Universe.Lookup("error").Type())), false))}, nil).Complete()

// schema describes how encoding/json reads into t.
func schema(t types.Type, seen map[*types.Named]bool, depth int) graph.Field {
	t = types.Unalias(t)
	f := graph.Field{Type: qualified(t)}
	if f.Type == "time.Time" {
		f.Kind = graph.FieldString
		return f
	}
	if types.Implements(t, unmarshaler) || types.Implements(types.NewPointer(t), unmarshaler) {
		f.Kind = graph.FieldAny // custom JSON (decimal, enum…): shape unknown
		return f
	}
	switch u := t.Underlying().(type) {
	case *types.Basic:
		switch {
		case u.Info()&types.IsString != 0:
			f.Kind = graph.FieldString
		case u.Info()&types.IsBoolean != 0:
			f.Kind = graph.FieldBoolean
		case u.Info()&types.IsInteger != 0:
			f.Kind = graph.FieldInteger
		case u.Info()&types.IsFloat != 0:
			f.Kind = graph.FieldNumber
		default:
			f.Kind = graph.FieldAny
		}
	case *types.Pointer:
		f = schema(u.Elem(), seen, depth)
		f.Nullable = true
	case *types.Slice, *types.Array:
		elem := u.(interface{ Elem() types.Type }).Elem()
		if b, ok := elem.Underlying().(*types.Basic); ok && b.Kind() == types.Byte {
			f.Kind = graph.FieldString // []byte is base64
			break
		}
		f.Kind = graph.FieldArray
		if depth < maxDepth {
			e := schema(elem, seen, depth+1)
			f.Elem = &e
		}
		_, f.Nullable = u.(*types.Slice)
	case *types.Map:
		f.Kind, f.Nullable = graph.FieldObject, true
	case *types.Struct:
		f.Kind = graph.FieldObject
		n, _ := t.(*types.Named)
		if depth >= maxDepth || (n != nil && seen[n]) {
			break
		}
		if n != nil {
			seen[n] = true
			defer delete(seen, n)
		}
		f.Fields = structFields(u, seen, depth)
	default:
		f.Kind, f.Nullable = graph.FieldAny, true // interface, func, chan
	}
	return f
}

// structFields: exported fields under their json names; untagged embedded structs are flattened.
// ponytail: Go's rule that a shallower field hides a deeper one of the same name is not applied.
func structFields(s *types.Struct, seen map[*types.Named]bool, depth int) []graph.Field {
	var out []graph.Field
	for i := range s.NumFields() {
		v := s.Field(i)
		name, ok := jsonName(v, s.Tag(i))
		if !ok {
			continue
		}
		if v.Embedded() && name == "" {
			if st, ok := deref(v.Type()).Underlying().(*types.Struct); ok {
				out = append(out, structFields(st, seen, depth)...)
			}
			continue
		}
		if name == "" {
			name = v.Name()
		}
		f := schema(v.Type(), seen, depth+1)
		f.Name = name
		out = append(out, f)
	}
	return out
}

// jsonName is the field's json tag name ("" when untagged); ok is false when encoding/json skips it.
func jsonName(v *types.Var, tag string) (string, bool) {
	name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ",")
	if name == "-" || (!v.Exported() && !v.Embedded()) {
		return "", false
	}
	return name, true
}

// dto is the qualified name of t (or *t) when it is a bound body type, else "".
func (w *walker) dto(t types.Type) string {
	if t == nil {
		return ""
	}
	if q := qualified(deref(t)); w.dtos[q] {
		return q
	}
	return ""
}

var folds = map[string]string{"strings.ToUpper": "upper", "strings.ToLower": "lower", "strings.TrimSpace": "trim"}

// ref resolves e to a field path in a bound body type, through parens, derefs and string folds
// (strings.ToUpper(strings.TrimSpace(x)) → fold [trim upper]).
func (w *walker) ref(e ast.Expr) *graph.Ref {
	var fold, path []string
	for {
		c, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok {
			break
		}
		f := folds[w.funcName(c)]
		if f == "" || len(c.Args) != 1 {
			return nil
		}
		fold = append([]string{f}, fold...)
		e = c.Args[0]
	}
	// Walk down to the outermost bound type: req.Source.Currency is TopupReq.source.currency, not
	// Amount.currency, which would be ambiguous if TopupReq held two Amounts.
	var best *graph.Ref
	for {
		e = ast.Unparen(e)
		if st, ok := e.(*ast.StarExpr); ok {
			e = st.X
			continue
		}
		if q := w.dto(w.info.TypeOf(e)); q != "" && len(path) > 0 {
			best = &graph.Ref{Type: q, Path: slices.Clone(path), Fold: fold}
			slices.Reverse(best.Path)
		}
		s, ok := e.(*ast.SelectorExpr)
		if !ok {
			return best
		}
		sel := w.info.Selections[s]
		if sel == nil || sel.Kind() != types.FieldVal {
			return best
		}
		name, ok := w.fieldJSON(sel)
		if !ok {
			return best
		}
		path = append(path, name)
		e = s.X
	}
}

// fieldJSON is the JSON name of the field a selection picks (the last struct on its index path).
func (w *walker) fieldJSON(sel *types.Selection) (string, bool) {
	t := sel.Recv()
	idx := sel.Index()
	for k, i := range idx {
		st, ok := deref(t).Underlying().(*types.Struct)
		if !ok {
			return "", false
		}
		if k == len(idx)-1 {
			name, ok := jsonName(st.Field(i), st.Tag(i))
			if name == "" {
				name = st.Field(i).Name()
			}
			return name, ok
		}
		t = st.Field(i).Type()
	}
	return "", false
}

// funcName is "pkgpath.Name" for a call of a package-level function, else "".
func (w *walker) funcName(c *ast.CallExpr) string {
	var id *ast.Ident
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	}
	if id == nil {
		return ""
	}
	fn, ok := w.info.Uses[id].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Type().(*types.Signature).Recv() != nil {
		return ""
	}
	return fn.Pkg().Path() + "." + fn.Name()
}

// value is e as a JSON constant, or nil when e is not a constant.
func (w *walker) value(e ast.Expr) json.RawMessage {
	tv := w.info.Types[e]
	if tv.IsNil() {
		return json.RawMessage("null")
	}
	if tv.Value == nil {
		return nil
	}
	var v any
	switch tv.Value.Kind() {
	case constant.String:
		v = constant.StringVal(tv.Value)
	case constant.Bool:
		v = constant.BoolVal(tv.Value)
	case constant.Int:
		return json.RawMessage(tv.Value.ExactString())
	case constant.Float:
		v, _ = constant.Float64Val(tv.Value)
	default:
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

var compareOps = map[token.Token]string{token.EQL: graph.OpEq, token.NEQ: graph.OpNe, token.LSS: graph.OpLt,
	token.LEQ: graph.OpLe, token.GTR: graph.OpGt, token.GEQ: graph.OpGe}

// flipped is the op with its operands swapped ("SGD" == x → x == "SGD", 0 < x → x > 0).
var flipped = map[string]string{graph.OpEq: graph.OpEq, graph.OpNe: graph.OpNe, graph.OpLt: graph.OpGt,
	graph.OpLe: graph.OpGe, graph.OpGt: graph.OpLt, graph.OpGe: graph.OpLe}

// test is cond as a body test, or nil when no part of it reads a bound body type.
func (w *walker) test(cond ast.Expr) *graph.Test {
	if len(w.dtos) == 0 || cond == nil {
		return nil
	}
	switch e := ast.Unparen(cond).(type) {
	case *ast.BinaryExpr:
		if e.Op == token.LAND || e.Op == token.LOR {
			a, b := w.test(e.X), w.test(e.Y)
			if a == nil && b == nil {
				return nil
			}
			op := graph.OpAnd
			if e.Op == token.LOR {
				op = graph.OpOr
			}
			return &graph.Test{Op: op, Args: []graph.Test{orUnknown(a), orUnknown(b)}}
		}
		if op, ok := compareOps[e.Op]; ok {
			return w.compare(op, e.X, e.Y)
		}
	case *ast.UnaryExpr:
		if e.Op == token.NOT {
			if t := w.test(e.X); t != nil {
				return &graph.Test{Op: graph.OpNot, Args: []graph.Test{*t}}
			}
		}
	case *ast.CallExpr:
		if w.funcName(e) == "strings.EqualFold" && len(e.Args) == 2 {
			t := w.compare(graph.OpEq, e.Args[0], e.Args[1])
			if t != nil && t.Op == graph.OpEq {
				t.Ref.Fold = append(t.Ref.Fold, "lower")
				var s string
				if json.Unmarshal(t.Value, &s) == nil {
					t.Value, _ = json.Marshal(strings.ToLower(s))
				}
			}
			return t
		}
	default:
		if r := w.ref(e); r != nil && isBool(w.info.TypeOf(e)) {
			return &graph.Test{Op: graph.OpTruthy, Ref: r}
		}
	}
	return nil
}

// compare: one side a body field, the other a constant. A body field compared with anything else
// is unknown; no body field at all is nil.
func (w *walker) compare(op string, x, y ast.Expr) *graph.Test {
	if r := w.ref(x); r != nil {
		if v := w.value(y); v != nil {
			return &graph.Test{Op: op, Ref: r, Value: v}
		}
		return &graph.Test{Op: graph.OpUnknown}
	}
	if r := w.ref(y); r != nil {
		if v := w.value(x); v != nil {
			return &graph.Test{Op: flipped[op], Ref: r, Value: v}
		}
		return &graph.Test{Op: graph.OpUnknown}
	}
	return nil
}

func isBool(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsBoolean != 0
}

func orUnknown(t *graph.Test) graph.Test {
	if t == nil {
		return graph.Test{Op: graph.OpUnknown}
	}
	return *t
}

// caseTests returns a test per clause of a switch: tag == item for a tagged switch, the item
// itself for a tagless one, and for default "no other case matched". Pruned clauses still count,
// so default stays right when the body matches a case that has no calls. nil when nothing reads
// a body.
func (w *walker) caseTests(tag ast.Expr, body *ast.BlockStmt) []*graph.Test {
	if len(w.dtos) == 0 {
		return nil
	}
	if tag != nil && w.ref(tag) == nil {
		return nil
	}
	tests := make([]*graph.Test, len(body.List))
	found, dflt := false, -1
	for i, cc := range body.List {
		c := cc.(*ast.CaseClause)
		if c.List == nil {
			dflt = i
			continue
		}
		var items []graph.Test
		hit := false
		for _, e := range c.List {
			var t *graph.Test
			if tag != nil {
				t = w.compare(graph.OpEq, tag, e)
			} else {
				t = w.test(e)
			}
			hit = hit || t != nil
			items = append(items, orUnknown(t))
		}
		if !hit {
			continue
		}
		found = true
		if len(items) == 1 {
			tests[i] = &items[0]
		} else {
			tests[i] = &graph.Test{Op: graph.OpOr, Args: items}
		}
	}
	if !found {
		return nil
	}
	if dflt >= 0 {
		var none []graph.Test
		for i, t := range tests {
			if i != dflt {
				none = append(none, graph.Test{Op: graph.OpNot, Args: []graph.Test{orUnknown(t)}})
			}
		}
		tests[dflt] = &graph.Test{Op: graph.OpAnd, Args: none}
	}
	return tests
}
