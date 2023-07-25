package mongo

import (
	"context"
	"encoding/json"
	"fmt"
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
// func NewTokenStore(cfg *Config, tcfgs ...*TokenConfig) (store *TokenStore) {
func NewTokenStore(cfg *Config) (store *TokenStore) {
	// clientOptions := options.Client().ApplyURI(cfg.URL).SetWriteConcern(writeconcern.New(writeconcern.WMajority()))

	fmt.Println("See url: ", cfg.URL)

	clientOptions := options.Client().ApplyURI(cfg.URL)

	if !cfg.IsReplicaSet {
		clientOptions.SetAuth(options.Credential{
			Username: cfg.Username,
			Password: cfg.Password,
		})
	}

	c, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal("ClientStore failed to connect mongo: ", err)
	} else {
		log.Println("Connection to mongoDB successful")
	}

	err = c.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal("MongoDB ping failed:", err)
	}

	log.Println("Ping db successfull")

	// return NewTokenStoreWithSession(c, cfg, tcfgs...)
	return NewTokenStoreWithSession(c, cfg)
}

// NewTokenStoreWithSession create a token store instance based on mongodb
// func NewTokenStoreWithSession(client *mongo.Client, cfg *Config, tcfgs ...*TokenConfig) (store *TokenStore) {
func NewTokenStoreWithSession(client *mongo.Client, cfg *Config) (store *TokenStore) {
	ts := &TokenStore{
		dbName:       cfg.DB,
		client:       client,
		tcfg:         NewDefaultTokenConfig(),
		isReplicaSet: cfg.IsReplicaSet,
	}

	if !cfg.IsReplicaSet {
		ts.txnHandler = NewTransactionHandler(client, cfg.DB, cfg.Service, ts.tcfg)

		// in case transactions did fail, remove garbage records
		err := ts.txnHandler.cleanupTransactionsData(context.TODO())
		if err != nil {
			// TODO what to do with that err ??
			log.Println("Err cleanupTransactionsData failed: ", err)
		}
	}

	// if len(tcfgs) > 0 {
	// 	ts.tcfg = tcfgs[0]
	// }

	_, err := ts.client.Database(ts.dbName).Collection(ts.tcfg.BasicCName).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys:    bson.D{{"ExpiredAt", 1}},
		Options: options.Index().SetExpireAfterSeconds(1),
	})
	if err != nil {
		log.Fatalln("Error creating index: ", ts.tcfg.BasicCName, " - ", err)
	}

	_, err = ts.client.Database(ts.dbName).Collection(ts.tcfg.AccessCName).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
		Keys:    bson.D{{"ExpiredAt", 1}},
		Options: options.Index().SetExpireAfterSeconds(1),
	})
	if err != nil {
		log.Fatalln("Error creating index: ", ts.tcfg.AccessCName, " - ", err)
	}

	_, err = ts.client.Database(ts.dbName).Collection(ts.tcfg.RefreshCName).Indexes().CreateOne(context.TODO(), mongo.IndexModel{
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
	tcfg         *TokenConfig
	dbName       string
	client       *mongo.Client
	isReplicaSet bool
	txnHandler   *transactionHandler
}

// Close close the mongo session
func (ts *TokenStore) Close() {
	if err := ts.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func (ts *TokenStore) c(name string) *mongo.Collection {
	return ts.client.Database(ts.dbName).Collection(name)
}

// Create create and store the new token information
func (ts *TokenStore) Create(ctx context.Context, info oauth2.TokenInfo) (err error) {
	jv, err := json.Marshal(info)
	if err != nil {
		return
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
	// id := bson.NewObjectId().Hex()
	id := primitive.NewObjectID().Hex()
	fmt.Println("the id: ", id)

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

	// MongoDB is deployed as a replicaSet
	if ts.isReplicaSet {

		// Create collections
		wcMajority := writeconcern.New(writeconcern.WMajority(), writeconcern.WTimeout(2*time.Second))
		wcMajorityCollectionOpts := options.Collection().SetWriteConcern(wcMajority)

		basicColl := ts.client.Database(ts.dbName).Collection(ts.tcfg.BasicCName, wcMajorityCollectionOpts)
		accessColl := ts.client.Database(ts.dbName).Collection(ts.tcfg.AccessCName, wcMajorityCollectionOpts)
		refreshColl := ts.client.Database(ts.dbName).Collection(ts.tcfg.RefreshCName, wcMajorityCollectionOpts)

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
	_, err = ts.c(ts.tcfg.BasicCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: code}})
	if err != nil {
		log.Println("Error RemoveByCode: ", err)
	}
	return
}

// RemoveByAccess use the access token to delete the token information
func (ts *TokenStore) RemoveByAccess(ctx context.Context, access string) (err error) {
	_, err = ts.c(ts.tcfg.AccessCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: access}})
	if err != nil {
		log.Println("Error RemoveByAccess: ", err)
	}
	return
}

// RemoveByRefresh use the refresh token to delete the token information
func (ts *TokenStore) RemoveByRefresh(ctx context.Context, refresh string) (err error) {
	_, err = ts.c(ts.tcfg.RefreshCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: refresh}})
	if err != nil {
		log.Println("Error RemoveByRefresh: ", err)
	}
	return
}

func (ts *TokenStore) getData(basicID string) (ti oauth2.TokenInfo, err error) {

	var bd basicData
	err = ts.c(ts.tcfg.BasicCName).FindOne(context.TODO(), bson.D{{Key: "_id", Value: basicID}}).Decode(&bd)
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
	var td tokenData
	err = ts.c(cname).FindOne(context.TODO(), bson.D{{Key: "_id", Value: token}}).Decode(&td)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return
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
