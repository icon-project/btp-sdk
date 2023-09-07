/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/icon-project/btp2/common/log"

	"github.com/icon-project/btp-sdk/autocaller"
	"github.com/icon-project/btp-sdk/contract"
	"github.com/icon-project/btp-sdk/service"
)

const (
	openapi3Version     = "3.0.3"
	infoTitlePrefix     = "BTP SDK "
	infoTitleSuffix     = " - OpenAPI " + openapi3Version
	infoDefaultVersion  = "0.1.0"
	tagReadonly         = "Readonly"
	tagWritable         = "Writable"
	tagGeneral          = "General"
	tagAutoCaller       = "AutoCaller"
	schemaRefPrefix     = "#/components/schemas/"
	schemaTxID          = "TxID"
	schemaBlockID       = "BlockID"
	schemaRequest       = "Request"
	schemaOptions       = "Options"
	schemaErrorResponse = "ErrorResponse"
	schemaTypeInfo      = "TypeInfo"
	schemaMethodInfo    = "MethodInfo"
	schemaMethodInfos   = "MethodInfos"
	parameterRefPrefix  = "#/components/parameters/"
)

var (
	methodNameRegexp  = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	infoLicenseApache = &openapi3.License{
		Name: "Apache 2.0",
		URL:  "http://www.apache.org/licenses/LICENSE-2.0.html",
	}
	externalDocs = &openapi3.ExternalDocs{
		Description: "Find out more about BTP SDK",
		URL:         "https://github.com/icon-project/btp-sdk",
	}
	integerSchema = openapi3.NewOneOfSchema(
		openapi3.NewStringSchema().WithPattern("^(0x|\\-0x)(0|[1-9a-f][0-9a-f]*)$"),
		openapi3.NewStringSchema().WithPattern("^(|\\-)(0|[1-9][0-9]*)$"),
		openapi3.NewIntegerSchema(),
	)
	booleanSchema  = openapi3.NewBoolSchema()
	stringSchema   = openapi3.NewStringSchema()
	bytesSchema    = openapi3.NewBytesSchema()
	addressSchema  = openapi3.NewStringSchema().WithFormat(contract.TAddress.String())
	defaultSchemas = map[string]*openapi3.Schema{
		schemaTxID: openapi3.NewOneOfSchema(
			openapi3.NewStringSchema().WithPattern("^0x([0-9a-f][0-9a-f])*$"),
			openapi3.NewBytesSchema()),
		schemaBlockID: openapi3.NewOneOfSchema(
			openapi3.NewStringSchema().WithPattern("^0x([0-9a-f][0-9a-f])*$"),
			openapi3.NewBytesSchema()),
		schemaRequest:              MustGenerateSchema(&Request{}),
		schemaOptions:              openapi3.NewObjectSchema(),
		schemaErrorResponse:        MustGenerateSchema(&ErrorResponse{}),
		schemaTypeInfo:             MustGenerateSchema(&TypeInfo{}),
		schemaMethodInfo:           MustGenerateSchema(&MethodInfo{}),
		schemaMethodInfos:          MustGenerateSchema(&MethodInfos{}),
		contract.TInteger.String(): integerSchema,
		contract.TBoolean.String(): booleanSchema,
		contract.TString.String():  stringSchema,
		contract.TBytes.String():   bytesSchema,
		contract.TAddress.String(): addressSchema,
	}
	defaultTags = openapi3.Tags{
		NewTag(tagReadonly, "Readonly service method"),
		NewTag(tagWritable, "Writable service method"),
	}
	schemaNameReplacer = strings.NewReplacer("$", ".")
)

func MustGenerateSchema(v interface{}) *openapi3.Schema {
	ref, err := openapi3gen.NewSchemaRefForValue(v, nil)
	if err != nil {
		log.Panicf("%+v", err)
	}
	return ref.Value
}

func DefaultSchemaRef(name string) *openapi3.SchemaRef {
	if s, ok := defaultSchemas[name]; ok {
		return openapi3.NewSchemaRef(schemaRefPrefix+name, s)
	}
	return nil
}

func NewSchemas() openapi3.Schemas {
	schemas := make(openapi3.Schemas)
	for k, s := range defaultSchemas {
		schemas[k] = s.NewRef()
	}
	return schemas
}

func NewTags() openapi3.Tags {
	tags := make(openapi3.Tags, len(defaultTags))
	copy(tags, defaultTags)
	return tags
}

func TagsIndex(ts openapi3.Tags, name string) int {
	for i, t := range ts {
		if t.Name == name {
			return i
		}
	}
	return -1
}

func NewTag(name, desc string) *openapi3.Tag {
	return &openapi3.Tag{
		Name:        name,
		Description: desc,
	}
}

func NewObjectSchema(m map[string]*contract.NameAndTypeSpec, schemas openapi3.Schemas) *openapi3.Schema {
	schema := openapi3.NewObjectSchema()
	for k, s := range m {
		schema.WithPropertyRef(k, TypeSpecToSchemaRef(s.Type, schemas))
		if !s.Optional {
			schema.Required = append(schema.Required, k)
		}
	}
	return schema
}

func TypeSpecToSchemaRef(s contract.TypeSpec, schemas openapi3.Schemas) *openapi3.SchemaRef {
	name := schemaNameReplacer.Replace(s.Name)
	ref, ok := schemas[name]
	if !ok {
		var schema *openapi3.Schema
		switch s.TypeID {
		case contract.TInteger:
			schema = integerSchema
		case contract.TBoolean:
			schema = booleanSchema
		case contract.TString:
			schema = stringSchema
		case contract.TBytes:
			schema = bytesSchema
		case contract.TAddress:
			schema = addressSchema
		case contract.TStruct:
			schema = NewObjectSchema(s.Resolved.FieldMap, schemas)
		case contract.TUnknown:
			schema = openapi3.NewObjectSchema()
			schema.Description = s.TypeID.String()
		case contract.TVoid:
			return nil
		}
		ref = schema.NewRef()
		schemas[name] = ref
	}
	if s.Dimension > 0 {
		schema := openapi3.NewArraySchema()
		schema.Items = ref
		for i := 1; i < s.Dimension; i++ {
			schema = openapi3.NewArraySchema().WithItems(schema)
		}
		return schema.NewRef()
	}
	return openapi3.NewSchemaRef(schemaRefPrefix+name, ref.Value)
}

func NewPathParameterWithSchema(name string, s *openapi3.Schema) *openapi3.Parameter {
	return openapi3.NewPathParameter(name).WithRequired(true).WithSchema(s)
}

func NewPathParameterWithSchemaRef(name string, sr *openapi3.SchemaRef) *openapi3.Parameter {
	p := openapi3.NewPathParameter(name).WithRequired(true)
	p.Schema = sr
	return p
}

func PutParameter(pm openapi3.ParametersMap, p *openapi3.Parameter) *openapi3.ParameterRef {
	pm[p.Name] = &openapi3.ParameterRef{Value: p}
	return &openapi3.ParameterRef{Ref: parameterRefPrefix + p.Name, Value: p}
}

func NewParameters(ps ...*openapi3.Parameter) openapi3.Parameters {
	parameters := make(openapi3.Parameters, 0)
	for _, p := range ps {
		pr := &openapi3.ParameterRef{
			Value: p,
		}
		parameters = append(parameters, pr)
	}
	return parameters
}

func NewQueryParametersByObjectSchema(s *openapi3.Schema) []*openapi3.Parameter {
	l := make([]*openapi3.Parameter, 0)
	for k, v := range s.Properties {
		p := openapi3.NewQueryParameter(k).WithSchema(v.Value)
		if service.StringSetContains(s.Required, k) {
			p = p.WithRequired(true)
		}
		l = append(l, p)
	}
	return l
}

func NewSuccessResponse() *openapi3.Response {
	return openapi3.NewResponse().WithDescription("Successful operation")
}

func NewSuccessResponseWithSchema(s *openapi3.Schema) *openapi3.Response {
	return NewSuccessResponse().WithJSONSchema(s)
}

func NewSuccessResponseWithSchemaRef(sr *openapi3.SchemaRef) *openapi3.Response {
	return NewSuccessResponse().WithJSONSchemaRef(sr)
}

func ResponsesWithResponse(m openapi3.Responses, status int, resp *openapi3.Response) openapi3.Responses {
	if m == nil {
		m = make(openapi3.Responses)
	}
	m[strconv.FormatInt(int64(status), 10)] = &openapi3.ResponseRef{
		Value: resp,
	}
	return m
}

func NewStringEnumSchema(strs ...string) *openapi3.Schema {
	values := make([]interface{}, len(strs))
	for i := 0; i < len(strs); i++ {
		values[i] = strs[i]
	}
	return openapi3.NewStringSchema().WithEnum(values...)
}

func NewOpenAPISpec(name string) openapi3.T {
	return openapi3.T{
		OpenAPI: openapi3Version,
		Info: &openapi3.Info{
			Title:          infoTitlePrefix + name + infoTitleSuffix,
			Version:        infoDefaultVersion,
			Description:    "",
			TermsOfService: "",
			Contact:        nil,
			License:        infoLicenseApache,
		},
		ExternalDocs: externalDocs,
		Tags:         NewTags(),
		Paths:        make(openapi3.Paths),
		Components: &openapi3.Components{
			Schemas:    NewSchemas(),
			Parameters: make(openapi3.ParametersMap),
		},
	}
}

func NewServiceOpenAPISpec(s service.Service) openapi3.T {
	networkTypeToNetworks := make(map[string][]string)
	for network, networkType := range s.Networks() {
		networks, ok := networkTypeToNetworks[networkType]
		if !ok {
			networks = make([]string, 0)
		}
		networks = append(networks, network)
		networkTypeToNetworks[networkType] = networks
	}
	networksMergeFunc := func(networkTypes []string) []string {
		networks := make([]string, 0)
		for _, networkType := range networkTypes {
			networks = service.StringSetMerge(networks, networkTypeToNetworks[networkType])
		}
		return networks
	}
	oas := NewOpenAPISpec(s.Name())
	oas.Tags = append(oas.Tags, NewTag(s.Name(), fmt.Sprintf("%s Service", s.Name())))
	for networkType, networks := range networkTypeToNetworks {
		desc := fmt.Sprintf("NetworkType:%s, Networks: {%s}", networkType, strings.Join(networks, ","))
		oas.Tags = append(oas.Tags, NewTag(networkType, desc))
	}

	ss := s.Spec()
	for _, sm := range ss.Methods {
		if !methodNameRegexp.MatchString(sm.Name) {
			continue
		}
		inputs := sm.Inputs
		output := sm.Output
		readonly := sm.Readonly
		pi := NewPathItemForMethodSpec(s.Name(), sm.NetworkTypes, inputs, output, readonly, oas.Components.Schemas)
		networkParam := PathParamNetwork
		networks := networksMergeFunc(sm.NetworkTypes)
		pi.Parameters = NewParameters(NewPathParameterWithSchema(networkParam, NewStringEnumSchema(networks...)))
		path := fmt.Sprintf("%s/{%s}/%s/%s", GroupUrlApi, networkParam, ss.Name, sm.Name)
		oas.Paths[path] = pi
		for i, o := range sm.Overloads {
			if o.Inputs != nil {
				inputs = *o.Inputs
			} else {
				inputs = sm.Inputs
			}
			if o.Output != nil {
				output = *o.Output
			} else {
				output = sm.Output
			}
			if o.Readonly != nil {
				readonly = *o.Readonly
			} else {
				readonly = sm.Readonly
			}
			pi = NewPathItemForMethodSpec(s.Name(), o.NetworkTypes, inputs, output, readonly, oas.Components.Schemas)
			networkParam = fmt.Sprintf("network_%d", i)
			networks = networksMergeFunc(o.NetworkTypes)
			pi.Parameters = NewParameters(NewPathParameterWithSchema(networkParam, NewStringEnumSchema(networks...)))
			path = fmt.Sprintf("%s/{%s}/%s/%s", GroupUrlApi, networkParam, ss.Name, sm.Name)
			oas.Paths[path] = pi
		}
	}
	return oas
}

func NewPathItemForMethodSpec(
	serviceName string,
	networkTypes []string,
	inputs map[string]*contract.NameAndTypeSpec,
	output contract.TypeSpec,
	readonly bool,
	schemas openapi3.Schemas) *openapi3.PathItem {
	pi := &openapi3.PathItem{}
	req := openapi3.NewObjectSchema().
		WithPropertyRef("options", DefaultSchemaRef(schemaOptions)).
		WithProperty("params", NewObjectSchema(inputs, schemas))
	if readonly {
		pi.Get = &openapi3.Operation{
			Tags:        append(networkTypes, tagReadonly, serviceName),
			Summary:     "",
			Description: "",
			OperationID: "",
			Parameters:  NewParameters(openapi3.NewQueryParameter("request").WithSchema(req)),
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchemaRef(TypeSpecToSchemaRef(output, schemas))),
			Callbacks:    nil,
			Deprecated:   false,
			Security:     nil,
			Servers:      nil,
			ExternalDocs: nil,
		}
	} else {
		pi.Post = &openapi3.Operation{
			Tags:        append(networkTypes, tagWritable, serviceName),
			Summary:     "",
			Description: "",
			OperationID: "",
			Parameters:  nil,
			RequestBody: &openapi3.RequestBodyRef{
				Value: openapi3.NewRequestBody().WithContent(
					openapi3.NewContentWithJSONSchema(req)),
			},
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchemaRef(DefaultSchemaRef(schemaTxID))),
			Callbacks:    nil,
			Deprecated:   false,
			Security:     nil,
			Servers:      nil,
			ExternalDocs: nil,
		}
	}
	return pi
}

type OpenAPISpecProvider struct {
	n2nt  map[string]string
	nt2ns map[string][]string
	s2d   map[string]openapi3.T
	d     openapi3.T
	npr   *openapi3.ParameterRef //Network ParameterRef
	gpi   openapi3.Paths
	mtx   sync.RWMutex
	l     log.Logger
}

func NewOpenAPISpecProvider(l log.Logger) *OpenAPISpecProvider {
	oas := NewOpenAPISpec("")
	oas.Tags = append(openapi3.Tags{NewTag(tagGeneral, "General purpose")}, oas.Tags...)

	oas.Paths[GroupUrlApi] = &openapi3.PathItem{
		Get: &openapi3.Operation{
			Tags:        []string{tagGeneral},
			Summary:     "Retrieve networks",
			Description: "",
			OperationID: "",
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(MustGenerateSchema(NetworkInfos{}))),
		},
	}

	gpi := make(openapi3.Paths)
	npr := PutParameter(oas.Components.Parameters, NewPathParameterWithSchema(PathParamNetwork, NewStringEnumSchema()))
	nau, napi := newNetworkAPIPathItem(npr)
	gpi[nau] = napi

	tpr := PutParameter(oas.Components.Parameters, NewPathParameterWithSchemaRef(PathParamTxID, DefaultSchemaRef(schemaTxID)))
	gru, grpi := newGetResultPathItem(npr, tpr)
	gpi[gru] = grpi

	bpr := PutParameter(oas.Components.Parameters, NewPathParameterWithSchemaRef(PathParamBlockID, DefaultSchemaRef(schemaBlockID)))
	hpr := PutParameter(oas.Components.Parameters, openapi3.NewQueryParameter(QueryParamHeight).WithSchema(openapi3.NewInt64Schema()))
	gfu, gfpi := newGetFinalityPathItem(npr, bpr, hpr)
	gpi[gfu] = gfpi

	as := openapi3.NewOneOfSchema(openapi3.NewStringSchema())
	as.OneOf = append(as.OneOf, DefaultSchemaRef(contract.TAddress.String()))
	apr := PutParameter(oas.Components.Parameters, NewPathParameterWithSchema(PathParamServiceOrAddress, as))
	sau, sapi := newServiceAPIPathItem(npr, apr)
	gpi[sau] = sapi

	mpr := PutParameter(oas.Components.Parameters, NewPathParameterWithSchema(PathParamMethod, openapi3.NewStringSchema()))
	mu, mpi := newMethodAPIPathItem(npr, apr, mpr)
	gpi[mu] = mpi

	for k, v := range gpi {
		oas.Paths[k] = v
	}

	oas.Tags = append(openapi3.Tags{NewTag(tagAutoCaller, "Auto Caller status")}, oas.Tags...)
	oas.Paths[GroupUrlAutoCaller] = &openapi3.PathItem{
		Get: &openapi3.Operation{
			Tags:        []string{tagAutoCaller},
			Summary:     "Retrieve Auto Callers",
			Description: "",
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(MustGenerateSchema(AutoCallerInfos{}))),
		},
	}
	acu := fmt.Sprintf("%s/{%s}", GroupUrlAutoCaller, PathParamService)
	oas.Paths[acu] = &openapi3.PathItem{
		Get: &openapi3.Operation{
			Tags:        []string{tagAutoCaller},
			Summary:     "Retrieve Auto Caller status",
			Description: "",
			OperationID: "",
			Parameters: NewParameters(append([]*openapi3.Parameter{NewPathParameterWithSchema(PathParamService, openapi3.NewStringSchema())},
				NewQueryParametersByObjectSchema(MustGenerateSchema(autocaller.FindParam{}))...)...),
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(openapi3.NewArraySchema().WithItems(openapi3.NewObjectSchema()))),
		},
	}
	return &OpenAPISpecProvider{
		n2nt:  make(map[string]string),
		nt2ns: make(map[string][]string),
		s2d:   make(map[string]openapi3.T),
		d:     oas,
		npr:   npr,
		gpi:   gpi,
		l:     l,
	}
}

func newNetworkAPIPathItem(npr *openapi3.ParameterRef) (string, *openapi3.PathItem) {
	pi := &openapi3.PathItem{
		Parameters: openapi3.Parameters{npr},
		Get: &openapi3.Operation{
			Tags:        []string{tagGeneral},
			Summary:     "Retrieve services",
			Description: "",
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(MustGenerateSchema(ServiceInfos{}))),
		},
		Post: &openapi3.Operation{
			Tags:        []string{tagGeneral},
			Summary:     "Register contract service",
			Description: "",
			RequestBody: &openapi3.RequestBodyRef{
				Value: openapi3.NewRequestBody().WithContent(
					openapi3.NewContentWithJSONSchema(MustGenerateSchema(RegisterContractServiceRequest{}))),
			},
			Responses: ResponsesWithResponse(nil, http.StatusOK, NewSuccessResponse()),
		},
	}
	return fmt.Sprintf("%s/{%s}", GroupUrlApi, npr.Value.Name), pi
}

func newGetResultPathItem(npr, tpr *openapi3.ParameterRef) (string, *openapi3.PathItem) {
	pi := &openapi3.PathItem{
		Parameters: openapi3.Parameters{npr, tpr},
		Get: &openapi3.Operation{
			Tags:        []string{tagGeneral},
			Summary:     "Get result of transaction with given TxID",
			Description: "",
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(openapi3.NewObjectSchema())),
		},
	}
	return fmt.Sprintf("%s/{%s}%s/{%s}", GroupUrlApi, npr.Value.Name, UrlGetResult, tpr.Value.Name), pi
}

func newGetFinalityPathItem(npr, bpr, hpr *openapi3.ParameterRef) (string, *openapi3.PathItem) {
	pi := &openapi3.PathItem{
		Parameters: openapi3.Parameters{npr, bpr, hpr},
		Get: &openapi3.Operation{
			Tags:        []string{tagGeneral},
			Summary:     "Get finality of block with given BlockID and height",
			Description: "",
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(openapi3.NewBoolSchema())),
		},
	}
	return fmt.Sprintf("%s/{%s}%s/{%s}", GroupUrlApi, npr.Value.Name, UrlGetFinality, bpr.Value.Name), pi
}

func newServiceAPIPathItem(npr, apr *openapi3.ParameterRef) (string, *openapi3.PathItem) {
	pi := &openapi3.PathItem{
		Parameters: openapi3.Parameters{npr, apr},
		Get: &openapi3.Operation{
			Tags:        []string{tagGeneral},
			Summary:     "Retrieve methods",
			Description: "",
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchemaRef(DefaultSchemaRef(schemaMethodInfos))),
		},
	}
	return fmt.Sprintf("%s/{%s}/{%s}", GroupUrlApi, npr.Value.Name, apr.Value.Name), pi
}

func newMethodAPIPathItem(npr, apr, mpr *openapi3.ParameterRef) (string, *openapi3.PathItem) {
	pi := &openapi3.PathItem{
		Parameters: openapi3.Parameters{npr, apr, mpr},
		Get: &openapi3.Operation{
			Tags:        []string{tagGeneral, tagReadonly},
			Summary:     "Call readonly method",
			Description: "",
			Parameters: NewParameters(openapi3.NewQueryParameter("request").
				WithSchema(DefaultSchemaRef(schemaRequest).Value)),
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchema(openapi3.NewObjectSchema())),
		},
		Post: &openapi3.Operation{
			Tags:        []string{tagGeneral, tagWritable},
			Summary:     "Call writable method",
			Description: "",
			RequestBody: &openapi3.RequestBodyRef{
				Value: openapi3.NewRequestBody().WithContent(
					openapi3.NewContentWithJSONSchema(DefaultSchemaRef(schemaRequest).Value)),
			},
			Responses: ResponsesWithResponse(nil, http.StatusOK,
				NewSuccessResponseWithSchemaRef(DefaultSchemaRef(schemaTxID))),
		},
	}
	return fmt.Sprintf("%s/{%s}/{%s}/{%s}", GroupUrlApi, npr.Value.Name, apr.Value.Name, mpr.Value.Name), pi
}

func (o *OpenAPISpecProvider) Get(name string) openapi3.T {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

	if len(name) == 0 {
		return o.d
	}
	return o.s2d[name]
}

func (o *OpenAPISpecProvider) PutNetworkToNetworkType(network, networkType string) {
	o.mtx.Lock()
	defer o.mtx.Unlock()

	if ont, ok := o.n2nt[network]; ok {
		if ont == networkType {
			return
		}
		o.l.Debugf("update networkType network:%s old:%s new:%s", network, ont, networkType)
		ons, _ := service.StringSetRemove(o.nt2ns[ont], network)
		o.nt2ns[ont] = ons
		if len(ons) == 0 {
			delete(o.nt2ns, ont)
			i := TagsIndex(o.d.Tags, ont)
			o.d.Tags = append(o.d.Tags[:i], o.d.Tags[i+1:]...)
			for _, pi := range o.gpi {
				for _, op := range pi.Operations() {
					op.Tags, _ = service.StringSetRemove(op.Tags, ont)
				}
			}
		} else {
			o.d.Tags.Get(ont).Description = strings.Join(ons, ",")
		}
	} else {
		nps := o.npr.Value.Schema.Value
		nps.Enum = append(nps.Enum, network)
	}
	o.n2nt[network] = networkType
	ns, ok := service.StringSetAdd(o.nt2ns[networkType], network)
	o.nt2ns[networkType] = ns
	if ok {
		desc := strings.Join(ns, ",")
		if t := o.d.Tags.Get(networkType); t == nil {
			o.d.Tags = append(o.d.Tags, NewTag(networkType, desc))
		} else {
			t.Description = desc
		}
		for _, pi := range o.gpi {
			for _, op := range pi.Operations() {
				op.Tags, _ = service.StringSetAdd(op.Tags, networkType)
			}
		}
	}
	o.l.Debugf("SetAdaptor network:%s", network)
}

func (o *OpenAPISpecProvider) Merge(svc service.Service) {
	name := svc.Name()
	if old, ok := o.s2d[name]; ok {
		o.l.Debugf("replace OpenAPISpec service:%s", name)
		for k := range old.Paths {
			delete(o.d.Paths, k)
		}
	}
	ss := NewServiceOpenAPISpec(svc)
	o.s2d[name] = ss
	for _, v := range ss.Tags {
		i := TagsIndex(o.d.Tags, v.Name)
		if i >= 0 {
			o.l.Warnf("overwrite OpenAPI tag:%s service:%s", v.Name, name)
			o.d.Tags[i] = v
		} else {
			o.d.Tags = append(o.d.Tags, v)
		}
	}
	for k, v := range ss.Paths {
		if _, ok := o.d.Paths[k]; ok {
			o.l.Warnf("overwrite OpenAPI path:%s service:%s", k, name)
		}
		o.d.Paths[k] = v
	}
	for k, v := range ss.Components.Schemas {
		if _, ok := o.d.Components.Schemas[k]; ok {
			o.l.Warnf("overwrite OpenAPI schema:%s service:%s", k, name)
		}
		o.d.Components.Schemas[k] = v
	}

	for k, v := range ss.Components.Parameters {
		if _, ok := o.d.Components.Parameters[k]; ok {
			o.l.Warnf("overwrite OpenAPI parameter:%s service:%s", k, name)
		}
		o.d.Components.Parameters[k] = v
	}
}
