package mongo

import (
	"context"
	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/oauth2.v3/models"
	"testing"
)

func TestClientStore(t *testing.T) {
	store := NewClientStore(NewConfig(url, dbName))
	ctx := context.Background()

	client := &models.Client{
		ID:     "id",
		Secret: "secret",
		Domain: "domain",
		UserID: "user_id",
	}

	Convey("Set", t, func() {
		Convey("HappyPath", func() {
			_ = store.RemoveByID(ctx,client.ID)

			err := store.Set(ctx,client)

			So(err, ShouldBeNil)
		})

		Convey("AlreadyExistingClient", func() {
			_ = store.RemoveByID(ctx,client.ID)

			_ = store.Set(ctx,client)
			err := store.Set(ctx,client)

			So(err, ShouldNotBeNil)
		})
	})

	Convey("GetByID", t, func() {
		Convey("HappyPath", func() {
			_ = store.RemoveByID(ctx,client.ID)
			_ = store.Set(ctx,client)

			got, err := store.GetByID(ctx,client.ID)

			So(err, ShouldBeNil)
			So(got, ShouldResemble, client)
		})

		Convey("UnknownClient", func() {
			_, err := store.GetByID(ctx,"unknown_client")

			So(err, ShouldNotBeNil)
		})
	})

	Convey("RemoveByID", t, func() {
		Convey("UnknownClient", func() {
			err := store.RemoveByID(ctx, "unknown_client")

			So(err, ShouldNotBeNil)
		})
	})
}
