package constant

type CallGraphType string

const (
	CallGraphTypeStatic  CallGraphType = "static"
	CallGraphTypeCha                   = "cha"
	CallGraphTypeRta                   = "rta"
	CallGraphTypePointer               = "pointer"
)
