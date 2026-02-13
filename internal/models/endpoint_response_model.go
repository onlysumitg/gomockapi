package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/onlysumitg/GoMockAPI/internal/validator"
	"github.com/onlysumitg/GoMockAPI/utils/jsonutils"
	"github.com/onlysumitg/GoMockAPI/utils/xmlutils"
	bolt "go.etcd.io/bbolt"
)

type EndPointResponse struct {
	ID string `json:"id" db:"id" form:"id"`
	// EndpointID string `json:"endpointid" db:"endpointid" form:"endpointid"`

	Name string `json:"name" db:"name" form:"name"`
	

	HttpCode int `json:"httpcode" db:"httpcode" form:"httpcode"`

	ResponseHeader            string `json:"header" db:"header" form:"header"`
	ResponseHeaderPlaceholder string `json:"headerplaceholder" db:"headerplaceholder" form:"-"`
	ResponseHeaderType        string `json:"headertype" db:"headertype" form:"headertype"`

	Response            string `json:"response" db:"response" form:"response"`
	ResponsePlaceholder string `json:"responseplaceholder" db:"responseplaceholder" form:"-"`
	ResponseType        string `json:"responsetype" db:"responsetype" form:"responsetype"`

	ResponseParams []*EndPointResponseParam `json:"-" db:"-" from:"-"`

	validator.Validator `json:"-" db:"-" from:"-"`
}

// ------------------------------------------------------------
// Build Response Placeholder
// ------------------------------------------------------------
func (s *EndPointResponse) BuildResponsePlaceholder() {
	switch s.ResponseType {
	case "JSON":
		uResponsePlaceholder, err := jsonutils.JsonToMapPlaceholder(s.Response)
		if err != nil {
		} else {
			asBytes, err := json.Marshal(uResponsePlaceholder)
			if err == nil {
				s.ResponsePlaceholder = string(asBytes)
			}

		}
	case "XML":
		_, uResponsePlaceholder, err := xmlutils.XmlToFlatMapAndPlaceholder(s.Response)
		if err != nil {
		} else {
			s.ResponsePlaceholder = uResponsePlaceholder
		}
	case "TEXT":
		s.ResponsePlaceholder = s.Response
	}

}

// ------------------------------------------------------------
// Build Response Placeholder
// ------------------------------------------------------------
func (s *EndPointResponse) BuildResponseHeaderPlaceholder() {
	switch s.ResponseHeaderType {
	case "JSON":
		uResponseHeaderPlaceholder, err := jsonutils.JsonToMapPlaceholder(s.ResponseHeader)
		if err != nil {
		} else {
			asBytes, err := json.Marshal(uResponseHeaderPlaceholder)
			if err == nil {
				s.ResponseHeaderPlaceholder = string(asBytes)
			}

		}
	case "XML":
		_, uResponseHeaderPlaceholder, err := xmlutils.XmlToFlatMapAndPlaceholder(s.ResponseHeader)
		if err != nil {
		} else {
			s.ResponseHeaderPlaceholder = uResponseHeaderPlaceholder
		}
	}

}

// ------------------------------------------------------------
//
// ------------------------------------------------------------
func (s *EndPointResponse) RebuildResponseParams(wg *sync.WaitGroup, DB *bolt.DB) error {
	//flatmap, err := jsonutils.JsonToFlatMap(endPoint.SampleResponse)

	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	var err error
	var flatmap map[string]xmlutils.ValueDatatype
	//xmlPlaceholder := ""

	switch s.ResponseType {
	case "JSON":
		flatmap, err = jsonutils.JsonToFlatMap(s.Response)
	case "XML":
		flatmap, _, err = xmlutils.XmlToFlatMapAndPlaceholder(s.Response)
	case "TEXT":
		flatmap = make(map[string]xmlutils.ValueDatatype)
	default:
		err = errors.New("Unknow Request Type")

	}

	if err != nil {

	} else {
		endPointResponseParamModel := &EndPointResponseParamModel{DB: DB}

		paramMap := make(map[string]*EndPointResponseParam)

		// endPointRequestParam := &EndPointResponseParam{
		// 	OwnerId:         s.ID,
		// 	Key:             "*HTTP_STATUS_CODE",
		// 	DefaultValue:    strconv.Itoa(s.HttpCode),
		// 	DefaultDatatype: "INT",
		// }
		// paramMap["*HTTP_STATUS_CODE"] = endPointRequestParam

		endPointRequestParam := &EndPointResponseParam{
			OwnerId:         s.ID,
			Key:             "*DELAY_RESPONSE_MILLI_SEC",
			DefaultValue:    "0",
			DefaultDatatype: "STRING",
		}
		paramMap["*DELAY_RESPONSE_MILLI_SEC"] = endPointRequestParam

		for key, jsonVal := range flatmap {
			endPointResponseParam := &EndPointResponseParam{
				OwnerId:         s.ID,
				Key:             key,
				DefaultValue:    jsonVal.Value,
				DefaultDatatype: jsonVal.DataType,
			}
			paramMap[key] = endPointResponseParam

		}

		savedParameters := endPointResponseParamModel.ListByOwnerId(s.ID)

		for _, savedParam := range savedParameters {
			if strings.HasPrefix(savedParam.Key, "*HEADER_") || strings.HasPrefix(savedParam.Key, "*PATH_") {
				continue
			}
			orgParam, found := paramMap[savedParam.Key]
			if !found {

				endPointResponseParamModel.Delete(savedParam.ID)

				continue
			}

			orgParam.OverrideValue = savedParam.OverrideValue
			orgParam.OverrrideDatatype = savedParam.OverrrideDatatype
			orgParam.ID = savedParam.ID
		}

		for _, v := range paramMap {
			endPointResponseParamModel.Save(v)
		}
	}

	return err
}

func (s *EndPointResponse) RebuildResponseHeaderParams(wg *sync.WaitGroup, DB *bolt.DB) error {
	//flatmap, err := jsonutils.JsonToFlatMap(endPoint.SampleResponseHeader)

	defer wg.Done()

	var err error
	var flatmap map[string]xmlutils.ValueDatatype
	//xmlPlaceholder := ""

	switch s.ResponseHeaderType {
	case "JSON":
		flatmap, err = jsonutils.JsonToFlatMap(s.ResponseHeader)
	case "XML":
		flatmap, _, err = xmlutils.XmlToFlatMapAndPlaceholder(s.ResponseHeader)

	default:
		err = errors.New("Unknow Request Type")

	}

	if err != nil {

	} else {
		endPointResponseParamModel := &EndPointResponseParamModel{DB: DB}

		paramMap := make(map[string]*EndPointResponseParam)

		for key, jsonVal := range flatmap {
			keyToUse := fmt.Sprintf("*HEADER_%s", key)

			endPointResponseParam := &EndPointResponseParam{
				OwnerId:         s.ID,
				Key:             keyToUse,
				DefaultValue:    jsonVal.Value,
				DefaultDatatype: jsonVal.DataType,
			}
			paramMap[keyToUse] = endPointResponseParam

		}

		savedParameters := endPointResponseParamModel.ListByOwnerId(s.ID)

		for _, savedParam := range savedParameters {
			if strings.HasPrefix(savedParam.Key, "*HEADER_") {
				orgParam, found := paramMap[savedParam.Key]
				if !found {
					endPointResponseParamModel.Delete(savedParam.ID)

					continue
				}

				orgParam.OverrideValue = savedParam.OverrideValue
				orgParam.OverrrideDatatype = savedParam.OverrrideDatatype
				orgParam.ID = savedParam.ID
			}
		}

		for _, v := range paramMap {
			endPointResponseParamModel.Save(v)
		}
	}

	return err
}

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // Define a new UserModel type which wraps a database connection pool.
// type EndPointResponsesModel struct {
// 	DB *bolt.DB
// }

// func (m *EndPointResponsesModel) getTableName() []byte {
// 	return []byte("endpointresponsesmodel")
// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Insert method to add a new record to the "users" table.
// func (m *EndPointResponsesModel) Save(u *EndPointResponse) (string, error) {
// 	if u.ID == "" {
// 		var id string = uuid.NewString()
// 		u.ID = id

// 	}

// 	err := m.Update(u, false)

// 	return u.ID, err
// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Insert method to add a new record to the "users" table.
// func (m *EndPointResponsesModel) Update(u *EndPointResponse, clearCache bool) error {
// 	err := m.DB.Update(func(tx *bolt.Tx) error {
// 		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
// 		if err != nil {
// 			return err
// 		}

// 		buf, err := json.Marshal(u)
// 		if err != nil {
// 			return err
// 		}

// 		// key = > user.name+ user.id
// 		key := strings.ToUpper(u.ID) // + string(itob(u.ID))

// 		return bucket.Put([]byte(key), buf)
// 	})

// 	return err

// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Insert method to add a new record to the "users" table.
// func (m *EndPointResponsesModel) Delete(id string) error {

// 	err := m.DB.Update(func(tx *bolt.Tx) error {
// 		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
// 		if err != nil {
// 			return err
// 		}
// 		key := strings.ToUpper(id)
// 		dbDeleteError := bucket.Delete([]byte(key))
// 		return dbDeleteError
// 	})

// 	return err
// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Exists method to check if a user exists with a specific ID.
// func (m *EndPointResponsesModel) Exists(id string) bool {

// 	var userJson []byte

// 	_ = m.DB.View(func(tx *bolt.Tx) error {
// 		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
// 		if err != nil {
// 			return err
// 		}
// 		key := strings.ToUpper(id)

// 		userJson = bucket.Get([]byte(key))

// 		return nil

// 	})

// 	return (userJson != nil)
// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Exists method to check if a user exists with a specific ID.
// func (m *EndPointResponsesModel) DuplicateName(s *EndPointResponse) bool {
// 	exists := false
// 	codeToCheck := s.HttpCode

// 	for _, server := range m.List() {

// 		currentCode := server.HttpCode

// 		if codeToCheck == currentCode && !strings.EqualFold(server.ID, s.ID) {
// 			exists = true
// 			break
// 		}
// 	}

// 	return exists
// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Exists method to check if a user exists with a specific ID.
// func (m *EndPointResponsesModel) Get(id string) (*EndPointResponse, error) {

// 	if id == "" {
// 		return nil, errors.New("Server blank id not allowed")
// 	}
// 	var serverJSON []byte // = make([]byte, 0)

// 	err := m.DB.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket(m.getTableName())
// 		if bucket == nil {
// 			return errors.New("table does not exits")
// 		}
// 		serverJSON = bucket.Get([]byte(strings.ToUpper(id)))

// 		return nil

// 	})
// 	endpoint := EndPointResponse{}
// 	if err != nil {
// 		return &endpoint, err
// 	}

// 	if serverJSON != nil {
// 		err := json.Unmarshal(serverJSON, &endpoint)

// 		if err == nil {
// 			//requestParamModel := &EndPointRequestParamModel{DB: m.DB}
// 			//endpoint.RequestParams = requestParamModel.ListById(endpoint.ID)
// 			responseParamModel := &EndPointResponseParamModel{DB: m.DB}
// 			endpoint.ResponseParams = responseParamModel.ListById(endpoint.ID)
// 			//m.LoadConditionGroups(&endpoint)

// 		}
// 		return &endpoint, err

// 	}

// 	return &endpoint, ErrServerNotFound

// }

// // -----------------------------------------------------------------
// //
// // -----------------------------------------------------------------
// // We'll use the Exists method to check if a user exists with a specific ID.
// func (m *EndPointResponsesModel) List() []*EndPointResponse {
// 	endpoints := make([]*EndPointResponse, 0)
// 	_ = m.DB.View(func(tx *bolt.Tx) error {
// 		bucket := tx.Bucket(m.getTableName())
// 		if bucket == nil {
// 			return errors.New("table does not exits")
// 		}
// 		c := bucket.Cursor()

// 		for k, v := c.First(); k != nil; k, v = c.Next() {

// 			endpoint := EndPointResponse{}
// 			err := json.Unmarshal(v, &endpoint)
// 			if err == nil {
// 				endpoints = append(endpoints, &endpoint)
// 			}
// 		}

// 		return nil
// 	})

// 	//requestParamModel := &EndPointRequestParamModel{DB: m.DB}
// 	responseParamModel := &EndPointResponseParamModel{DB: m.DB}

// 	for _, endpoint := range endpoints {
// 		//endpoint.RequestParams = requestParamModel.ListById(endpoint.ID)
// 		endpoint.ResponseParams = responseParamModel.ListById(endpoint.ID)
// 		//m.LoadConditionGroups(endpoint)
// 	}

// 	return endpoints

// }
