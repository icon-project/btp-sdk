package types

type ResultType int

type InvokeResult interface {
}

type QueryResult interface {
}

type Parameters interface {
	Get(name string) interface{}
	Keys() []string
}

type Request interface {
	Parameters() Parameters
	GetOption(key string) string
}

type ServiceHandler interface {
	GetSpec() *ServiceSpec
	Invoke(Request) (InvokeResult, error)
	Query(Request) (QueryResult, error)
}
