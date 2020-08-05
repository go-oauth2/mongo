package mongo

import (
	"context"
	"github.com/globalsign/mgo"
	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
)

// ClientConfig client configuration parameters
type ClientConfig struct {
	// store clients data collection name(The default is oauth2_clients)
	ClientsCName string
}

// NewDefaultClientConfig create a default client configuration
func NewDefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ClientsCName: "oauth2_clients",
	}
}

// NewClientStore create a client store instance based on mongodb
func NewClientStore(cfg *Config, ccfgs ...*ClientConfig) *ClientStore {
	session, err := mgo.Dial(cfg.URL)
	if err != nil {
		panic(err)
	}

	return NewClientStoreWithSession(session, cfg.DB, ccfgs...)
}

// NewClientStoreWithSession create a client store instance based on mongodb
func NewClientStoreWithSession(session *mgo.Session, dbName string, ccfgs ...*ClientConfig) *ClientStore {
	cs := &ClientStore{
		dbName:  dbName,
		session: session,
		ccfg:    NewDefaultClientConfig(),
	}
	if len(ccfgs) > 0 {
		cs.ccfg = ccfgs[0]
	}

	return cs
}

// ClientStore MongoDB storage for OAuth 2.0
type ClientStore struct {
	ccfg    *ClientConfig
	dbName  string
	session *mgo.Session
}

// Close close the mongo session
func (cs *ClientStore) Close() {
	cs.session.Close()
}

func (cs *ClientStore) c(name string) *mgo.Collection {
	return cs.session.DB(cs.dbName).C(name)
}

func (cs *ClientStore) cHandler(name string, handler func(c *mgo.Collection)) {
	session := cs.session.Clone()
	defer session.Close()
	handler(session.DB(cs.dbName).C(name))
	return
}

// Set set client information
func (cs *ClientStore) Set(ctx context.Context, info oauth2.ClientInfo) (err error) {
	cs.cHandler(cs.ccfg.ClientsCName, func(c *mgo.Collection) {
		entity := &client{
			ID:     info.GetID(),
			Secret: info.GetSecret(),
			Domain: info.GetDomain(),
			UserID: info.GetUserID(),
		}

		if cerr := c.Insert(entity); cerr != nil {
			err = cerr
			return
		}
	})

	return
}

// GetByID according to the ID for the client information
func (cs *ClientStore) GetByID(ctx context.Context, id string) (info oauth2.ClientInfo, err error) {
	cs.cHandler(cs.ccfg.ClientsCName, func(c *mgo.Collection) {
		entity := new(client)

		if cerr := c.FindId(id).One(entity); cerr != nil {
			err = cerr
			return
		}

		info = &models.Client{
			ID:     entity.ID,
			Secret: entity.Secret,
			Domain: entity.Domain,
			UserID: entity.UserID,
		}
	})

	return
}

// RemoveByID use the client id to delete the client information
func (cs *ClientStore) RemoveByID(ctx context.Context, id string) (err error) {
	cs.cHandler(cs.ccfg.ClientsCName, func(c *mgo.Collection) {
		if cerr := c.RemoveId(id); cerr != nil {
			err = cerr
			return
		}
	})

	return
}

type client struct {
	ID     string `bson:"_id"`
	Secret string `bson:"secret"`
	Domain string `bson:"domain"`
	UserID string `bson:"userid"`
}
