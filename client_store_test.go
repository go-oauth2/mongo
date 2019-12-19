package mongo

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/oauth2.v3/models"
)

func TestClientStore(t *testing.T) {
	store := NewClientStore(NewConfig(url, dbName))

	client := &models.Client{
		ID:     "id",
		Secret: "secret",
		Domain: "domain",
		UserID: "user_id",
	}

	Convey("Set", t, func() {
		Convey("HappyPath", func() {
			_ = store.RemoveByID(client.ID)

			err := store.Set(client)

			So(err, ShouldBeNil)
		})

		Convey("AlreadyExistingClient", func() {
			_ = store.RemoveByID(client.ID)

			_ = store.Set(client)
			err := store.Set(client)

			So(err, ShouldNotBeNil)
		})
	})

	Convey("GetByID", t, func() {
		Convey("HappyPath", func() {
			_ = store.RemoveByID(client.ID)
			_ = store.Set(client)

			got, err := store.GetByID(client.ID)

			So(err, ShouldBeNil)
			So(got, ShouldResemble, client)
		})

		Convey("UnknownClient", func() {
			_, err := store.GetByID("unknown_client")

			So(err, ShouldNotBeNil)
		})
	})

	Convey("RemoveByID", t, func() {
		Convey("UnknownClient", func() {
			err := store.RemoveByID("unknown_client")

			So(err, ShouldNotBeNil)
		})
	})
}
