package mongo

import (
	"context"
	"github.com/go-oauth2/oauth2/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type tokenInfo struct {
	tokenID    string // tokenID
	collection string
}

type transactionData struct {
	ID         string    `bson:"_id"`
	TxnID      string    `bson:"TxnID"`
	Collection string    `bson:"Collection"`
	Service    string    `bson:"Service"`
	CreatedAt  time.Time `bson:"CreatedAt"`
}

type transactionHandler struct {
	client      *mongo.Client
	tcfg        *TokenConfig
	dbName      string
	serviceName string
}

func NewTransactionHandler(client *mongo.Client, dbName, serviceName string, tcfg *TokenConfig) *transactionHandler {
	return &transactionHandler{
		client:      client,
		tcfg:        tcfg,
		dbName:      dbName,
		serviceName: serviceName,
	}
}

func (th *transactionHandler) runTransactionCreate(ctx context.Context, info oauth2.TokenInfo, basicData basicData, accessData tokenData, id string, rexp time.Time) (err error) {

	// TODO the overall transaction must have a TIMEOUT
	// create id transaction idTXN
	txnID := primitive.NewObjectID().Hex()

	// T1
	basicTxnData := transactionData{
		ID:         basicData.ID,
		TxnID:      txnID,
		Collection: th.tcfg.BasicCName,
		Service:    th.serviceName,
		CreatedAt:  time.Now(),
	}
	err = th.insertBasicTransactionData(ctx, basicTxnData)
	if err != nil {
		log.Println("T1: Failed add basicData to TxnCName: ", err)
		return
	} else {
		err = th.insertBasicData(ctx, basicData)
		if err != nil {
			log.Println("T1: Failed add basicData to BasicCName: ", err)
			err = th.removeTransactionData(ctx, basicData.ID)
			if err != nil {
				// basicTxnData from will be remove when service restart
				log.Println("T2: Failed remove basicData from TxnCName: ", err)
			}
			return
		}
	}

	// T2
	acccessTxnData := transactionData{
		ID:         info.GetAccess(),
		TxnID:      txnID,
		Collection: th.tcfg.AccessCName,
		Service:    th.serviceName,
		CreatedAt:  time.Now(),
	}
	err = th.insertTokenTransactionData(ctx, acccessTxnData)
	if err != nil {
		log.Println("T2: Failed insert accessData to TxnCName: ", err)
		err = th.removeBasicData(ctx, basicData.ID)
		if err != nil {
			// basicData will be remove when service restart
			log.Println("T2: Failed remove basicData from BasicCName: ", err)
		} else {
			// basicData has been removed, then removed it in TxnCName
			err = th.removeTransactionData(ctx, basicData.ID)
			if err != nil {
				log.Println("T2: Failed remove basicData from TxnCName: ", err)
			}
		}
		return
	} else {
		err = th.insertTokenData(ctx, accessData, th.tcfg.AccessCName)
		if err != nil {
			err = th.removeBasicData(ctx, basicData.ID)
			if err != nil {
				// basicData from will be remove when service restart
				log.Println("T2: Failed remove basicData from BasicCName: ", err)
			} else {
				err = th.removeTransactionData(ctx, basicData.ID)
				if err != nil {
					// basicTxnData from will be remove when service restart
					log.Println("T2: Failed remove basicData from TxnCName: ", err)
				}
			}
			err = th.removeTransactionData(ctx, accessData.ID)
			if err != nil {
				log.Println("T2: Failed remove txnData from TxnCName: ", err)
			}
			return
		}
	}

	// T3
	refresh := info.GetRefresh()
	if refresh != "" {
		refreshData := tokenData{
			ID:        refresh,
			BasicID:   id,
			ExpiredAt: rexp,
		}
		err = th.insertTokenData(ctx, refreshData, th.tcfg.RefreshCName)
		if err != nil {
			err = th.removeBasicData(ctx, basicData.ID)
			if err != nil {
				// basicData will be remove when service restart
				log.Println("T3: Failed remove basicData from BasicCName: ", err)
			} else {
				err = th.removeTransactionData(ctx, basicData.ID)
				if err != nil {
					// basicData will be remove when service restart
					log.Println("T3: Failed remove basicData from TxnCName: ", err)
				}
			}

			err = th.removeTokenData(ctx, accessData.ID, th.tcfg.AccessCName)
			if err != nil {
				// accessData will be remove when service restart
				log.Println("T3: Failed remove accessData from AccessCName: ", err)
			} else {
				err = th.removeTransactionData(ctx, accessData.ID)
				if err != nil {
					log.Println("T3: Failed remove txnData from TxnCName: ", err)
				}
			}
			return
		}
	}

	// case all is fine, finally delete all txnDatas
	err = th.removeTransactionData(ctx, basicData.ID)
	if err != nil {
		// basicTxnData will be remove when service restart
		log.Println("EndTnx cleanup: Failed remove basicData from TxnCName: ", err)
	}

	err = th.removeTransactionData(ctx, accessData.ID)
	if err != nil {
		// accessTxnData will be remove when service restart
		log.Println("EndTxn cleanup: Failed remove txnData from TxnCName: ", err)
	}

	return nil

	// TODO set a retry on all methods
	// if retry fail ping the db
	// if ping fail crash the server for it to restart

}

func (th *transactionHandler) getCollection(collName string) *mongo.Collection {
	return th.client.Database(th.dbName).Collection(collName)
}

func (th *transactionHandler) insertBasicData(ctx context.Context, basicData basicData) error {
	log.Println("insertBasicData in BasicCName id: ", basicData.ID)
	_, err := th.getCollection(th.tcfg.BasicCName).InsertOne(ctx, basicData)
	if err != nil {
		log.Println("Err insertBasicData into BasicCname: ", err)
		return err
	}
	return err
}

func (th *transactionHandler) removeBasicData(ctx context.Context, basicDataID string) error {
	log.Println("removeBasicData from BasicCName id: ", basicDataID)
	_, err := th.getCollection(th.tcfg.BasicCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: basicDataID}})
	if err != nil {
		log.Println("Err removeBasicData from BasicCname: ", err)
		return err
	}
	return err
}

func (th *transactionHandler) insertBasicTransactionData(ctx context.Context, txnData transactionData) error {
	log.Println("insertBasicTransactionData in TxnCName id: ", txnData.ID)
	_, err := th.getCollection(th.tcfg.TxnCName).InsertOne(ctx, txnData)
	if err != nil {
		log.Println("Err insertBasicTransactionData to basicCname: ", err)
		return err
	}
	return err
}

// InsertTokenData insert accessData and refreshData
func (th *transactionHandler) insertTokenData(ctx context.Context, tokenData tokenData, collectionName string) error {
	log.Printf("insertTokenData in %v id: %v", collectionName, tokenData.ID)
	_, err := th.getCollection(collectionName).InsertOne(ctx, tokenData)
	if err != nil {
		log.Printf("Err insertTokenData into %v: %v", collectionName, err)
		return err
	}
	return err
}

func (th *transactionHandler) removeTokenData(ctx context.Context, tokenDataID, collectionName string) error {
	log.Printf("removeTokenData from %v id: %v", collectionName, tokenDataID)
	_, err := th.getCollection(collectionName).DeleteOne(ctx, bson.D{{Key: "_id", Value: tokenDataID}})
	if err != nil {
		log.Printf("Err removeTransactionData from %v: %v", collectionName, err)
		return err
	}
	return err
}

// InsertTokenData insert accessData and refreshData
func (th *transactionHandler) insertTokenTransactionData(ctx context.Context, txnData transactionData) error {
	log.Println("insertTokenTransactionData in TxnCName id: ", txnData.ID)
	_, err := th.getCollection(th.tcfg.TxnCName).InsertOne(ctx, txnData)
	if err != nil {
		log.Println("Err insertTokenTransactionData into TxnCname: ", err)
	}
	return err
}

func (th *transactionHandler) removeTransactionData(ctx context.Context, tokenDataID string) error {
	log.Println("removeTransactionData from TxnCName id: ", tokenDataID)
	_, err := th.getCollection(th.tcfg.TxnCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: tokenDataID}})
	if err != nil {
		log.Println("Err removeTransactionData from TxnCName: ", err)
		return err
	}
	return err
}

func (th *transactionHandler) cleanupTransactionsData(ctx context.Context) (err error) {
	filter := bson.M{}
	cursor, err := th.getCollection(th.tcfg.TxnCName).Find(ctx, filter)
	if err != nil {
		log.Println("Err cleanupTransactionsData findAll TxnCName: ", err)
		return
	}
	// Iterate over the cursor to get all documents
	var txnsData []transactionData
	if err := cursor.All(ctx, &txnsData); err != nil {
		log.Println("Err removeTransactionsData when iterate cursor: ", err)
	}

	if len(txnsData) > 0 {
		for _, txn := range txnsData {
			if txn.Collection == th.tcfg.BasicCName {
				err = th.removeBasicData(ctx, txn.ID)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeBasicData id: ", txn.ID)
					continue

				}

				err = th.removeTransactionData(ctx, txn.ID)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeTransactionData(basic) id: ", txn.ID)
				}

			} else if txn.Collection == th.tcfg.AccessCName {
				err = th.removeTokenData(ctx, txn.ID, txn.Collection)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeAccessData id: ", txn.ID)
					continue
				}

				err = th.removeTransactionData(ctx, txn.ID)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeTransactionData(access) id: ", txn.ID)
				}
			} else {
				log.Println("Err cleanupTransactionsData unfound collection: ", txn.Collection)
			}
		}
	}

	return
}
