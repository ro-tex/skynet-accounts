package database

import (
	"context"
	"encoding/base64"
	"time"

	"gitlab.com/NebulousLabs/errors"
	"gitlab.com/NebulousLabs/fastrand"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type (
	// PubAPIKey is a base64URL-encoded representation of []byte with length
	// PubKeySize
	PubAPIKey string
	// PubAPIKeyRecord is a non-expiring authentication token generated on user
	// demand. This token allows anyone to access a set of pre-determined
	// skylinks. The traffic generated by this access is counted towards the
	// issuing user's balance.
	PubAPIKeyRecord struct {
		ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
		UserID    primitive.ObjectID `bson:"user_id" json:"userID"`
		Key       PubAPIKey          `bson:"key" json:"key"`
		Skylinks  []string           `bson:"skylinks" json:"skylinks"`
		CreatedAt time.Time          `bson:"created_at" json:"createdAt"`
	}
)

// IsValid checks whether the underlying string satisfies the type's requirement
// to represent a []byte with length PubKeySize which is encoded as base64URL.
// This method does NOT check whether the public API key exists in the database.
func (pak PubAPIKey) IsValid() bool {
	b := make([]byte, PubKeySize)
	n, err := base64.URLEncoding.Decode(b, []byte(pak))
	return err == nil && n == PubKeySize
}

// PubAPIKeyCreate creates a new public API key.
func (db *DB) PubAPIKeyCreate(ctx context.Context, user User, skylinks []string) (*PubAPIKeyRecord, error) {
	if user.ID.IsZero() {
		return nil, errors.New("invalid user")
	}
	n, err := db.staticPubAPIKeys.CountDocuments(ctx, bson.M{"user_id": user.ID})
	if err != nil {
		return nil, errors.AddContext(err, "failed to ensure user can create a new API key")
	}
	if n > int64(MaxNumAPIKeysPerUser) {
		return nil, ErrMaxNumAPIKeysExceeded
	}
	// Validate all given skylinks.
	for _, s := range skylinks {
		if !ValidSkylinkHash(s) {
			return nil, ErrInvalidSkylink
		}
	}
	pakRec := PubAPIKeyRecord{
		UserID:    user.ID,
		Key:       PubAPIKey(base64.URLEncoding.EncodeToString(fastrand.Bytes(PubKeySize))),
		Skylinks:  skylinks,
		CreatedAt: time.Now().UTC(),
	}
	ior, err := db.staticAPIKeys.InsertOne(ctx, pakRec)
	if err != nil {
		return nil, err
	}
	pakRec.ID = ior.InsertedID.(primitive.ObjectID)
	return &pakRec, nil
}

// PubAPIKeyUpdate updates an existing PubAPIKey. This works by replacing the
// list of Skylinks within the PubAPIKey record.
func (db *DB) PubAPIKeyUpdate(ctx context.Context, user User, pak PubAPIKey, skylinks []string) error {
	if user.ID.IsZero() {
		return errors.New("invalid user")
	}
	// Validate all given skylinks.
	for _, s := range skylinks {
		if !ValidSkylinkHash(s) {
			return ErrInvalidSkylink
		}
	}
	filter := bson.M{
		"key":     pak,
		"user_id": user.ID,
	}
	update := bson.M{"skylinks": skylinks}
	opts := options.UpdateOptions{
		Upsert: &False,
	}
	_, err := db.staticPubAPIKeys.UpdateOne(ctx, filter, update, &opts)
	return err
}

// PubAPIKeyPatch updates an existing PubAPIKey. This works by adding and
// removing specific elements directly in Mongo.
func (db *DB) PubAPIKeyPatch(ctx context.Context, user User, pak PubAPIKey, addSkylinks, removeSkylinks []string) error {
	if user.ID.IsZero() {
		return errors.New("invalid user")
	}
	// Validate all given skylinks.
	for _, s := range append(addSkylinks, removeSkylinks...) {
		if !ValidSkylinkHash(s) {
			return ErrInvalidSkylink
		}
	}
	var filter, update bson.M
	// First, all new skylinks to the record.
	if len(addSkylinks) > 0 {
		filter = bson.M{"key": pak}
		update = bson.M{
			"$push": bson.M{"skylinks": bson.M{"$each": addSkylinks}},
		}
		opts := options.UpdateOptions{
			Upsert: &False,
		}
		_, err := db.staticPubAPIKeys.UpdateOne(ctx, filter, update, &opts)
		if err != nil {
			return err
		}
	}
	// Then, remove all skylinks that need to be removed.
	if len(removeSkylinks) > 0 {
		filter = bson.M{"key": pak}
		update = bson.M{
			"pull": bson.M{"skylinks": bson.M{"$in": addSkylinks}},
		}
		opts := options.UpdateOptions{
			Upsert: &False,
		}
		_, err := db.staticPubAPIKeys.UpdateOne(ctx, filter, update, &opts)
		if err != nil {
			return err
		}
	}
	return nil
}

// PubAPIKeyDelete deletes a public API key.
func (db *DB) PubAPIKeyDelete(ctx context.Context, user User, pakID string) error {
	if user.ID.IsZero() {
		return errors.New("invalid user")
	}
	id, err := primitive.ObjectIDFromHex(pakID)
	if err != nil {
		return errors.AddContext(err, "invalid API key ID")
	}
	filter := bson.M{
		"_id":     id,
		"user_id": user.ID,
	}
	dr, err := db.staticPubAPIKeys.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if dr.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// PubAPIKeyGetRecord returns a specific public API key.
func (db *DB) PubAPIKeyGetRecord(ctx context.Context, pak PubAPIKey) (PubAPIKeyRecord, error) {
	sr := db.staticPubAPIKeys.FindOne(ctx, bson.M{"key": pak})
	if sr.Err() != nil {
		return PubAPIKeyRecord{}, sr.Err()
	}
	var pakRec PubAPIKeyRecord
	err := sr.Decode(&pakRec)
	if err != nil {
		return PubAPIKeyRecord{}, err
	}
	return pakRec, nil
}

// PubAPIKeyList lists all public API keys that belong to the user.
func (db *DB) PubAPIKeyList(ctx context.Context, user User) ([]*PubAPIKeyRecord, error) {
	if user.ID.IsZero() {
		return nil, errors.New("invalid user")
	}
	c, err := db.staticPubAPIKeys.Find(ctx, bson.M{"user_id": user.ID})
	if err != nil {
		return nil, err
	}
	var paks []*PubAPIKeyRecord
	err = c.All(ctx, &paks)
	if err != nil {
		return nil, err
	}
	return paks, nil
}
