package mongo

import (
	"encoding/json"
	"time"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"github.com/globalsign/mgo/txn"
	"gopkg.in/oauth2.v3"
	"gopkg.in/oauth2.v3/models"
)

// Config mongodb configuration parameters
type Config struct {
	URL string
	DB  string
}

// NewConfig create mongodb configuration
func NewConfig(url, db string) *Config {
	return &Config{
		URL: url,
		DB:  db,
	}
}

// TokenConfig token configuration parameters
type TokenConfig struct {
	// store txn collection name(The default is oauth2)
	TxnCName string
	// store token based data collection name(The default is oauth2_basic)
	BasicCName string
	// store access token data collection name(The default is oauth2_access)
	AccessCName string
	// store refresh token data collection name(The default is oauth2_refresh)
	RefreshCName string
}

// NewDefaultTokenConfig create a default token configuration
func NewDefaultTokenConfig() *TokenConfig {
	return &TokenConfig{
		TxnCName:     "oauth2_txn",
		BasicCName:   "oauth2_basic",
		AccessCName:  "oauth2_access",
		RefreshCName: "oauth2_refresh",
	}
}

// NewTokenStore create a token store instance based on mongodb
func NewTokenStore(cfg *Config, tcfgs ...*TokenConfig) (store *TokenStore) {
	session, err := mgo.Dial(cfg.URL)
	if err != nil {
		panic(err)
	}

	return NewTokenStoreWithSession(session, cfg.DB, tcfgs...)
}

// NewTokenStoreWithSession create a token store instance based on mongodb
func NewTokenStoreWithSession(session *mgo.Session, dbName string, tcfgs ...*TokenConfig) (store *TokenStore) {
	ts := &TokenStore{
		dbName:  dbName,
		session: session,
		tcfg:    NewDefaultTokenConfig(),
	}
	if len(tcfgs) > 0 {
		ts.tcfg = tcfgs[0]
	}

	ts.c(ts.tcfg.BasicCName).EnsureIndex(mgo.Index{
		Key:         []string{"ExpiredAt"},
		ExpireAfter: time.Second * 1,
	})

	ts.c(ts.tcfg.AccessCName).EnsureIndex(mgo.Index{
		Key:         []string{"ExpiredAt"},
		ExpireAfter: time.Second * 1,
	})

	ts.c(ts.tcfg.RefreshCName).EnsureIndex(mgo.Index{
		Key:         []string{"ExpiredAt"},
		ExpireAfter: time.Second * 1,
	})

	store = ts
	return
}

// TokenStore MongoDB storage for OAuth 2.0
type TokenStore struct {
	tcfg    *TokenConfig
	dbName  string
	session *mgo.Session
}

// Close close the mongo session
func (ts *TokenStore) Close() {
	ts.session.Close()
}

func (ts *TokenStore) c(name string) *mgo.Collection {
	return ts.session.DB(ts.dbName).C(name)
}

func (ts *TokenStore) cHandler(name string, handler func(c *mgo.Collection)) {
	session := ts.session.Clone()
	defer session.Close()
	handler(session.DB(ts.dbName).C(name))
	return
}

// Create create and store the new token information
func (ts *TokenStore) Create(info oauth2.TokenInfo) (err error) {
	jv, err := json.Marshal(info)
	if err != nil {
		return
	}

	if code := info.GetCode(); code != "" {
		ts.cHandler(ts.tcfg.BasicCName, func(c *mgo.Collection) {
			err = c.Insert(basicData{
				ID:        code,
				Data:      jv,
				ExpiredAt: info.GetCodeCreateAt().Add(info.GetCodeExpiresIn()),
			})
		})
		return
	}

	aexp := info.GetAccessCreateAt().Add(info.GetAccessExpiresIn())
	rexp := aexp
	if refresh := info.GetRefresh(); refresh != "" {
		rexp = info.GetRefreshCreateAt().Add(info.GetRefreshExpiresIn())
		if aexp.Second() > rexp.Second() {
			aexp = rexp
		}
	}
	id := bson.NewObjectId().Hex()
	ops := []txn.Op{{
		C:      ts.tcfg.BasicCName,
		Id:     id,
		Assert: txn.DocMissing,
		Insert: basicData{
			Data:      jv,
			ExpiredAt: rexp,
		},
	}, {
		C:      ts.tcfg.AccessCName,
		Id:     info.GetAccess(),
		Assert: txn.DocMissing,
		Insert: tokenData{
			BasicID:   id,
			ExpiredAt: aexp,
		},
	}}
	if refresh := info.GetRefresh(); refresh != "" {
		ops = append(ops, txn.Op{
			C:      ts.tcfg.RefreshCName,
			Id:     refresh,
			Assert: txn.DocMissing,
			Insert: tokenData{
				BasicID:   id,
				ExpiredAt: rexp,
			},
		})
	}
	ts.cHandler(ts.tcfg.TxnCName, func(c *mgo.Collection) {
		runner := txn.NewRunner(c)
		err = runner.Run(ops, "", nil)
	})
	return
}

// RemoveByCode use the authorization code to delete the token information
func (ts *TokenStore) RemoveByCode(code string) (err error) {
	ts.cHandler(ts.tcfg.BasicCName, func(c *mgo.Collection) {
		verr := c.RemoveId(code)
		if verr != nil {
			if verr == mgo.ErrNotFound {
				return
			}
			err = verr
		}
	})
	return
}

// RemoveByAccess use the access token to delete the token information
func (ts *TokenStore) RemoveByAccess(access string) (err error) {
	ts.cHandler(ts.tcfg.AccessCName, func(c *mgo.Collection) {
		verr := c.RemoveId(access)
		if verr != nil {
			if verr == mgo.ErrNotFound {
				return
			}
			err = verr
		}
	})
	return
}

// RemoveByRefresh use the refresh token to delete the token information
func (ts *TokenStore) RemoveByRefresh(refresh string) (err error) {
	ts.cHandler(ts.tcfg.RefreshCName, func(c *mgo.Collection) {
		verr := c.RemoveId(refresh)
		if verr != nil {
			if verr == mgo.ErrNotFound {
				return
			}
			err = verr
		}
	})
	return
}

func (ts *TokenStore) getData(basicID string) (ti oauth2.TokenInfo, err error) {
	ts.cHandler(ts.tcfg.BasicCName, func(c *mgo.Collection) {
		var bd basicData
		verr := c.FindId(basicID).One(&bd)
		if verr != nil {
			if verr == mgo.ErrNotFound {
				return
			}
			err = verr
			return
		}
		var tm models.Token
		err = json.Unmarshal(bd.Data, &tm)
		if err != nil {
			return
		}
		ti = &tm
	})
	return
}

func (ts *TokenStore) getBasicID(cname, token string) (basicID string, err error) {
	ts.cHandler(cname, func(c *mgo.Collection) {
		var td tokenData
		verr := c.FindId(token).One(&td)
		if verr != nil {
			if verr == mgo.ErrNotFound {
				return
			}
			err = verr
			return
		}
		basicID = td.BasicID
	})
	return
}

// GetByCode use the authorization code for token information data
func (ts *TokenStore) GetByCode(code string) (ti oauth2.TokenInfo, err error) {
	ti, err = ts.getData(code)
	return
}

// GetByAccess use the access token for token information data
func (ts *TokenStore) GetByAccess(access string) (ti oauth2.TokenInfo, err error) {
	basicID, err := ts.getBasicID(ts.tcfg.AccessCName, access)
	if err != nil && basicID == "" {
		return
	}
	ti, err = ts.getData(basicID)
	return
}

// GetByRefresh use the refresh token for token information data
func (ts *TokenStore) GetByRefresh(refresh string) (ti oauth2.TokenInfo, err error) {
	basicID, err := ts.getBasicID(ts.tcfg.RefreshCName, refresh)
	if err != nil && basicID == "" {
		return
	}
	ti, err = ts.getData(basicID)
	return
}

type basicData struct {
	ID        string    `bson:"_id"`
	Data      []byte    `bson:"Data"`
	ExpiredAt time.Time `bson:"ExpiredAt"`
}

type tokenData struct {
	ID        string    `bson:"_id"`
	BasicID   string    `bson:"BasicID"`
	ExpiredAt time.Time `bson:"ExpiredAt"`
}
