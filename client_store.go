package mongo

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TODO
// add retry mecanism on all requests
// if retry fail ping the db
// if ping fail crash the service
// that should be optional as a service's restart mecanism should be implemented

// StoreConfig hold configs common to all Configs(ClientConfig, TokenConfig)
type StoreConfig struct {
	db                string
	service           string
	connectionTimeout int
	requestTimeout    int
	isReplicaSet      bool
}

func NewStoreConfig(ctout, rtout int) *StoreConfig {
	return &StoreConfig{
		connectionTimeout: ctout,
		requestTimeout:    rtout,
	}
}

func NewDefaultStoreConfig(db, service string, isReplicasSet bool) *StoreConfig {
	return &StoreConfig{
		db:                db,
		service:           service,
		connectionTimeout: 0,
		requestTimeout:    0,
		isReplicaSet:      isReplicasSet,
	}
}

// setRequestContext set a WithTimeout or Background context
func (sc *StoreConfig) setRequestContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if sc.requestTimeout > 0 {
		log.Println("Request timeout: ", sc.requestTimeout)
		timeout := time.Duration(sc.requestTimeout) * time.Second
		return context.WithTimeout(ctx, timeout)
	}
	return nil, func() {}
}

// setTransactionCreateContext is specific to the transaction(if not a replicaSet)
func (sc *StoreConfig) setTransactionCreateContext() (context.Context, context.CancelFunc) {
	ctx := context.Background()
	if sc.requestTimeout > 0 {
		// at max TransactionCreate run 9 requests
		timeout := time.Duration(sc.requestTimeout*9) * time.Second
		return context.WithTimeout(ctx, timeout)
	}
	return nil, func() {}
}

// ClientConfig client configuration parameters
type ClientConfig struct {
	// store clients data collection name(The default is oauth2_clients)
	ClientsCName string
	storeConfig  *StoreConfig
}

// NewDefaultClientConfig create a default client configuration
func NewDefaultClientConfig(strCfgs *StoreConfig) *ClientConfig {
	return &ClientConfig{
		ClientsCName: "oauth2_clients",
		storeConfig:  strCfgs,
	}
}

// NewClientStore create a client store instance based on mongodb
func NewClientStore(cfg *Config, scfgs ...*StoreConfig) *ClientStore {
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

	return NewClientStoreWithSession(c, cfg, scfgs...)
}

// NewClientStoreWithSession create a client store instance based on mongodb
func NewClientStoreWithSession(client *mongo.Client, cfg *Config, scfgs ...*StoreConfig) *ClientStore {
	strCfgs := NewDefaultStoreConfig(cfg.DB, cfg.Service, cfg.IsReplicaSet)

	cs := &ClientStore{
		client: client,
		ccfg:   NewDefaultClientConfig(strCfgs),
	}

	if len(scfgs) > 0 {
		if scfgs[0].connectionTimeout > 0 {
			cs.ccfg.storeConfig.connectionTimeout = scfgs[0].connectionTimeout
		}
		if scfgs[0].requestTimeout > 0 {
			cs.ccfg.storeConfig.requestTimeout = scfgs[0].requestTimeout
		}
	}

	return cs
}

// ClientStore MongoDB storage for OAuth 2.0
type ClientStore struct {
	ccfg   *ClientConfig
	client *mongo.Client
}

// Close close the mongo session
func (cs *ClientStore) Close() {
	if err := cs.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func (cs *ClientStore) c(name string) *mongo.Collection {
	return cs.client.Database(cs.ccfg.storeConfig.db).Collection(name)
}

// Create create client information
func (cs *ClientStore) Create(info oauth2.ClientInfo) (err error) {
	ctx := context.Background()

	ctxReq, cancel := cs.ccfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	entity := &client{
		ID:     info.GetID(),
		Secret: info.GetSecret(),
		Domain: info.GetDomain(),
		UserID: info.GetUserID(),
	}

	collection := cs.c(cs.ccfg.ClientsCName)

	_, err = collection.InsertOne(ctx, entity)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			log.Fatal(err)
		} else {
			return nil
		}
	}

	return
}

// GetByID according to the ID for the client information
func (cs *ClientStore) GetByID(ctx context.Context, id string) (info oauth2.ClientInfo, err error) {
	ctxReq, cancel := cs.ccfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	filter := bson.M{"_id": id}
	result := cs.c(cs.ccfg.ClientsCName).FindOne(ctx, filter)
	if err := result.Err(); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, err
		}
		return nil, errors.New("Internal server error, no client found for this ID")

	}

	entity := &client{}
	if err := result.Decode(entity); err != nil {
		log.Println(err)
	}

	info = &models.Client{
		ID:     entity.ID,
		Secret: entity.Secret,
		Domain: entity.Domain,
		UserID: entity.UserID,
	}

	return
}

// RemoveByID use the client id to delete the client information
func (cs *ClientStore) RemoveByID(id string) (err error) {
	ctx := context.Background()

	ctxReq, cancel := cs.ccfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	filter := bson.M{"_id": id}
	_, err = cs.c(cs.ccfg.ClientsCName).DeleteOne(ctx, filter)
	return err
}

type client struct {
	ID     string `bson:"_id"`
	Secret string `bson:"secret"`
	Domain string `bson:"domain"`
	UserID string `bson:"userid"`
}
