package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// Define a new UserModel type which wraps a database connection pool.
type EndPointModel struct {
	DB *bolt.DB
}

func (m *EndPointModel) BuildEndPointCache(limit int) map[string]*EndPoint {

	cache := make(map[string]*EndPoint)

	endPoints := m.List()

	for i, endPoint := range endPoints {
		if limit > 0 && i+1 > limit {
			break
		}

		if endPoint.CollectionName == "" {
			endPoint.CollectionName = "V1"
		}
		cache[fmt.Sprintf("%s_%s_%s", strings.ToLower(endPoint.CollectionName), strings.ToLower(endPoint.Name), strings.ToLower(endPoint.Method))] = endPoint
	}

	return cache
}

func (m *EndPointModel) getTableName() []byte {
	return []byte("endpoints")
}

// -----------------------------------------------------------------
// CountCreatedBy returns how many endpoints the given user created.
//
// Ownership gained by sharing is deliberately excluded: the per-user quota
// limits what someone creates, and counting endpoints shared *to* them would
// let anyone exhaust another user's allowance by sharing endpoints at them.
// -----------------------------------------------------------------
func (m *EndPointModel) CountCreatedBy(userEmail string) int {
	count := 0

	_ = m.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(m.getTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			endpoint := EndPoint{}
			if err := json.Unmarshal(v, &endpoint); err == nil {
				if strings.EqualFold(strings.TrimSpace(endpoint.CreatedBy), strings.TrimSpace(userEmail)) {
					count++
				}
			}
		}

		return nil
	})

	return count
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointModel) ReBuildURL(u *EndPoint) (string, error) {
	u.BuildMockUrl()
	err := m.Update(u, false)
	return u.ID, err
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointModel) Save(u *EndPoint, userEmail string) (string, error) {
	if u.ID == "" {
		var id string = uuid.NewString()
		u.ID = id
		u.CreatedBy = userEmail
		u.CreatedOn = time.Now().Local()
	}

	u.BuildMockUrl()

	u.BuildResponsePlaceholder()

	//u.BuildResponseHeaderPlaceholder()

	err := m.Update(u, false)

	var wg sync.WaitGroup

	wg.Add(1)
	go m.RebuildResponseHeaderParams(&wg, u)

	wg.Add(1)
	go m.RebuildRequestPathParams(&wg, u)

	wg.Add(1)
	go m.RebuildRequestParams(&wg, u)

	wg.Add(1)
	go m.RebuildResponseParams(&wg, u)

	wg.Add(1)
	go m.RebuildRequestHeaderParams(&wg, u)

	wg.Wait()
	return u.ID, err
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointModel) Update(u *EndPoint, clearCache bool) error {
	err := m.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
		if err != nil {
			return err
		}
		u.Name = strings.ToUpper(strings.TrimSpace(u.Name))
		u.Method = strings.ToUpper(strings.TrimSpace(u.Method))

		if !u.OnHold {
			//u.OnHoldMessage = ""
		} else {
			go u.ClearCache()
		}

		if clearCache {
			go u.ClearCache()
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
func (m *EndPointModel) Delete(id string) error {

	err := m.DB.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
		if err != nil {
			return err
		}
		key := strings.ToUpper(id)
		dbDeleteError := bucket.Delete([]byte(key))
		return dbDeleteError
	})

	// TODO ==> delete request and response params
	if err == nil {
		go func() {
			m1 := &UserModel{DB: m.DB}
			m1.ClearEndPointowners(id)

			m2 := &EndPointRequestParamModel{DB: m.DB}
			m2.ClearEndPointData(id)

			m3 := &EndPointRequestParamModel{DB: m.DB}
			m3.ClearEndPointData(id)

			m4 := &ConditionModel{DB: m.DB}
			m4.ClearEndPointData(id)

			m5 := &ConditionGroupModel{DB: m.DB}
			m5.ClearEndPointData(id)

		}()
	}
	return err
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointModel) Exists(id string) bool {

	var userJson []byte

	_ = m.DB.View(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(m.getTableName())
		if err != nil {
			return err
		}
		key := strings.ToUpper(id)

		userJson = bucket.Get([]byte(key))

		return nil

	})

	return (userJson != nil)
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointModel) DuplicateName(s *EndPoint) bool {
	exists := false
	fullNameToCheck := strings.ToUpper(fmt.Sprintf("%s_%s_%s", s.CollectionID, s.Name, s.Method))

	for _, ep := range m.List() {

		currentFullName := strings.ToUpper(fmt.Sprintf("%s_%s_%s", ep.CollectionID, ep.Name, ep.Method))

		if strings.EqualFold(currentFullName, fullNameToCheck) && !strings.EqualFold(ep.ID, s.ID) {
			exists = true
			break
		}
	}

	return exists
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointModel) Get(id string) (*EndPoint, error) {

	if id == "" {
		return nil, errors.New("Server blank id not allowed")
	}
	var endPointJson []byte // = make([]byte, 0)

	err := m.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(m.getTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}
		endPointJson = bucket.Get([]byte(strings.ToUpper(id)))

		return nil

	})
	endpoint := EndPoint{}
	if err != nil {
		return &endpoint, err
	}

	if endPointJson != nil {
		err := json.Unmarshal(endPointJson, &endpoint)

		if err == nil {
			requestParamModel := &EndPointRequestParamModel{DB: m.DB}
			responseParamModel := &EndPointResponseParamModel{DB: m.DB}
			endpoint.RequestParams = requestParamModel.ListById(endpoint.ID)

			for _, r := range endpoint.ResponseMap {
				r.ResponseParams = responseParamModel.ListByOwnerId(r.ID)
			}

			m.LoadConditionGroups(&endpoint)

		}
		return &endpoint, err

	}

	return &endpoint, ErrServerNotFound

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointModel) List() []*EndPoint {
	endpoints := make([]*EndPoint, 0)
	_ = m.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(m.getTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}
		c := bucket.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {

			endpoint := EndPoint{}
			err := json.Unmarshal(v, &endpoint)
			if err == nil {
				endpoints = append(endpoints, &endpoint)
			}
		}

		return nil
	})

	requestParamModel := &EndPointRequestParamModel{DB: m.DB}
	responseParamModel := &EndPointResponseParamModel{DB: m.DB}

	for _, endpoint := range endpoints {
		endpoint.RequestParams = requestParamModel.ListById(endpoint.ID)
		//endpoint.ResponseParams = responseParamModel.ListById(endpoint.ID)

		for _, r := range endpoint.ResponseMap {
			r.ResponseParams = responseParamModel.ListByOwnerId(r.ID)
		}
		m.LoadConditionGroups(endpoint)
	}

	return endpoints

}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Insert method to add a new record to the "users" table.
func (m *EndPointModel) LoadConditionGroups(u *EndPoint) {
	cgDB := &ConditionGroupModel{DB: m.DB}

	u.ConditionGroups = append(u.ConditionGroups, cgDB.ListById(u.ID)...)
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// We'll use the Exists method to check if a user exists with a specific ID.
func (m *EndPointModel) ListByCollectionID(id string) []*EndPoint {
	endpoints := make([]*EndPoint, 0)
	_ = m.DB.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(m.getTableName())
		if bucket == nil {
			return errors.New("table does not exits")
		}
		c := bucket.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {

			endpoint := EndPoint{}
			err := json.Unmarshal(v, &endpoint)

			// load only for the given collection id
			if err == nil && endpoint.CollectionID == id {
				endpoints = append(endpoints, &endpoint)
			}
		}

		return nil
	})

	requestParamModel := &EndPointRequestParamModel{DB: m.DB}
	responseParamModel := &EndPointResponseParamModel{DB: m.DB}

	for _, endpoint := range endpoints {
		endpoint.RequestParams = requestParamModel.ListById(endpoint.ID)
		//endpoint.ResponseParams = responseParamModel.ListById(endpoint.ID)

		for _, r := range endpoint.ResponseMap {
			r.ResponseParams = responseParamModel.ListByOwnerId(r.ID)
		}
		m.LoadConditionGroups(endpoint)
	}

	return endpoints

}
