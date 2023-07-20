package mongo

import (
	"context"
	"errors"
	"log"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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

	return NewClientStoreWithSession(c, cfg.DB, ccfgs...)
}

// NewClientStoreWithSession create a client store instance based on mongodb
func NewClientStoreWithSession(client *mongo.Client, dbName string, ccfgs ...*ClientConfig) *ClientStore {
	cs := &ClientStore{
		dbName: dbName,
		client: client,
		ccfg:   NewDefaultClientConfig(),
	}
	if len(ccfgs) > 0 {
		cs.ccfg = ccfgs[0]
	}

	return cs
}

// ClientStore MongoDB storage for OAuth 2.0
type ClientStore struct {
	ccfg   *ClientConfig
	dbName string
	client *mongo.Client
}

// Close close the mongo session
func (cs *ClientStore) Close() {
	if err := cs.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func (cs *ClientStore) c(name string) *mongo.Collection {
	return cs.client.Database(cs.dbName).Collection(name)
}

// Create create client information
func (cs *ClientStore) Create(info oauth2.ClientInfo) (err error) {
	entity := &client{
		ID:     info.GetID(),
		Secret: info.GetSecret(),
		Domain: info.GetDomain(),
		UserID: info.GetUserID(),
	}

	collection := cs.c(cs.ccfg.ClientsCName)

	filter := bson.M{"_id": entity.ID}
	existingCount, err := collection.CountDocuments(context.Background(), filter)
	if err != nil {
		log.Fatal(err)
	}

	if existingCount == 0 {
		if _, err := collection.InsertOne(context.Background(), entity); err != nil {
			log.Fatal(err)
		}
	}

	return
}

// GetByID according to the ID for the client information
func (cs *ClientStore) GetByID(ctx context.Context, id string) (info oauth2.ClientInfo, err error) {
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
	filter := bson.M{"_id": id}
	_, err = cs.c(cs.ccfg.ClientsCName).DeleteOne(context.Background(), filter)
	return err
}

type client struct {
	ID     string `bson:"_id"`
	Secret string `bson:"secret"`
	Domain string `bson:"domain"`
	UserID string `bson:"userid"`
}
