package db_service

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type DbService[DocType interface{}] interface {
    CreateDocument(ctx context.Context, id string, document *DocType) error
    FindDocument(ctx context.Context, id string) (*DocType, error)
    ListDocuments(ctx context.Context) ([]DocType, error)               // ← new
    UpdateDocument(ctx context.Context, id string, document *DocType) error
    DeleteDocument(ctx context.Context, id string) error
    Disconnect(ctx context.Context) error
	FindDocumentsByField(ctx context.Context, fieldName string, value any) ([]*DocType, error)
}


var ErrNotFound = fmt.Errorf("document not found")
var ErrConflict = fmt.Errorf("conflict: document already exists")

type MongoServiceConfig struct {
	ServerHost string
	ServerPort int
	UserName   string
	Password   string
	DbName     string
	Collection string
	Timeout    time.Duration
}

type mongoSvc[DocType interface{}] struct {
	MongoServiceConfig
	client     atomic.Pointer[mongo.Client]
	clientLock sync.Mutex
}

func NewMongoService[DocType interface{}](config MongoServiceConfig) DbService[DocType] {
	enviro := func(name string, defaultValue string) string {
		if value, ok := os.LookupEnv(name); ok {
			return value
		}
		return defaultValue
	}

	svc := &mongoSvc[DocType]{}
	svc.MongoServiceConfig = config

	if svc.ServerHost == "" {
		svc.ServerHost = enviro("AMBULANCE_API_MONGODB_HOST", "localhost")
	}

	if svc.ServerPort == 0 {
		port := enviro("AMBULANCE_API_MONGODB_PORT", "27017")
		if port, err := strconv.Atoi(port); err == nil {
			svc.ServerPort = port
		} else {
			log.Printf("Invalid port value: %v", port)
			svc.ServerPort = 27017
		}
	}

	if svc.UserName == "" {
		svc.UserName = enviro("AMBULANCE_API_MONGODB_USERNAME", "")
	}

	if svc.Password == "" {
		svc.Password = enviro("AMBULANCE_API_MONGODB_PASSWORD", "")
	}

	if svc.DbName == "" {
		svc.DbName = enviro("AMBULANCE_API_MONGODB_DATABASE", "xdudakm-wac-ambulance-wl")
	}

	if svc.Collection == "" {
		svc.Collection = enviro("AMBULANCE_API_MONGODB_COLLECTION", "ambulance")
	}

	if svc.Timeout == 0 {
		seconds := enviro("AMBULANCE_API_MONGODB_TIMEOUT_SECONDS", "10")
		if seconds, err := strconv.Atoi(seconds); err == nil {
			svc.Timeout = time.Duration(seconds) * time.Second
		} else {
			log.Printf("Invalid timeout value: %v", seconds)
			svc.Timeout = 10 * time.Second
		}
	}

	log.Printf(
		"MongoDB config: //%v@%v:%v/%v/%v",
		svc.UserName,
		svc.ServerHost,
		svc.ServerPort,
		svc.DbName,
		svc.Collection,
	)
	return svc
}

func (m *mongoSvc[DocType]) connect(ctx context.Context) (*mongo.Client, error) {
	client := m.client.Load()
	if client != nil {
		return client, nil
	}

	m.clientLock.Lock()
	defer m.clientLock.Unlock()

	client = m.client.Load()
	if client != nil {
		return client, nil
	}

	ctx, contextCancel := context.WithTimeout(ctx, m.Timeout)
	defer contextCancel()

	var uri = fmt.Sprintf("mongodb://%v:%v", m.ServerHost, m.ServerPort)
	log.Printf("Using URI: " + uri)

	if len(m.UserName) != 0 {
		uri = fmt.Sprintf("mongodb://%v:%v@%v:%v", m.UserName, m.Password, m.ServerHost, m.ServerPort)
	}

	if client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri).SetConnectTimeout(10*time.Second)); err != nil {
		return nil, err
	} else {
		m.client.Store(client)
		return client, nil
	}
}

func (m *mongoSvc[DocType]) Disconnect(ctx context.Context) error {
	client := m.client.Load()

	if client != nil {
		m.clientLock.Lock()
		defer m.clientLock.Unlock()

		client = m.client.Load()
		defer m.client.Store(nil)
		if client != nil {
			if err := client.Disconnect(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *mongoSvc[DocType]) CreateDocument(ctx context.Context, id string, document *DocType) error {
	ctx, contextCancel := context.WithTimeout(ctx, m.Timeout)
	defer contextCancel()
	client, err := m.connect(ctx)
	if err != nil {
		return err
	}
	db := client.Database(m.DbName)
	collection := db.Collection(m.Collection)
	result := collection.FindOne(ctx, bson.D{{Key: "id", Value: id}})
	switch result.Err() {
	case nil:
		return ErrConflict
	case mongo.ErrNoDocuments:
	default:
		return result.Err()
	}

	_, err = collection.InsertOne(ctx, document)
	return err
}

func (m *mongoSvc[DocType]) FindDocument(ctx context.Context, id string) (*DocType, error) {
	ctx, contextCancel := context.WithTimeout(ctx, m.Timeout)
	defer contextCancel()
	client, err := m.connect(ctx)
	if err != nil {
		return nil, err
	}
	db := client.Database(m.DbName)
	collection := db.Collection(m.Collection)
	result := collection.FindOne(ctx, bson.D{{Key: "id", Value: id}})
	switch result.Err() {
	case nil:
	case mongo.ErrNoDocuments:
		return nil, ErrNotFound
	default:
		return nil, result.Err()
	}
	var document *DocType
	if err := result.Decode(&document); err != nil {
		return nil, err
	}
	return document, nil
}

func (m *mongoSvc[DocType]) UpdateDocument(ctx context.Context, id string, document *DocType) error {
	ctx, contextCancel := context.WithTimeout(ctx, m.Timeout)
	defer contextCancel()
	client, err := m.connect(ctx)
	if err != nil {
		return err
	}
	db := client.Database(m.DbName)
	collection := db.Collection(m.Collection)
	result := collection.FindOne(ctx, bson.D{{Key: "id", Value: id}})
	switch result.Err() {
	case nil:
	case mongo.ErrNoDocuments:
		return ErrNotFound
	default:
		return result.Err()
	}
	_, err = collection.ReplaceOne(ctx, bson.D{{Key: "id", Value: id}}, document)
	return err
}

func (m *mongoSvc[DocType]) DeleteDocument(ctx context.Context, id string) error {
	ctx, contextCancel := context.WithTimeout(ctx, m.Timeout)
	defer contextCancel()
	client, err := m.connect(ctx)
	if err != nil {
		return err
	}
	db := client.Database(m.DbName)
	collection := db.Collection(m.Collection)
	result := collection.FindOne(ctx, bson.D{{Key: "id", Value: id}})
	switch result.Err() {
	case nil:
	case mongo.ErrNoDocuments:
		return ErrNotFound
	default:
		return result.Err()
	}
	_, err = collection.DeleteOne(ctx, bson.D{{Key: "id", Value: id}})
	return err
}

func (m *mongoSvc[DocType]) ListDocuments(ctx context.Context) ([]DocType, error) {
    ctx, cancel := context.WithTimeout(ctx, m.Timeout)
    defer cancel()

    client, err := m.connect(ctx)
    if err != nil {
        return nil, err
    }
    coll := client.Database(m.DbName).Collection(m.Collection)

    cursor, err := coll.Find(ctx, bson.D{}) // no filter = list all
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []DocType
    for cursor.Next(ctx) {
        var doc DocType
        if err := cursor.Decode(&doc); err != nil {
            return nil, err
        }
        results = append(results, doc)
    }
    if err := cursor.Err(); err != nil {
        return nil, err
    }
    return results, nil
}

func (m *mongoSvc[DocType]) FindDocumentsByField(ctx context.Context, fieldName string, value any) ([]*DocType, error) {
    ctx, cancel := context.WithTimeout(ctx, m.Timeout)
    defer cancel()

    client, err := m.connect(ctx)
    if err != nil {
        return nil, err
    }

    coll := client.Database(m.DbName).Collection(m.Collection)

    filter := bson.D{{Key: fieldName, Value: value}}
    cursor, err := coll.Find(ctx, filter)
    if err != nil {
        return nil, err
    }
    defer cursor.Close(ctx)

    var results []*DocType
    for cursor.Next(ctx) {
        var doc DocType
        if err := cursor.Decode(&doc); err != nil {
            return nil, err
        }
        docCopy := doc // capture current doc in memory
        results = append(results, &docCopy)
    }

    if err := cursor.Err(); err != nil {
        return nil, err
    }

    return results, nil
}
