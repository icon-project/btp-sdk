package types

type TypeTag int64

const (
	TUnknown TypeTag = iota
	TVoid
	TInteger
	TBoolean
	TString
	TBytes
	TStruct
)

const (
	listDepthOffset = 4
	listDepthBits   = 4
	listDepthMask   = (1 << listDepthBits) - 1

	valueTagBits = listDepthOffset
	valueTagMask = (1 << valueTagBits) - 1
)

type DataType int64

func (dt DataType) Tag() TypeTag {
	return TypeTag(dt & valueTagMask)
}

type TypeSpec struct {
	TypeName string

	typeID   DataType
	resolved *StructSpec
}

type TypeAndName struct {
	Type TypeSpec
	Name string
}

type MethodSpec struct {
	Name     string
	Inputs   []TypeAndName
	Output   TypeSpec
	Payable  bool
	ReadOnly bool
}

type EventSpec struct {
	Name    string
	Indexed int
	Inputs  []TypeAndName
}

type StructSpec struct {
	name   string
	fields []TypeAndName
}

type ServiceSpec struct {
	SpecVersion string       `json:"specVersion"`
	Name        string       `json:"name"`
	Methods     []MethodSpec `json:"methods"`
	Events      []EventSpec  `json:"events"`
	Structs     []StructSpec `json:"structs"`
}
