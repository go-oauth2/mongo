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
	tcfg *TokenConfig
	tw   TransactionWorker
}

func NewTransactionHandler(client *mongo.Client, tcfg *TokenConfig) *transactionHandler {

	return &transactionHandler{
		tcfg: tcfg,
		tw:   NewTransactionWorker(tcfg, client),
	}
}

func (th *transactionHandler) runTransactionCreate(ctx context.Context, info oauth2.TokenInfo, basicData basicData, accessData tokenData, id string, rexp time.Time) (err error) {

	ctxReq, cancel := th.tcfg.storeConfig.setRequestContext()
	defer cancel()
	if ctxReq != nil {
		ctx = ctxReq
	}

	// create id transaction idTXN
	txnID := primitive.NewObjectID().Hex()

	// T1
	basicTxnData := transactionData{
		ID:         basicData.ID,
		TxnID:      txnID,
		Collection: th.tcfg.BasicCName,
		Service:    th.tcfg.storeConfig.service,
		CreatedAt:  time.Now(),
	}
	err = th.tw.insertBasicTransactionData(ctx, basicTxnData)
	if err != nil {
		log.Println("T1: Failed add basicData to TxnCName: ", err)
		return
	} else {
		err = th.tw.insertBasicData(ctx, basicData)
		if err != nil {
			log.Println("T1: Failed add basicData to BasicCName: ", err)
			err = th.tw.removeTransactionData(ctx, basicData.ID)
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
		Service:    th.tcfg.storeConfig.service,
		CreatedAt:  time.Now(),
	}
	err = th.tw.insertTokenTransactionData(ctx, acccessTxnData)
	if err != nil {
		log.Println("T2: Failed insert accessData to TxnCName: ", err)
		err = th.tw.removeBasicData(ctx, basicData.ID)
		if err != nil {
			// basicData will be remove when service restart
			log.Println("T2: Failed remove basicData from BasicCName: ", err)
		} else {
			// basicData has been removed, then removed it in TxnCName
			err = th.tw.removeTransactionData(ctx, basicData.ID)
			if err != nil {
				log.Println("T2: Failed remove basicData from TxnCName: ", err)
			}
		}
		return
	} else {
		err = th.tw.insertTokenData(ctx, accessData, th.tcfg.AccessCName)
		if err != nil {
			log.Println("T2: Failed insert accessData to AccessCName: ", err)
			err = th.tw.removeBasicData(ctx, basicData.ID)
			if err != nil {
				// basicData from will be remove when service restart
				log.Println("T2: Failed remove basicData from BasicCName: ", err)
			} else {
				err = th.tw.removeTransactionData(ctx, basicData.ID)
				if err != nil {
					// basicTxnData from will be remove when service restart
					log.Println("T2: Failed remove basicData from TxnCName: ", err)
				}
			}
			err = th.tw.removeTransactionData(ctx, accessData.ID)
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
		err = th.tw.insertTokenData(ctx, refreshData, th.tcfg.RefreshCName)
		if err != nil {
			err = th.tw.removeBasicData(ctx, basicData.ID)
			if err != nil {
				// basicData will be remove when service restart
				log.Println("T3: Failed remove basicData from BasicCName: ", err)
			} else {
				err = th.tw.removeTransactionData(ctx, basicData.ID)
				if err != nil {
					// basicData will be remove when service restart
					log.Println("T3: Failed remove basicData from TxnCName: ", err)
				}
			}

			err = th.tw.removeTokenData(ctx, accessData.ID, th.tcfg.AccessCName)
			if err != nil {
				// accessData will be remove when service restart
				log.Println("T3: Failed remove accessData from AccessCName: ", err)
			} else {
				err = th.tw.removeTransactionData(ctx, accessData.ID)
				if err != nil {
					log.Println("T3: Failed remove txnData from TxnCName: ", err)
				}
			}
			return
		}
	}

	// case all is fine, finally delete all txnDatas
	err = th.tw.removeTransactionData(ctx, basicData.ID)
	if err != nil {
		// basicTxnData will be remove when service restart
		log.Println("EndTnx cleanup: Failed remove basicData from TxnCName: ", err)
	}

	err = th.tw.removeTransactionData(ctx, accessData.ID)
	if err != nil {
		// accessTxnData will be remove when service restart
		log.Println("EndTxn cleanup: Failed remove txnData from TxnCName: ", err)
	}

	return nil

	// TODO set a retry on all methods
	// if retry fail ping the db
	// if ping fail crash the server for it to restart

}

type TransactionWorker interface {
	insertBasicData(ctx context.Context, basicData basicData) error
	removeBasicData(ctx context.Context, basicDataID string) error
	insertBasicTransactionData(ctx context.Context, txnData transactionData) error
	insertTokenData(ctx context.Context, tokenData tokenData, collectionName string) error
	removeTokenData(ctx context.Context, tokenDataID, collectionName string) error
	insertTokenTransactionData(ctx context.Context, txnData transactionData) error
	removeTransactionData(ctx context.Context, tokenDataID string) error
	cleanupTransactionsData(ctx context.Context, service string) error
}

type transactionWorker struct {
	tc     *TokenConfig
	client *mongo.Client
}

func NewTransactionWorker(t *TokenConfig, cl *mongo.Client) *transactionWorker {
	return &transactionWorker{
		tc:     t,
		client: cl,
	}
}

func (tw *transactionWorker) getCollection(collName string) *mongo.Collection {
	return tw.client.Database(tw.tc.storeConfig.db).Collection(collName)
}

func (tw *transactionWorker) insertBasicData(ctx context.Context, basicData basicData) error {
	log.Println("insertBasicData in BasicCName id: ", basicData.ID)
	_, err := tw.getCollection(tw.tc.BasicCName).InsertOne(ctx, basicData)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			log.Println("Err insertBasicData into BasicCname: ", err)
		} else {
			// in case of retry, the tuple may have already been inserted
			// we like to carry on
			log.Println("Err insertBasicData duplicated _id: ", err)
			return nil
		}
	}
	return err
}

func (tw *transactionWorker) removeBasicData(ctx context.Context, basicDataID string) error {
	log.Println("removeBasicData from BasicCName id: ", basicDataID)
	_, err := tw.getCollection(tw.tc.BasicCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: basicDataID}})
	if err != nil {
		log.Println("Err removeBasicData from BasicCname: ", err)
		return err
	}
	return err
}

func (tw *transactionWorker) insertBasicTransactionData(ctx context.Context, txnData transactionData) error {
	log.Println("insertBasicTransactionData in TxnCName id: ", txnData.ID)
	_, err := tw.getCollection(tw.tc.TxnCName).InsertOne(ctx, txnData)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			log.Println("Err insertBasicTransactionData to basicCname: ", err)
		} else {
			log.Println("Err insertBasicTransactionData duplicated _id: ", err)
			return nil
		}
	}
	return err
}

// InsertTokenData insert accessData and refreshData
func (tw *transactionWorker) insertTokenData(ctx context.Context, tokenData tokenData, collectionName string) error {
	log.Printf("insertTokenData in %v id: %v", collectionName, tokenData.ID)
	_, err := tw.getCollection(collectionName).InsertOne(ctx, tokenData)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			log.Printf("Err insertTokenData into %v: %v", collectionName, err)
		} else {
			log.Println("Err insertTokenData duplicated _id: ", err)
			return nil
		}
	}
	return err
}

func (tw *transactionWorker) removeTokenData(ctx context.Context, tokenDataID, collectionName string) error {
	log.Printf("removeTokenData from %v id: %v", collectionName, tokenDataID)
	_, err := tw.getCollection(collectionName).DeleteOne(ctx, bson.D{{Key: "_id", Value: tokenDataID}})
	if err != nil {
		log.Printf("Err removeTransactionData from %v: %v", collectionName, err)
		return err
	}
	return err
}

// InsertTokenData insert accessData and refreshData
func (tw *transactionWorker) insertTokenTransactionData(ctx context.Context, txnData transactionData) error {
	log.Println("insertTokenTransactionData in TxnCName id: ", txnData.ID)
	_, err := tw.getCollection(tw.tc.TxnCName).InsertOne(ctx, txnData)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			log.Println("Err insertTokenTransactionData into TxnCname: ", err)
		} else {
			log.Println("Err insertTokenTransactionData duplicated _id: ", err)
			return nil
		}
	}
	return err
}

func (tw *transactionWorker) removeTransactionData(ctx context.Context, tokenDataID string) error {
	log.Println("removeTransactionData from TxnCName id: ", tokenDataID)
	_, err := tw.getCollection(tw.tc.TxnCName).DeleteOne(ctx, bson.D{{Key: "_id", Value: tokenDataID}})
	if err != nil {
		log.Println("Err removeTransactionData from TxnCName: ", err)
		return err
	}
	return err
}

func (tw *transactionWorker) cleanupTransactionsData(ctx context.Context, service string) (err error) {
	filter := bson.M{"Service": service}
	cursor, err := tw.getCollection(tw.tc.TxnCName).Find(ctx, filter)
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
			if txn.Collection == tw.tc.BasicCName {
				err = tw.removeBasicData(ctx, txn.ID)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeBasicData id: ", txn.ID)
					continue

				}

				err = tw.removeTransactionData(ctx, txn.ID)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeTransactionData(basic) id: ", txn.ID)
				}

			} else if txn.Collection == tw.tc.AccessCName {
				err = tw.removeTokenData(ctx, txn.ID, txn.Collection)
				if err != nil {
					log.Println("Err cleanupTransactionsData removeAccessData id: ", txn.ID)
					continue
				}

				err = tw.removeTransactionData(ctx, txn.ID)
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
