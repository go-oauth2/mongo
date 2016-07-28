package mongo

import (
	"encoding/json"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/txn"
	"gopkg.in/oauth2.v3"
	"gopkg.in/oauth2.v3/models"
)

// TokenConfig Token Config
type TokenConfig struct {
	TxnCName     string // Store txn collection name(The default is oauth2)
	BasicCName   string // Store token based data collection name(The default is oauth2_basic)
	AccessCName  string // Store access token data collection name(The default is oauth2_access)
	RefreshCName string // Store refresh token data collection name(The default is oauth2_refresh)
}

// NewDefaultTokenConfig Create default token config
func NewDefaultTokenConfig() *TokenConfig {
	return &TokenConfig{
		TxnCName:     "oauth2_txn",
		BasicCName:   "oauth2_basic",
		AccessCName:  "oauth2_access",
		RefreshCName: "oauth2_refresh",
	}
}

// NewTokenStore Create a token store instance based on mongodb
func NewTokenStore(cfg *Config, tcfgs ...*TokenConfig) (store oauth2.TokenStore, err error) {
	ts := &TokenStore{
		mcfg: cfg,
		tcfg: NewDefaultTokenConfig(),
	}
	if len(tcfgs) > 0 {
		ts.tcfg = tcfgs[0]
	}
	session, err := mgo.Dial(ts.mcfg.URL)
	if err != nil {
		return
	}
	ts.session = session
	err = ts.c(ts.tcfg.BasicCName).EnsureIndex(mgo.Index{
		Key:         []string{"ExpiredAt"},
		ExpireAfter: time.Second * 1,
	})
	if err != nil {
		return
	}
	err = ts.c(ts.tcfg.AccessCName).EnsureIndex(mgo.Index{
		Key:         []string{"ExpiredAt"},
		ExpireAfter: time.Second * 1,
	})
	if err != nil {
		return
	}
	err = ts.c(ts.tcfg.RefreshCName).EnsureIndex(mgo.Index{
		Key:         []string{"ExpiredAt"},
		ExpireAfter: time.Second * 1,
	})
	if err != nil {
		return
	}
	store = ts
	return
}

// TokenStore MongoDB token store
type TokenStore struct {
	tcfg    *TokenConfig
	mcfg    *Config
	session *mgo.Session
}

func (ts *TokenStore) c(name string) *mgo.Collection {
	return ts.session.DB(ts.mcfg.DB).C(name)
}

// Create Create and store the new token information
func (ts *TokenStore) Create(info oauth2.TokenInfo) (err error) {
	jv, err := json.Marshal(info)
	if err != nil {
		return
	}

	if code := info.GetCode(); code != "" {
		err = ts.c(ts.tcfg.BasicCName).Insert(basicData{
			ID:        code,
			Data:      jv,
			ExpiredAt: info.GetCodeCreateAt().Add(info.GetCodeExpiresIn()),
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
	runner := txn.NewRunner(ts.c(ts.tcfg.TxnCName))
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
	err = runner.Run(ops, "", nil)
	return
}

// RemoveByCode Use the authorization code to delete the token information
func (ts *TokenStore) RemoveByCode(code string) (err error) {
	verr := ts.c(ts.tcfg.BasicCName).RemoveId(code)
	if verr != nil {
		if verr == mgo.ErrNotFound {
			return
		}
		err = verr
	}
	return
}

// RemoveByAccess Use the access token to delete the token information
func (ts *TokenStore) RemoveByAccess(access string) (err error) {
	verr := ts.c(ts.tcfg.AccessCName).RemoveId(access)
	if verr != nil {
		if verr == mgo.ErrNotFound {
			return
		}
		err = verr
	}
	return
}

// RemoveByRefresh Use the refresh token to delete the token information
func (ts *TokenStore) RemoveByRefresh(refresh string) (err error) {
	verr := ts.c(ts.tcfg.RefreshCName).RemoveId(refresh)
	if verr != nil {
		if verr == mgo.ErrNotFound {
			return
		}
		err = verr
	}
	return
}

func (ts *TokenStore) getData(basicID string) (ti oauth2.TokenInfo, err error) {
	var bd basicData
	verr := ts.c(ts.tcfg.BasicCName).FindId(basicID).One(&bd)
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
	return
}

func (ts *TokenStore) getBasicID(cname, token string) (basicID string, err error) {
	var td tokenData
	verr := ts.c(cname).FindId(token).One(&td)
	if verr != nil {
		if verr == mgo.ErrNotFound {
			return
		}
		err = verr
		return
	}
	basicID = td.BasicID
	return
}

// GetByCode Use the authorization code for token information data
func (ts *TokenStore) GetByCode(code string) (ti oauth2.TokenInfo, err error) {
	ti, err = ts.getData(code)
	return
}

// GetByAccess Use the access token for token information data
func (ts *TokenStore) GetByAccess(access string) (ti oauth2.TokenInfo, err error) {
	basicID, err := ts.getBasicID(ts.tcfg.AccessCName, access)
	if err != nil && basicID == "" {
		return
	}
	ti, err = ts.getData(basicID)
	return
}

// GetByRefresh Use the refresh token for token information data
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
