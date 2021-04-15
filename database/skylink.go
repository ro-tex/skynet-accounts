package database

import (
	"context"
	"regexp"

	"gitlab.com/NebulousLabs/errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	extractSkylinkRE      = regexp.MustCompile("^.*([a-zA-Z0-9-_]{46}).*$")
	validateSkylinkHashRE = regexp.MustCompile("^([a-zA-Z0-9-_]{46})$")
)

// Skylink represents a skylink object in the DB.
type Skylink struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	Skylink string             `bson:"skylink" json:"skylink"`
	Size    int64              `bson:"size" json:"size"`
}

// Skylink gets the DB object for the given skylink.
// If it doesn't exist it creates it.
func (db *DB) Skylink(ctx context.Context, skylink string) (*Skylink, error) {
	skylinkHash, err := ExtractSkylinkHash(skylink)
	if err != nil {
		return nil, ErrInvalidSkylink
	}
	// Provisional skylink object.
	skylinkRec := Skylink{
		Skylink: skylinkHash,
	}
	// Try to find the skylink in the database.
	filter := bson.D{{"skylink", skylinkHash}}
	sr := db.staticSkylinks.FindOne(ctx, filter)
	err = sr.Decode(&skylinkRec)
	if errors.Contains(err, mongo.ErrNoDocuments) {
		// It's not there, upsert it. We use upsert instead of insert in order
		// to avoid races. And we use an update object instead of just passing
		// the skylink record to UpdateOne because we want to omit the _id in
		// case it has a zero value. The struct tags instruct the compiler to
		// omit it when it's empty but that doesn't cover the case where it's
		// zero because in that case it's a valid array of ints which happen to
		// be zeros.
		upsert := bson.M{"$set": bson.M{
			"skylink": skylinkHash,
		}}
		opts := options.Update().SetUpsert(true)
		var ur *mongo.UpdateResult
		ur, err = db.staticSkylinks.UpdateOne(ctx, filter, upsert, opts)
		if err == nil {
			skylinkRec.ID = ur.UpsertedID.(primitive.ObjectID)
		}
	}
	if err != nil {
		return nil, err
	}
	return &skylinkRec, nil
}

// SkylinkByID finds a skylink by its ID.
func (db *DB) SkylinkByID(ctx context.Context, id primitive.ObjectID) (*Skylink, error) {
	filter := bson.D{{"_id", id}}
	sr := db.staticSkylinks.FindOne(ctx, filter)
	var sl Skylink
	err := sr.Decode(&sl)
	if err != nil {
		return nil, err
	}
	return &sl, nil
}

// SkylinkUpdate updates the meta data about the given skylink. If any of the
// parameters is empty they won't be used in the update operation.
func (db *DB) SkylinkUpdate(ctx context.Context, id primitive.ObjectID, name string, size int64) error {
	filter := bson.M{"_id": id}
	updates := bson.M{}
	if name != "" {
		updates["name"] = name
	}
	if size > 0 {
		updates["size"] = size
	}
	_, err := db.staticSkylinks.UpdateOne(ctx, filter, bson.M{"$set": updates})
	if err != nil {
		return errors.AddContext(err, "failed to update")
	}
	return nil
}

// SkylinkDownloadsUpdate changes the size of the full downloads of this
// skylink. Those should have zero `bytes` in the DB. This method should be
// called from the fetcher.
func (db *DB) SkylinkDownloadsUpdate(ctx context.Context, id primitive.ObjectID, bytes int64) error {
	filter := bson.M{"_id": id}
	updates := bson.M{}
	updates["bytes"] = bytes
	_, err := db.staticDownloads.UpdateMany(ctx, filter, bson.M{"$set": updates})
	if err != nil {
		return errors.AddContext(err, "failed to update")
	}
	return nil
}

// ExtractSkylinkHash extracts the skylink hash from the given skylink that might
// have protocol, path, etc. within it.
func ExtractSkylinkHash(skylink string) (string, error) {
	m := extractSkylinkRE.FindStringSubmatch(skylink)
	if len(m) < 2 {
		return "", errors.New("no valid skylink found in string " + skylink)
	}
	return m[1], nil
}

// ValidSkylinkHash returns true if the given string is a valid skylink hash.
func ValidSkylinkHash(skylink string) bool {
	return validateSkylinkHashRE.Match([]byte(skylink))
}
