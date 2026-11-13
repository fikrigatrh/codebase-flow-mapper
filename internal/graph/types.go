// Package graph holds the report graph (GRAPH_SCHEMA.md): types, IDs, validation and encoding.
package graph

import "encoding/json"

const SchemaVersion = "2.1.0"

type Graph struct {
	SchemaVersion   string       `json:"schemaVersion"`
	AnalyzerVersion string       `json:"analyzerVersion"`
	Module          string       `json:"module"`
	GoVersion       string       `json:"goVersion"`
	GeneratedAt     string       `json:"generatedAt"`
	EditorRoot      *string      `json:"editorRoot"`
	Nodes           []Node       `json:"nodes"`
	Edges           []Edge       `json:"edges"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
	Stats           Stats        `json:"stats"`
}

type Stats struct {
	Packages   int   `json:"packages"`
	Functions  int   `json:"functions"`
	Endpoints  int   `json:"endpoints"`
	Edges      int   `json:"edges"`
	Unresolved int   `json:"unresolved"`
	ElapsedMs  int64 `json:"elapsedMs"`
	Blockers   []int `json:"blockers"` // indexes into Graph.Diagnostics
}

// Loc is a source span; File is relative to the module root.
type Loc struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col,omitempty"`
	EndLine int    `json:"endLine,omitempty"`
}

type (
	NodeKind   string
	Status     string
	EdgeKind   string
	Confidence string
	Method     string
	Severity   string
)

const (
	KindEndpoint   NodeKind = "endpoint"
	KindFunction   NodeKind = "function"
	KindExternal   NodeKind = "external"
	KindUnresolved NodeKind = "unresolved"

	StatusResolved          Status = "resolved"
	StatusPartiallyResolved Status = "partially_resolved"
	StatusUnresolved        Status = "unresolved"

	EdgeRoutesTo       EdgeKind = "routes_to"
	EdgeUsesMiddleware EdgeKind = "uses_middleware"
	EdgeCalls          EdgeKind = "calls"

	ConfHigh    Confidence = "high"
	ConfMedium  Confidence = "medium"
	ConfLow     Confidence = "low"
	ConfUnknown Confidence = "unknown"

	MethodStatic                Method = "static"
	MethodVTA                   Method = "vta"
	MethodFrameworkRegistration Method = "framework_registration"
	MethodWrapperHeuristic      Method = "wrapper_heuristic"
	MethodUnresolved            Method = "unresolved"

	SevInfo    Severity = "info"
	SevWarning Severity = "warning"
	SevError   Severity = "error"
)

// Node is flat: kind-specific fields are empty (and omitted) for other kinds.
type Node struct {
	ID     NodeID   `json:"id"`
	Kind   NodeKind `json:"kind"`
	Name   string   `json:"name"`
	Pkg    string   `json:"pkg,omitempty"`
	Loc    *Loc     `json:"loc,omitempty"`
	Status Status   `json:"status"`

	// endpoint
	Framework string `json:"framework,omitempty"`
	Method    string `json:"method,omitempty"`
	Path      string `json:"path,omitempty"`

	// function
	Receiver  string `json:"receiver,omitempty"`
	Signature string `json:"signature,omitempty"`
	Flow      []Step `json:"flow,omitempty"`
	Body      *Body  `json:"body,omitempty"` // the request body this function binds (CFM-401)

	// external
	Category string `json:"category,omitempty"`

	// unresolved
	Reason string `json:"reason,omitempty"`
}

type Edge struct {
	Source       NodeID     `json:"source"`
	Target       NodeID     `json:"target"`
	Kind         EdgeKind   `json:"kind"`
	Confidence   Confidence `json:"confidence"`
	Method       Method     `json:"method"`
	Reason       string     `json:"reason"`
	Site         Loc        `json:"site"`
	SiteID       string     `json:"siteId,omitempty"`       // calls only
	Order        *int       `json:"order,omitempty"`        // uses_middleware only
	Alternatives int        `json:"alternatives,omitempty"` // low-confidence vta only
}

type Diagnostic struct {
	Severity Severity          `json:"severity"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Loc      *Loc              `json:"loc,omitempty"`
	NodeID   NodeID            `json:"nodeId,omitempty"`
	Details  map[string]string `json:"details,omitempty"` // e.g. "fix": "wire ./..."
}

// Step kinds for Node.Flow (GRAPH_SCHEMA §5).
const (
	StepCall   = "call"
	StepIf     = "if"
	StepSwitch = "switch"
	StepLoop   = "loop"
	StepSelect = "select"
	StepGo     = "go"
	StepDefer  = "defer"
	StepReturn = "return"
	StepPanic  = "panic"
)

type Step struct {
	K     string `json:"k"`
	Site  string `json:"site,omitempty"` // call/go/defer: SiteKey, matches Edge.SiteID
	Line  int    `json:"line"`
	Cond  string `json:"cond,omitempty"`
	Tag   string `json:"tag,omitempty"`
	Head  string `json:"head,omitempty"`
	Then  []Step `json:"then,omitempty"`
	Else  []Step `json:"else,omitempty"`
	Cases []Case `json:"cases,omitempty"`
	Body  []Step `json:"body,omitempty"`
	Test  *Test  `json:"test,omitempty"` // if: the condition, when it reads a bound body type
}

type Case struct {
	Label string `json:"label"`
	Body  []Step `json:"body"`
	Test  *Test  `json:"test,omitempty"` // when the case (or default) depends on a bound body type
}

// Body is a bound request body (GRAPH_SCHEMA §8).
type Body struct {
	Type   string  `json:"type"`   // qualified Go type, e.g. example.com/app/dto.TopupReq
	Binder string  `json:"binder"` // ShouldBindJSON, BodyParser, Decode, …
	Line   int     `json:"line"`
	Fields []Field `json:"fields"`
}

// Field kinds.
const (
	FieldString  = "string"
	FieldInteger = "integer"
	FieldNumber  = "number"
	FieldBoolean = "boolean"
	FieldObject  = "object"
	FieldArray   = "array"
	FieldAny     = "any"
)

type Field struct {
	Name     string  `json:"name,omitempty"` // JSON name; empty for an array element
	Kind     string  `json:"kind"`
	Type     string  `json:"type,omitempty"` // qualified Go type for named types
	Nullable bool    `json:"nullable,omitempty"`
	Fields   []Field `json:"fields,omitempty"` // object
	Elem     *Field  `json:"elem,omitempty"`   // array
}

// Test ops: and/or/not take Args; eq…ge compare Ref with Value; truthy reads a bool Ref; unknown
// is a part of the condition CFM can't evaluate.
const (
	OpAnd     = "and"
	OpOr      = "or"
	OpNot     = "not"
	OpEq      = "eq"
	OpNe      = "ne"
	OpLt      = "lt"
	OpLe      = "le"
	OpGt      = "gt"
	OpGe      = "ge"
	OpTruthy  = "truthy"
	OpUnknown = "unknown"
)

type Test struct {
	Op    string          `json:"op"`
	Args  []Test          `json:"args,omitempty"`
	Ref   *Ref            `json:"ref,omitempty"`
	Value json.RawMessage `json:"value,omitempty"` // JSON constant; null for nil
}

// Ref is a field path inside a bound body type, with string transforms applied in order.
type Ref struct {
	Type string   `json:"type"`
	Path []string `json:"path"`
	Fold []string `json:"fold,omitempty"` // upper, lower, trim
}
