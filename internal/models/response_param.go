package models

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/onlysumitg/GoMockAPI/internal/validator"
	"github.com/onlysumitg/GoMockAPI/utils/httputils"
	"github.com/onlysumitg/GoMockAPI/utils/typeutils"
	"github.com/onlysumitg/GoMockAPI/utils/xmlutils"
	bolt "go.etcd.io/bbolt"
)

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// Define a new User type. Notice how the field names and types align
// with the columns in the database "users" table?
type EndPointResponseParam struct {
	ID      string `json:"id" db:"id" form:"id"`
	OwnerId string `json:"ownerid" db:"ownerid" form:"ownerid"`

	Key           string `json:"key" db:"key" form:"key"`
	DefaultValue  any    `json:"defaultvalue" db:"defaultvalue" form:"defaultvalue"`
	OverrideValue string `json:"overridevalue" db:"overridevalue" form:"overridevalue"`
	ValueToUse    any    `json:"-" db:"-" form:"-"`

	DefaultDatatype   string `json:"defaultdatatype" db:"defaultdatatype" form:"defaultdatatype"`
	OverrrideDatatype string `json:"overridedatatype" db:"overridedatatype" form:"overridedatatype"`

	validator.Validator
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func (p *EndPointResponseParam) getValueToUse(requestMap map[string]xmlutils.ValueDatatype, overrideValue string) (any, error) {

	if overrideValue == "" {
		return p.DefaultValue, nil

	}

	brokenValues := strings.Split(overrideValue, ":")

	valueSource := strings.TrimSpace(brokenValues[0])
	if strings.HasPrefix(valueSource, "REQUEST[") && len(brokenValues) > 1 {
		valueKey := strings.TrimSpace(strings.Join(brokenValues[1:], ":"))
		requestValue, found := requestMap[valueKey]
		if found {
			return requestValue.Value, nil
		} else {
			return overrideValue, fmt.Errorf("Request Parameter not found:%s", overrideValue)
		}

	} else if strings.HasPrefix(valueSource, "*RANDOM") && len(brokenValues) > 1 {
		randomValue, err := GenerateRandom(brokenValues[1])
		if err == nil {
			return randomValue, nil
		} else {
			return overrideValue, err
		}
	}

	return overrideValue, nil
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func (p *EndPointResponseParam) processSpecials(apiCall *ApiCall, forResponse string, value string) {

	apiTrackKey := fmt.Sprintf("%s_%s", forResponse, p.Key)

	if apiCall.HasSet(apiTrackKey) {
		apiCall.LogInfo(fmt.Sprintf("Special param %s has already assgined. Skipping new assignment", p.Key))

		return
	}

	specialTypeBroken := strings.Split(p.Key, "_")

	specialType := specialTypeBroken[0]

	var specialKey string = ""

	if len(specialTypeBroken) > 1 {
		specialKey = strings.Join(specialTypeBroken[1:], "_")
	}

	switch specialType {

	case "*DELAY":
		if specialKey == "RESPONSE_MILLI_SEC" {

			// c := typeutils.GetIntVal(value)
			// if c > 0 {
			//apiCall.LogInfo(fmt.Sprintf("Will add %d millisecond delay", c))

			apiCall.AdditionalResponseValues[apiTrackKey] = value
			apiCall.SetKey(apiTrackKey)

			// } else {
			// 	apiCall.LogInfo(fmt.Sprintf("Response delay skipped due to invalid value %d", c))

			// }
		}

	case "*HTTP":
		apiCall.LogInfo(fmt.Sprintf("*HTTP Special param %s.", p.Key))

		if specialKey == "STATUS_CODE" {

			c := typeutils.GetIntVal(value)

			_, found := httputils.Codes[c]

			if found {
				apiCall.LogInfo(fmt.Sprintf("HTTP STATUS_CODE set to %d", c))

				apiCall.StatusCode = c
				apiCall.SetKey(p.Key)

			} else {
				apiCall.LogInfo(fmt.Sprintf("HTTP STATUS_CODE skipped due to invalid code %d", c))

			}
		}

	case "*HEADER":
		apiCall.LogInfo(fmt.Sprintf("Setting HEADER %s %s", specialKey, value))

		//apiCall.ResponseHeader[specialKey] = value
		for _, r := range apiCall.ResponseMapXX {

			if r.ID == forResponse {
				r.ResponseHeader[specialKey] = value

			}
		}

		apiCall.SetKey(apiTrackKey)

	}

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func (p *EndPointResponseParam) process(apiCall *ApiCall, forResponse string, value string) {
	searchString := fmt.Sprintf("\"{{%s}}\"", p.Key)
	replaceString := ""

	validStringValue := ""

	var valueToUse any
	if value != "" {
		valueToUseX, err := p.getValueToUse(apiCall.RequestFlatMap, value)
		if err != nil {
			apiCall.LogError(err.Error())
		}
		valueToUse = valueToUseX
	} else {
		valueToUseX, err := p.getValueToUse(apiCall.RequestFlatMap, p.OverrideValue)
		if err != nil {
			apiCall.LogError(err.Error())
		}
		valueToUse = valueToUseX
	}

	apiCall.LogInfo(fmt.Sprintf("Param %s assignement. Raw Value %s", p.Key, valueToUse))

	switch strings.ToUpper(p.DefaultDatatype) {
	case "BOOL": // without quotes
		replaceString = fmt.Sprintf("%t", typeutils.GetBoolVal(valueToUse))
		validStringValue = replaceString

	case "FLOAT64":
		replaceString = fmt.Sprintf("%f", typeutils.GetFloatVal(valueToUse))
		validStringValue = replaceString

	case "INT":
		replaceString = fmt.Sprintf("%d", typeutils.GetIntVal(valueToUse))
		validStringValue = replaceString

	case "INVALID":
		replaceString = "null"
		validStringValue = replaceString
	case "XMLSTRING": // string without quotes
		replaceString = fmt.Sprintf("%s", valueToUse)
		validStringValue = fmt.Sprintf("%s", valueToUse)
	default:
		replaceString = strconv.Quote(fmt.Sprintf("%s", valueToUse)) // escape double quotes
		validStringValue = fmt.Sprintf("%s", valueToUse)

	}

	apiCall.LogInfo(fmt.Sprintf("Param %s assignement. Final Value %s", p.Key, replaceString))
	if strings.HasPrefix(p.Key, "*") {
		apiCall.LogInfo(fmt.Sprintf("Param %s assignement. Special variable.", p.Key))
		p.processSpecials(apiCall, forResponse, validStringValue)
	}

	// if status code has been set --> only update that status code
	// else => update all status code
	for _, r := range apiCall.ResponseMapXX {

		if r.ID == forResponse {
			r.Response = strings.ReplaceAll(r.Response, searchString, replaceString)

		}

	}
	//apiCall.ResponseString = strings.ReplaceAll(apiCall.ResponseString, searchString, replaceString)
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// Define a new UserModel type which wraps a database connection pool.
type EndPointResponseParamModel struct {
	DB *bolt.DB
}

func (m *EndPointResponseParamModel) getTableName() []byte {
	return []byte("responseparam")
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointResponseParamModel) Save(u *EndPointResponseParam) (string, error) {
	if u.ID == "" {
		var id string = u.Key //uuid.NewString()
		u.ID = fmt.Sprintf("%s_%s", u.OwnerId, id)
	}
	err := m.Update(u, false)

	return u.ID, err
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointResponseParamModel) Update(u *EndPointResponseParam, clearCache bool) error {
	err := m.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
		if err != nil {
			return err
		}
		buf, err := json.Marshal(u)
		if err != nil {
			return err
		}

		// key = > user.name+ user.id
		key := strings.ToUpper(u.ID) // + string(itob(u.ID))

		return bucket.Put([]byte(key), buf)
	})

	return err

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointResponseParamModel) Delete(id string) error {

	err := m.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
		if err != nil {
			return err
		}
		key := strings.ToUpper(id)
		dbDeleteError := bucket.Delete([]byte(key))
		return dbDeleteError
	})

	return err
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointResponseParamModel) Get(id string) (*EndPointResponseParam, error) {

	if id == "" {
		return nil, errors.New("param blank id not allowed")
	}
	var paramJSON []byte // = make([]byte, 0)

	err := m.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(m.getTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}
		paramJSON = bucket.Get([]byte(strings.ToUpper(id)))

		return nil

	})
	param := EndPointResponseParam{}
	if err != nil {
		return &param, err
	}

	if paramJSON != nil {
		err := json.Unmarshal(paramJSON, &param)
		return &param, err

	}

	return &param, ErrServerNotFound

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointResponseParamModel) ListByOwnerId(ownerid string) []*EndPointResponseParam {
	params := make([]*EndPointResponseParam, 0)
	_ = m.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(m.getTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}
		c := bucket.Cursor()
		prefix := []byte(strings.ToUpper(ownerid) + "_")

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if bytes.HasPrefix(k, prefix) {

				param := EndPointResponseParam{}
				err := json.Unmarshal(v, &param)
				if err == nil {
					params = append(params, &param)
				}
			}
		}

		return nil
	})
	return params

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func (m *EndPointResponseParamModel) ClearEndPointData(endpointID string) {
	// TODO >>>
	// for _, r := range endpoint.ResponseMap {
	// 	r.ResponseParams = responseParamModel.ListByOwnerId(r.ID)
	// }

	// rlist := m.ListById(endpointID)
	// for _, r := range rlist {
	// 	m.Delete(r.ID)
	// }
}
