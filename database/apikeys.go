package database

import (
	"context"
	"encoding/base64"
	"time"

	"gitlab.com/NebulousLabs/errors"
	"gitlab.com/NebulousLabs/fastrand"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

/**
API keys are authentication tokens generated by users. They do not expire, thus
allowing users to use them for a long time and to embed them in apps and on
machines. API keys can be revoked when they are no longer needed or if they get
compromised. This is done by deleting them from this service.
*/

type (
	// APIKey is a base64URL-encoded representation of []byte with length PubKeySize
	APIKey string
	// APIKeyRecord is a non-expiring authentication token generated on user demand.
	APIKeyRecord struct {
		ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
		UserID    primitive.ObjectID `bson:"user_id" json:"-"`
		Key       APIKey             `bson:"key" json:"key"`
		CreatedAt time.Time          `bson:"created_at" json:"createdAt"`
	}
)

// IsValid checks whether the underlying string satisfies the type's requirement
// to represent a []byte with length PubKeySize which is encoded as base64URL.
// This method does NOT check whether the API exists in the database.
func (ak APIKey) IsValid() bool {
	b := make([]byte, PubKeySize)
	n, err := base64.URLEncoding.Decode(b, []byte(ak))
	return err == nil && n == PubKeySize
}

// APIKeyCreate creates a new API key.
func (db *DB) APIKeyCreate(ctx context.Context, user User) (*APIKeyRecord, error) {
	if user.ID.IsZero() {
		return nil, errors.New("invalid user")
	}
	ak := APIKeyRecord{
		UserID:    user.ID,
		Key:       APIKey(base64.URLEncoding.EncodeToString(fastrand.Bytes(PubKeySize))),
		CreatedAt: time.Now().UTC(),
	}
	ior, err := db.staticAPIKeys.InsertOne(ctx, ak)
	if err != nil {
		return nil, err
	}
	ak.ID = ior.InsertedID.(primitive.ObjectID)
	return &ak, nil
}

// APIKeyDelete deletes an API key.
func (db *DB) APIKeyDelete(ctx context.Context, user User, ak string) error {
	if user.ID.IsZero() {
		return errors.New("invalid user")
	}
	filter := bson.M{
		"key":     ak,
		"user_id": user.ID,
	}
	_, err := db.staticAPIKeys.DeleteOne(ctx, filter)
	return err
}

// APIKeyList lists all API keys that belong to the user.
func (db *DB) APIKeyList(ctx context.Context, user User) ([]*APIKeyRecord, error) {
	if user.ID.IsZero() {
		return nil, errors.New("invalid user")
	}
	c, err := db.staticAPIKeys.Find(ctx, bson.M{"user_id": user.ID})
	if err != nil {
		return nil, err
	}
	var aks []*APIKeyRecord
	err = c.All(ctx, &aks)
	if err != nil {
		return nil, err
	}
	return aks, nil
}
