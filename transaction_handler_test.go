package mongo

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4/models"
	. "github.com/smartystreets/goconvey/convey"
)

// record the called methods
var record = []string{}
var callNumber int

// shut the the down the database, test should fail within a second
func TestTransaction(t *testing.T) {

	info := &models.Token{
		ClientID:         "1",
		UserID:           "1_2",
		RedirectURI:      "http://localhost/",
		Scope:            "all",
		Access:           "1_2_1",
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Second * 5,
		Refresh:          "1_2_2",
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: time.Second * 15,
	}

	basicData := basicData{
		ID:        "functionName",
		Data:      []byte("success"),
		ExpiredAt: time.Now(),
	}

	accessData := tokenData{
		ID:        "functionName",
		BasicID:   "success",
		ExpiredAt: time.Now(),
	}

	storeConfig := NewStoreConfig(1, 5)

	if isReplicaSet {
		t.Skip("Skipping the test as it is a replica set.")
	}

	store := NewTokenStore(NewConfigNonReplicaSet(url, dbName, username, password, service), storeConfig)

	store.txnHandler.tw = &mockTransactionWorker{}

	Convey("Test mongodb token store", t, func() {

		// if replicaSet stop
		if store.tcfg.storeConfig.isReplicaSet {
			t.Skip("Skipping the test as replicaSet is enabled")
		}

		Convey("Test insertBasicTransactionData fail", func() {
			basicData.ID = "insertBasicTransactionData"
			store.txnHandler.tcfg.BasicCName = "fail"

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err.Error(), ShouldEqual, "insertBasicTransactionData")
			So(len(record), ShouldEqual, 1)
			So(record[0], ShouldEqual, "insertBasicTransactionData") // fail

			record = []string{}
		})

		Convey("Test insertBasicData fail", func() {
			basicData.ID = "insertBasicData"
			basicData.Data = []byte("fail")

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err.Error(), ShouldEqual, "insertBasicData")
			So(len(record), ShouldEqual, 3)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData") // fail
			So(record[2], ShouldEqual, "removeTransactionData")

			record = []string{}
		})

		// transaction succeed
		Convey("Test insertBasicData success", func() {
			basicData.ID = "insertBasicData"
			basicData.Data = []byte("success")

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err, ShouldBeNil)
			So(len(record), ShouldEqual, 7)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData")
			So(record[3], ShouldEqual, "insertTokenData") // accessToken
			So(record[4], ShouldEqual, "insertTokenData") // refreshToken
			So(record[5], ShouldEqual, "removeTransactionData")
			So(record[6], ShouldEqual, "removeTransactionData")

			record = []string{}
		})

		Convey("Test insertTokenTransactionData fail", func() {
			info.Access = "insertTokenTransactionData"
			store.txnHandler.tcfg.AccessCName = "fail"

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err.Error(), ShouldEqual, "insertTokenTransactionData")
			So(len(record), ShouldEqual, 5)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData") // fail
			So(record[3], ShouldEqual, "removeBasicData")
			So(record[4], ShouldEqual, "removeTransactionData")

			// clean
			record = []string{}
			info.Access = "1_2_1"
			store.txnHandler.tcfg.AccessCName = "AccessCName"

		})

		Convey("Test insertTokenData(access) fail", func() {
			accessData.ID = "insertTokenData"
			accessData.BasicID = "fail"

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err.Error(), ShouldEqual, "insertTokenData")
			So(len(record), ShouldEqual, 7)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData")
			So(record[3], ShouldEqual, "insertTokenData") // fail
			So(record[4], ShouldEqual, "removeBasicData")
			So(record[5], ShouldEqual, "removeTransactionData")
			So(record[6], ShouldEqual, "removeTransactionData")

			accessData.BasicID = "success"
			record = []string{}
		})

		// transaction succeed
		Convey("Test insertTokenData(access) success", func() {
			accessData.ID = "insertTokenData"

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err, ShouldBeNil)
			So(len(record), ShouldEqual, 7)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData")
			So(record[3], ShouldEqual, "insertTokenData") // accessToken
			So(record[4], ShouldEqual, "insertTokenData") // refreshToken
			So(record[5], ShouldEqual, "removeTransactionData")
			So(record[6], ShouldEqual, "removeTransactionData")

			record = []string{}
			callNumber = 0
		})

		// transaction succeed
		Convey("Test insertTokenData(refresh) success", func() {
			accessData.ID = "insertTokenDataRefresh"
			info.Refresh = "insertTokenDataRefresh"

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "success", time.Now())

			So(err, ShouldBeNil)
			So(len(record), ShouldEqual, 7)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData")
			So(record[3], ShouldEqual, "insertTokenData") // accessToken
			So(record[4], ShouldEqual, "insertTokenData") // refreshToken
			So(record[5], ShouldEqual, "removeTransactionData")
			So(record[6], ShouldEqual, "removeTransactionData")

			record = []string{}
			callNumber = 0
		})

		Convey("Test insertTokenData(refresh) fail", func() {
			accessData.ID = "insertTokenDataRefresh"
			info.Refresh = "insertTokenDataRefresh"

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "fail", time.Now())

			So(err.Error(), ShouldEqual, "insertTokenDataRefresh")
			So(len(record), ShouldEqual, 9)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData")
			So(record[3], ShouldEqual, "insertTokenData")
			So(record[4], ShouldEqual, "insertTokenData") // fail insertRefreshToken
			So(record[5], ShouldEqual, "removeBasicData")
			So(record[6], ShouldEqual, "removeTransactionData")
			So(record[7], ShouldEqual, "removeTokenData")
			So(record[8], ShouldEqual, "removeTransactionData")

			record = []string{}
		})

		/*
		* case no refreshToken
		* transaction succeed
		**/
		Convey("Test insertTokenData(access) success with no refreshToken", func() {
			accessData.ID = "insertTokenData"
			info.Refresh = ""

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			So(err, ShouldBeNil)
			So(len(record), ShouldEqual, 6)
			So(record[0], ShouldEqual, "insertBasicTransactionData")
			So(record[1], ShouldEqual, "insertBasicData")
			So(record[2], ShouldEqual, "insertBasicTransactionData")
			So(record[3], ShouldEqual, "insertTokenData") // accessToken
			So(record[4], ShouldEqual, "removeTransactionData")
			So(record[5], ShouldEqual, "removeTransactionData")

			record = []string{}
			callNumber = 0
		})

	})
}

// mock the transactionWorker
type mockTransactionWorker struct{}

func (mt *mockTransactionWorker) insertBasicData(ctx context.Context, basicData basicData) error {
	record = append(record, "insertBasicData")
	if basicData.ID == "insertBasicData" {
		if string(basicData.Data) == "success" {
			return nil
		} else if string(basicData.Data) == "fail" {
			return errors.New("insertBasicData")
		}
	}
	return nil
}

func (mt *mockTransactionWorker) removeBasicData(ctx context.Context, basicDataID string) error {
	record = append(record, "removeBasicData")
	if basicDataID == "fail" {
		return errors.New("removeBasicData")
	}
	return nil
}

func (mt *mockTransactionWorker) insertBasicTransactionData(ctx context.Context, txnData transactionData) error {
	record = append(record, "insertBasicTransactionData")
	if txnData.ID == "insertBasicTransactionData" {
		if txnData.Collection == "fail" {
			return errors.New("insertBasicTransactionData")
		}
	}
	return nil
}

func (mt *mockTransactionWorker) insertTokenData(ctx context.Context, tokenData tokenData, collectionName string) error {
	record = append(record, "insertTokenData")
	if tokenData.ID == "insertTokenData" {
		if tokenData.BasicID == "fail" {
			return errors.New("insertTokenData")
		}
	} else if tokenData.ID == "insertTokenDataRefresh" && callNumber == 0 {
		callNumber++
	} else if tokenData.ID == "insertTokenDataRefresh" && callNumber == 1 {
		if tokenData.BasicID == "fail" {
			return errors.New("insertTokenDataRefresh")
		}
	}
	return nil
}

func (mt *mockTransactionWorker) removeTokenData(ctx context.Context, tokenDataID, collectionName string) error {
	record = append(record, "removeTokenData")
	return nil
}

func (mt *mockTransactionWorker) insertTokenTransactionData(ctx context.Context, txnData transactionData) error {
	record = append(record, "insertBasicTransactionData")
	if txnData.ID == "insertTokenTransactionData" {
		if txnData.Collection == "fail" {
			return errors.New("insertTokenTransactionData")
		}
	}

	return nil
}

func (mt *mockTransactionWorker) removeTransactionData(ctx context.Context, tokenDataID string) error {
	record = append(record, "removeTransactionData")
	return nil
}

func (mt *mockTransactionWorker) cleanupTransactionsData(ctx context.Context, service string) error {
	record = append(record, "cleanupTransactionsData")
	return nil
}
