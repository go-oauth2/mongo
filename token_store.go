package mongo

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

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
	storeConfig  *StoreConfig
}

// NewDefaultTokenConfig create a default token configuration
func NewDefaultTokenConfig(strConfig *StoreConfig) *TokenConfig {
	return &TokenConfig{
		TxnCName:     "oauth2_txn",
		BasicCName:   "oauth2_basic",
		AccessCName:  "oauth2_access",
		RefreshCName: "oauth2_refresh",
		storeConfig:  strConfig,
	}
}

// NewTokenStore create a token store instance based on mongodb
func NewTokenStore(cfg *Config, scfgs ...*StoreConfig) (store *TokenStore) {
	clientOptions := options.Client().ApplyURI(cfg.URL)
	ctx := context.TODO()
	ctxPing := context.TODO()

	if len(scfgs) > 0 && scfgs[0].connectionTimeout > 0 {
		newCtx, cancel := context.WithTimeout(context.Background(), time.Duration(scfgs[0].connectionTimeout)*time.Second)
		ctx = newCtx
		defer cancel()
		clientOptions.SetConnectTimeout(time.Duration(scfgs[0].connectionTimeout) * time.Second)
	}

	if len(scfgs) > 0 && scfgs[0].requestTimeout > 0 {
		newCtx, cancel := context.WithTimeout(context.Background(), time.Duration(scfgs[0].requestTimeout)*time.Second)
		ctxPing = newCtx
		defer cancel()
		clientOptions.SetConnectTimeout(time.Duration(scfgs[0].requestTimeout) * time.Second)
	}

	if !cfg.IsReplicaSet {
		clientOptions.SetAuth(options.Credential{
			Username: cfg.Username,
			Password: cfg.Password,
		})
	}

	c, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal("ClientStore failed to connect mongo: ", err)
	} else {
		log.Println("Connection to mongoDB successful")
	}

	err = c.Ping(ctxPing, nil)
	if err != nil {
		log.Fatal("MongoDB ping failed:", err)
	}

	log.Println("Ping db successfull")

	return NewTokenStoreWithSession(c, cfg, scfgs...)
}

// NewTokenStoreWithSession create a token store instance based on mongodb
func NewTokenStoreWithSession(client *mongo.Client, cfg *Config, scfgs ...*StoreConfig) (store *TokenStore) {
	strCfgs := NewDefaultStoreConfig(cfg.DB, cfg.Service, cfg.IsReplicaSet)

	ts := &TokenStore{
		client: client,
		tcfg:   NewDefaultTokenConfig(strCfgs),
	}

	if len(scfgs) > 0 {
		if scfgs[0].connectionTimeout > 0 {
			ts.tcfg.storeConfig.connectionTimeout = scfgs[0].connectionTimeout
		}
		if scfgs[0].requestTimeout > 0 {
			ts.tcfg.storeConfig.requestTimeout = scfgs[0].requestTimeout
		}
	}

	if !ts.tcfg.storeConfig.isReplicaSet {
		ts.txnHandler = NewTransactionHandler(client, ts.tcfg)

		// in case transactions did fail, remove garbage records
		err := ts.txnHandler.tw.cleanupTransactionsData(context.TODO(), cfg.Service)
		if err != nil {
			// TODO what to do with that err ??
			log.Println("Err cleanupTransactionsData failed: ", err)
		}
	}

	_, err := ts.client.Database(ts.tcfg.storeConfig.db).Collection(ts.tcfg.BasicCName).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys:    bson.D{{"ExpiredAt", 1}},
		Options: options.Index().SetExpireAfterSeconds(1),
	})
	if err != nil {
		log.Fatalln("Error creating index: ", ts.tcfg.BasicCName, " - ", err)
	}

	_, err = ts.client.Database(ts.tcfg.storeConfig.db).Collection(ts.tcfg.AccessCName).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys:    bson.D{{"ExpiredAt", 1}},
		Options: options.Index().SetExpireAfterSeconds(1),
	})
	if err != nil {
		log.Fatalln("Error creating index: ", ts.tcfg.AccessCName, " - ", err)
	}

	_, err = ts.client.Database(ts.tcfg.storeConfig.db).Collection(ts.tcfg.RefreshCName).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys:    bson.D{{"ExpiredAt", 1}},
		Options: options.Index().SetExpireAfterSeconds(1),
	})
	if err != nil {
		log.Fatalln("Error creating index: ", ts.tcfg.RefreshCName, " - ", err)
	}

	store = ts
	return
}

// TokenStore MongoDB storage for OAuth 2.0
type TokenStore struct {
	tcfg       *TokenConfig
	client     *mongo.Client
	txnHandler *transactionHandler
}

// Close close the mongo session
func (ts *TokenStore) Close() {
	if err := ts.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func (ts *TokenStore) c(name string) *mongo.Collection {
	return ts.client.Database(ts.tcfg.storeConfig.db).Collection(name)
}

// Create create and store the new token information
func (ts *TokenStore) Create(ctx context.Context, info oauth2.TokenInfo) (err error) {
	jv, err := json.Marshal(info)
	if err != nil {
		return
	}

	ctxReq, cancel := ts.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	if code := info.GetCode(); code != "" {
		// Create the basicData document
		basicData := basicData{
			ID:        code,
			Data:      jv,
			ExpiredAt: info.GetCodeCreateAt().Add(info.GetCodeExpiresIn()),
		}

		_, err = ts.c(ts.tcfg.BasicCName).InsertOne(ctx, basicData)
		if err != nil {
			log.Println("Error CreateToken with code: ", err)
		}

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

	id := primitive.NewObjectID().Hex()

	// Create the basicData document
	basicData := basicData{
		ID:        id,
		Data:      jv,
		ExpiredAt: rexp,
	}

	// Create the tokenData document for access
	accessData := tokenData{
		ID:        info.GetAccess(),
		BasicID:   id,
		ExpiredAt: aexp,
	}

	// if context is defined, increase it for the transaction
	ctxTxn, cancel := ts.tcfg.storeConfig.setTransactionCreateContext()
	defer cancel()
	if ctxTxn != nil {
		ctx = ctxReq
	}

	// MongoDB is deployed as a replicaSet
	if ts.tcfg.storeConfig.isReplicaSet {

		// Create collections
		wcMajority := writeconcern.New(writeconcern.WMajority(), writeconcern.WTimeout(2*time.Second))
		wcMajorityCollectionOpts := options.Collection().SetWriteConcern(wcMajority)

		basicColl := ts.client.Database(ts.tcfg.storeConfig.db).Collection(ts.tcfg.BasicCName, wcMajorityCollectionOpts)
		accessColl := ts.client.Database(ts.tcfg.storeConfig.db).Collection(ts.tcfg.AccessCName, wcMajorityCollectionOpts)
		refreshColl := ts.client.Database(ts.tcfg.storeConfig.db).Collection(ts.tcfg.RefreshCName, wcMajorityCollectionOpts)

		callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
			if _, err := basicColl.InsertOne(sessCtx, basicData); err != nil {
				return nil, err
			}
			if _, err := accessColl.InsertOne(sessCtx, accessData); err != nil {
				return nil, err
			}

			refresh := info.GetRefresh()
			if refresh != "" {
				refreshData := tokenData{
					ID:        refresh,
					BasicID:   id,
					ExpiredAt: rexp,
				}
				if _, err := refreshColl.InsertOne(sessCtx, refreshData); err != nil {
					return nil, err
				}

			}
			return nil, nil
		}

		session, err := ts.client.StartSession()
		if err != nil {
			return err
		}
		defer session.EndSession(ctx)
		result, err := session.WithTransaction(ctx, callback)
		if err != nil {
			return err
		}
		log.Printf("result: %v\n", result)

	} else {
		// MongoDB is deployed as a single instance
		return ts.txnHandler.runTransactionCreate(ctx, info, basicData, accessData, id, rexp)

	}
	return
}

// RemoveByCode use the authorization code to delete the token information
func (ts *TokenStore) RemoveByCode(ctx context.Context, code string) (err error) {
	ctxReq, cancel := ts.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	_, err = ts.c(ts.tcfg.BasicCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: code}})
	if err != nil {
		log.Println("Error RemoveByCode: ", err)
	}
	return
}

// RemoveByAccess use the access token to delete the token information
func (ts *TokenStore) RemoveByAccess(ctx context.Context, access string) (err error) {
	ctxReq, cancel := ts.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	_, err = ts.c(ts.tcfg.AccessCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: access}})
	if err != nil {
		log.Println("Error RemoveByAccess: ", err)
	}
	return
}

// RemoveByRefresh use the refresh token to delete the token information
func (ts *TokenStore) RemoveByRefresh(ctx context.Context, refresh string) (err error) {
	ctxReq, cancel := ts.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	_, err = ts.c(ts.tcfg.RefreshCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: refresh}})
	if err != nil {
		log.Println("Error RemoveByRefresh: ", err)
	}
	return
}

func (ts *TokenStore) getData(basicID string) (ti oauth2.TokenInfo, err error) {
	ctx := context.Background()
	ctxReq, cancel := ts.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	var bd basicData
	err = ts.c(ts.tcfg.BasicCName).FindOne(ctx, bson.D{{Key: "_id", Value: basicID}}).Decode(&bd)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
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
	ctx := context.Background()
	ctxReq, cancel := ts.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	var td tokenData
	err = ts.c(cname).FindOne(ctx, bson.D{{Key: "_id", Value: token}}).Decode(&td)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil
		}
		return
	}
	basicID = td.BasicID
	return
}

// GetByCode use the authorization code for token information data
func (ts *TokenStore) GetByCode(ctx context.Context, code string) (ti oauth2.TokenInfo, err error) {
	ti, err = ts.getData(code)
	return
}

// GetByAccess use the access token for token information data
func (ts *TokenStore) GetByAccess(ctx context.Context, access string) (ti oauth2.TokenInfo, err error) {
	basicID, err := ts.getBasicID(ts.tcfg.AccessCName, access)
	if err != nil && basicID == "" {
		return
	}
	ti, err = ts.getData(basicID)
	return
}

// GetByRefresh use the refresh token for token information data
func (ts *TokenStore) GetByRefresh(ctx context.Context, refresh string) (ti oauth2.TokenInfo, err error) {
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
