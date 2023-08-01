package mongo

import (
	"context"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4/models"

	. "github.com/smartystreets/goconvey/convey"
)

// shut the the down the database, test should fail within a second
func TestTokenStoreWithTimeout(t *testing.T) {
	Convey("Test mongodb token store", t, func() {

		storeConfig := NewStoreConfig(1, 5)

		var store *TokenStore
		if !isReplicaSet {
			store = NewTokenStore(NewConfigNonReplicaSet(url, dbName, username, password, service), storeConfig)
		} else {
			store = NewTokenStore(NewConfigReplicaSet(url, dbName), storeConfig)
		}

		Convey("Test authorization code store", func() {
			info := &models.Token{
				ClientID:      "1",
				UserID:        "1_1",
				RedirectURI:   "http://localhost/",
				Scope:         "all",
				Code:          "11_11_11",
				CodeCreateAt:  time.Now(),
				CodeExpiresIn: time.Second * 5,
			}
			err := store.Create(context.TODO(), info)
			So(err, ShouldBeNil)

			cinfo, err := store.GetByCode(context.TODO(), info.Code)
			So(err, ShouldBeNil)
			So(cinfo.GetUserID(), ShouldEqual, info.UserID)

			err = store.RemoveByCode(context.TODO(), info.Code)
			So(err, ShouldBeNil)

			cinfo, err = store.GetByCode(context.TODO(), info.Code)
			So(err, ShouldBeNil)
			So(cinfo, ShouldBeNil)
		})

		Convey("Test access token store", func() {
			info := &models.Token{
				ClientID:        "1",
				UserID:          "1_1",
				RedirectURI:     "http://localhost/",
				Scope:           "all",
				Access:          "1_1_1",
				AccessCreateAt:  time.Now(),
				AccessExpiresIn: time.Second * 5,
			}
			err := store.Create(context.TODO(), info)
			So(err, ShouldBeNil)

			ainfo, err := store.GetByAccess(context.TODO(), info.GetAccess())
			So(err, ShouldBeNil)
			So(ainfo.GetUserID(), ShouldEqual, info.GetUserID())

			err = store.RemoveByAccess(context.TODO(), info.GetAccess())
			So(err, ShouldBeNil)

			ainfo, err = store.GetByAccess(context.TODO(), info.GetAccess())
			So(err.Error(), ShouldEqual, "mongo: no documents in result")
			So(ainfo, ShouldBeNil)

			// cleanup
			err = store.RemoveByCode(context.TODO(), info.ClientID)
			So(err, ShouldBeNil)

		})

		Convey("Test refresh token store", func() {
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
			err := store.Create(context.TODO(), info)
			So(err, ShouldBeNil)

			rinfo, err := store.GetByRefresh(context.TODO(), info.GetRefresh())
			So(err, ShouldBeNil)
			So(rinfo.GetUserID(), ShouldEqual, info.GetUserID())

			err = store.RemoveByRefresh(context.TODO(), info.GetRefresh())
			So(err, ShouldBeNil)

			rinfo, err = store.GetByRefresh(context.TODO(), info.GetRefresh())
			So(err.Error(), ShouldEqual, "mongo: no documents in result")
			So(rinfo, ShouldBeNil)

			// cleanup
			err = store.RemoveByCode(context.TODO(), info.ClientID)
			So(err, ShouldBeNil)

			err = store.RemoveByAccess(context.TODO(), info.GetAccess())
			So(err, ShouldBeNil)

			err = store.RemoveByRefresh(context.TODO(), info.GetRefresh())
			So(err, ShouldBeNil)
		})
	})
}

func TestTokenStore(t *testing.T) {
	Convey("Test mongodb token store", t, func() {
		var store *TokenStore
		if !isReplicaSet {
			store = NewTokenStore(NewConfigNonReplicaSet(url, dbName, username, password, service))
		} else {
			store = NewTokenStore(NewConfigReplicaSet(url, dbName))
		}

		Convey("Test authorization code store", func() {
			info := &models.Token{
				ClientID:      "1",
				UserID:        "1_1",
				RedirectURI:   "http://localhost/",
				Scope:         "all",
				Code:          "11_11_11",
				CodeCreateAt:  time.Now(),
				CodeExpiresIn: time.Second * 5,
			}
			err := store.Create(context.TODO(), info)
			So(err, ShouldBeNil)

			cinfo, err := store.GetByCode(context.TODO(), info.Code)
			So(err, ShouldBeNil)
			So(cinfo.GetUserID(), ShouldEqual, info.UserID)

			err = store.RemoveByCode(context.TODO(), info.Code)
			So(err, ShouldBeNil)

			cinfo, err = store.GetByCode(context.TODO(), info.Code)
			So(err, ShouldBeNil)
			So(cinfo, ShouldBeNil)
		})

		Convey("Test access token store", func() {
			info := &models.Token{
				ClientID:        "1",
				UserID:          "1_1",
				RedirectURI:     "http://localhost/",
				Scope:           "all",
				Access:          "1_1_1",
				AccessCreateAt:  time.Now(),
				AccessExpiresIn: time.Second * 5,
			}
			err := store.Create(context.TODO(), info)
			So(err, ShouldBeNil)

			ainfo, err := store.GetByAccess(context.TODO(), info.GetAccess())
			So(err, ShouldBeNil)
			So(ainfo.GetUserID(), ShouldEqual, info.GetUserID())

			err = store.RemoveByAccess(context.TODO(), info.GetAccess())
			So(err, ShouldBeNil)

			ainfo, err = store.GetByAccess(context.TODO(), info.GetAccess())
			So(err.Error(), ShouldEqual, "mongo: no documents in result")
			So(ainfo, ShouldBeNil)
		})

		Convey("Test refresh token store", func() {
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
			err := store.Create(context.TODO(), info)
			So(err, ShouldBeNil)

			rinfo, err := store.GetByRefresh(context.TODO(), info.GetRefresh())
			So(err, ShouldBeNil)
			So(rinfo.GetUserID(), ShouldEqual, info.GetUserID())

			err = store.RemoveByRefresh(context.TODO(), info.GetRefresh())
			So(err, ShouldBeNil)

			rinfo, err = store.GetByRefresh(context.TODO(), info.GetRefresh())
			So(err.Error(), ShouldEqual, "mongo: no documents in result")
			So(rinfo, ShouldBeNil)

			// cleanup
			err = store.RemoveByCode(context.TODO(), info.ClientID)
			So(err, ShouldBeNil)

			err = store.RemoveByAccess(context.TODO(), info.GetAccess())
			So(err, ShouldBeNil)

			err = store.RemoveByRefresh(context.TODO(), info.GetRefresh())
			So(err, ShouldBeNil)

		})
	})
}
