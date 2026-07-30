package models

import (
	"errors"

	bolt "go.etcd.io/bbolt"
)

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func DailyDataCleanup_TESTMODE(db *bolt.DB) {
	//go DeleteALLEndpoint(db)
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
// DailyDataCleanup prunes both databases: db is the main store (internal.db),
// logDB the current daily call log (log_YYYYMMDD.db).
func DailyDataCleanup(db *bolt.DB, logDB *bolt.DB) {
	ClearupDeleteendpointsFromOwnr(db)
	go ClearLogs(db, logDB)
	go ClearUserVerificationData(db)
}

// -----------------------------------------------------------------
//
// -----------------------------------------------------------------
func ClearupDeleteendpointsFromOwnr(db *bolt.DB) {

	userModel := &UserModel{DB: db}
	endPointModel := &EndPointModel{DB: db}

	for _, u := range userModel.List() {
		newOwnList := make([]string, 0)
		for _, epid := range u.OwnedEndPoints {

			_, err := endPointModel.Get(epid)
			if err == nil {
				newOwnList = append(newOwnList, epid)
			}
		}

		u.OwnedEndPoints = newOwnList

		userModel.Save(u, false)
	}

}

// ------------------------------------------------------
//
// ------------------------------------------------------
// ClearLogs discards stored API call logs.
//
// These span two BoltDB files: the call logs themselves are written by
// ApiCall.SaveLogs into the daily log database, while the per-endpoint call log
// is a field on the endpoint record in the main database. Passing the main
// database for both silently left the log database untouched.
func ClearLogs(db *bolt.DB, logDB *bolt.DB) {
	err := logDB.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(getLogTableName()); err != nil && !errors.Is(err, bolt.ErrBucketNotFound) {
			return err
		}
		_, err := tx.CreateBucketIfNotExists(getLogTableName())
		return err
	})
	if err != nil {
		errorLog.Println("clearing api logs:", err)
	}

	// clear end point log entrie

	epm := &EndPointModel{DB: db}

	eps := epm.List()

	for _, ep := range eps {
		ep.EndPointCallLog = make([]EndPointCallLog, 0)
		epm.Save(ep, "")
	}

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func ClearUserVerificationData(db *bolt.DB) {

	userModel := &UserModel{DB: db}

	db.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket(userModel.GetVerificationTableName())
		tx.CreateBucketIfNotExists(userModel.GetVerificationTableName())

		tx.DeleteBucket(userModel.GetPasswordResetTableName())
		tx.CreateBucketIfNotExists(userModel.GetPasswordResetTableName())
		return nil
	})

}

// ------------------------------------------------------
//
// ------------------------------------------------------
func DeleteALLEndpoint(db *bolt.DB) {

	epm := &EndPointModel{DB: db}

	for _, e := range epm.List() {
		epm.Delete(e.ID)
	}
}
