package mongo

import (
	"context"
	"log"

	// "log"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4/models"
	// "github.com/stretchr/testify/mock"
	// "github.com/go-oauth2/oauth2/v4"

	. "github.com/smartystreets/goconvey/convey"
)

type mockTransactionWorker struct{}

func (mt *mockTransactionWorker) insertBasicData(ctx context.Context, basicData basicData) error {
	return nil
}
func (mt *mockTransactionWorker) removeBasicData(ctx context.Context, basicDataID string) error {
	return nil
}

func (mt *mockTransactionWorker) insertBasicTransactionData(ctx context.Context, txnData transactionData) error {

	return nil
}
func (mt *mockTransactionWorker) insertTokenData(ctx context.Context, tokenData tokenData, collectionName string) error {

	return nil
}
func (mt *mockTransactionWorker) removeTokenData(ctx context.Context, tokenDataID, collectionName string) error {

	return nil
}
func (mt *mockTransactionWorker) insertTokenTransactionData(ctx context.Context, txnData transactionData) error {

	return nil
}
func (mt *mockTransactionWorker) removeTransactionData(ctx context.Context, tokenDataID string) error {

	return nil
}
func (mt *mockTransactionWorker) cleanupTransactionsData(ctx context.Context, service string) error {

	return nil
}

// type MockDB struct {
// 	mk mock.Mock
// }
//
// func NewMockDB() *MockDB {
// 	return &MockDB{
// 		mk: mock.Mock{},
// 	}
// }
//
// func (m *MockDB) InsertBasicTransactionData(ctx context.Context, data interface{}) error {
// 	args := m.mk.Called(ctx, data)
// 	return args.Error(0)
// }

// shut the the down the database, test should fail within a second
func TestTransaction(t *testing.T) {
	// mockDB := NewMockDB()

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
		// ID:        "",
		Data: []byte("marshalled info"),
		// ExpiredAt: time.Now(),
	}

	accessData := tokenData{
		ID:        info.GetAccess(),
		BasicID:   "the basicID",
		ExpiredAt: time.Now(),
	}

	storeConfig := NewStoreConfig(1, 5)
	store := NewTokenStore(NewConfigNonReplicaSet(url, dbName, username, password, service), storeConfig)
	store.txnHandler.tw = &mockTransactionWorker{}

	Convey("Test mongodb token store", t, func() {

		// if replicaSet stop
		if store.tcfg.storeConfig.isReplicaSet {
			t.Skip("Skipping the test as replicaSet is enabled")
		}

		Convey("Test insert basic transaction data fail", func() {

			err := store.txnHandler.runTransactionCreate(context.TODO(), info, basicData, accessData, "an id", time.Now())

			log.Println("theeeeeeeee errrr: ", err)
			So(err, ShouldBeNil)

			// rinfo, err := store.GetByRefresh(context.TODO(), info.GetRefresh())
			// So(err, ShouldBeNil)
			// So(rinfo.GetUserID(), ShouldEqual, info.GetUserID())

			// err = store.RemoveByRefresh(context.TODO(), info.GetRefresh())
			// So(err, ShouldBeNil)

			// rinfo, err = store.GetByRefresh(context.TODO(), info.GetRefresh())
			// So(err.Error(), ShouldEqual, "mongo: no documents in result")
			// So(rinfo, ShouldBeNil)

			// // cleanup
			// err = store.RemoveByCode(context.TODO(), info.ClientID)
			// So(err, ShouldBeNil)

			// err = store.RemoveByAccess(context.TODO(), info.GetAccess())
			// So(err, ShouldBeNil)

			// err = store.RemoveByRefresh(context.TODO(), info.GetRefresh())
			// So(err, ShouldBeNil)
		})
	})
}
