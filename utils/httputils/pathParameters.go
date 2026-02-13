package httputils

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/onlysumitg/GoMockAPI/internal/validator"
	"github.com/onlysumitg/GoMockAPI/utils/typeutils"
)

type PathParam struct {
	Name        string
	Value       any
	StringValue string
	DataType    string
	IsVariable  bool
}

func (p *PathParam) String() string {
	return fmt.Sprintf("Path Param: %s %s %s %t", p.Name, p.Value, p.DataType, p.IsVariable)
}

func GetPathParamMap(urlString string, removePrefix string) ([]*PathParam, error) {
	u, err := url.Parse(urlString)

	pathParams := make([]*PathParam, 0)

	if err != nil {
		return pathParams, err
	}

	path := u.Path

	if removePrefix != "" {
		path = strings.TrimPrefix(path, removePrefix)

	}

	path = strings.Trim(path, "/")

	parms := strings.Split(path, "/")
	for i, p := range parms {
		p, err := processPathParam(p)

		if err == nil {
			p.Name = fmt.Sprintf("*PATH_%d", i)
			pathParams = append(pathParams, p)
		} else {

			return nil, err
		}

	}

	return pathParams, err
}

func processPathParam(p string) (*PathParam, error) {
	v := p
	d := "string"

	isVariable := false
	if strings.HasPrefix(p, "{") && !strings.HasSuffix(p, "}") {
		return nil, fmt.Errorf("invalid format for '%s'. missing '{' or '}'", p)
	}
	if !strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
		return nil, fmt.Errorf("invalid format for '%s'. missing '{' or '}'", p)
	}

	if strings.HasPrefix(p, "{") {
		isVariable = true
		value := strings.TrimPrefix(p, "{")
		value = strings.TrimSuffix(value, "}")
		splitDT := strings.Split(value, ":")

		switch len(splitDT) {
		case 1:
			v = splitDT[0]
			d = "string"
		case 2:
			v = splitDT[0]
			d = splitDT[1]
		default:

			return nil, fmt.Errorf("invalid format for '%s'.too many ':'", p)
		}

	}

	d = strings.ToUpper(d)

	err := validateParam(v, d)

	if err != nil {
		return nil, err
	}

	return &PathParam{
		Value:       typeutils.ConvertToType(v, d),
		StringValue: v,
		DataType:    d,
		IsVariable:  isVariable,
	}, nil

}

func validateParam(v string, d string) error {
	allowedDataType := []string{"FLOAT64", "INT", "BOOL", "STRING"}

	dataTypeOK := false

	for _, x := range allowedDataType {
		if x == d {
			dataTypeOK = true
		}
	}

	if !dataTypeOK {
		return fmt.Errorf("unknown datatype '%s'", d)
	}

	valueOk := validator.MustBeOfType(v, d)

	if !valueOk {
		return fmt.Errorf("invalid value %s for datatype '%s'", v, d)
	}
	return nil
}
