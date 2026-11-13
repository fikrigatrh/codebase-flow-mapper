// Package graph holds the report graph (GRAPH_SCHEMA.md): types, IDs, validation and encoding.
package graph

const SchemaVersion = "2.0.0"

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
}

type Case struct {
	Label string `json:"label"`
	Body  []Step `json:"body"`
}
