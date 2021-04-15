package test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/NebulousLabs/skynet-accounts/database"
	"github.com/NebulousLabs/skynet-accounts/skynet"

	"gitlab.com/NebulousLabs/errors"
	"gitlab.com/NebulousLabs/fastrand"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// userSubLen is string length of a user's `sub` field
	userSubLen = 36
	// skylinkLen is the byte length of a skylink
	skylinkLen = 46
)

var (
	// skylinkCharset lists all character allowed in a skylink
	skylinkCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
)

// TestUpload_UploadsByUser ensures UploadsByUser returns the correct uploads,
// in the correct order, with the correct sized and so on.
func TestUpload_UploadsByUser(t *testing.T) {
	ctx := context.Background()
	db, err := database.New(ctx, DBTestCredentials(), nil)
	if err != nil {
		t.Fatal(err)
	}
	testUploadSize := int64(1 + fastrand.Intn(1e10))
	// Add a test user.
	sub := string(fastrand.Bytes(userSubLen))
	u, err := db.UserCreate(ctx, sub, database.TierPremium5)
	if err != nil {
		t.Fatal(err)
	}
	defer func(user *database.User) {
		_ = db.UserDelete(ctx, user)
	}(u)
	// Create a skylink record and register an upload for it.
	sl, _, err := createTestUpload(ctx, db, u, testUploadSize)
	if err != nil {
		t.Fatal(err)
	}
	totalUploadSize := testUploadSize
	rawStorageUsed := skynet.RawStorageUsed(testUploadSize)
	uploadBandwidth := skynet.BandwidthUploadCost(testUploadSize)
	uploadsCount := 1
	// Fetch the user's uploads.
	ups, n, err := db.UploadsByUser(ctx, *u, 0, database.DefaultPageSize)
	if err != nil {
		t.Fatal("Failed to fetch uploads by user.", err)
	}
	if n != uploadsCount {
		t.Fatalf("Expected to have %d upload(s), got %d.", uploadsCount, n)
	}
	if ups[0].RawStorage != rawStorageUsed {
		t.Fatalf("Expected the raw storage used of an upload with file size of %d (%d MiB) to be %d (%d MiB), got %d (%d MiB).",
			testUploadSize, testUploadSize/skynet.MiB, rawStorageUsed, rawStorageUsed/skynet.MiB, ups[0].Size, ups[0].Size/skynet.MiB)
	}
	if ups[0].Size != totalUploadSize {
		t.Fatalf("Expected the uploads size of an upload with file size of %d (%d MiB) to be %d (%d MiB), got %d (%d MiB).",
			testUploadSize, testUploadSize/skynet.MiB, totalUploadSize, totalUploadSize/skynet.MiB, ups[0].Size, ups[0].Size/skynet.MiB)
	}
	// Refresh the user's record and make sure we report storage used accurately.
	stats, err := db.UserStats(ctx, *u)
	if err != nil {
		t.Fatal("Failed to fetch user.", err)
	}
	if stats.RawStorageUsed != rawStorageUsed {
		t.Fatalf("Expected raw storage used of %d (%d MiB), got %d (%d MiB).",
			rawStorageUsed, rawStorageUsed/skynet.MiB, stats.RawStorageUsed, stats.RawStorageUsed/skynet.MiB)
	}
	if stats.TotalUploadsSize != totalUploadSize {
		t.Fatalf("Expected total upload size of %d (%d MiB), got %d (%d MiB).",
			totalUploadSize, totalUploadSize/skynet.MiB, stats.TotalUploadsSize, stats.TotalUploadsSize/skynet.MiB)
	}
	if stats.BandwidthUploads != uploadBandwidth {
		t.Fatalf("Expected upload bandwidth used of %d (%d MiB), got %d (%d MiB).",
			uploadBandwidth, uploadBandwidth/skynet.MiB, stats.BandwidthUploads, stats.BandwidthUploads/skynet.MiB)
	}
	if stats.NumUploads != uploadsCount {
		t.Fatalf("Expected to have %d upload(s), got %d.", uploadsCount, stats.NumUploads)
	}
	// Create a second upload for the same skylink. The user's used storage
	// should stay the same but the upload bandwidth should increase.
	_, uid, err := createUpload(ctx, db, u, sl)
	if err != nil {
		t.Fatal("Failed to re-upload.", err)
	}
	totalUploadSize += 0 // raw storage used stays the same
	rawStorageUsed += 0  // storage stays the same
	uploadBandwidth += skynet.BandwidthUploadCost(testUploadSize)
	uploadsCount++
	// Refresh the user's record and make sure we report storage used accurately.
	stats, err = db.UserStats(ctx, *u)
	if err != nil {
		t.Fatal("Failed to fetch user.", err)
	}
	if stats.RawStorageUsed != rawStorageUsed {
		t.Fatalf("Expected raw storage used of %d (%d MiB), got %d (%d MiB).",
			rawStorageUsed, rawStorageUsed/skynet.MiB, stats.RawStorageUsed, stats.RawStorageUsed/skynet.MiB)
	}
	if stats.TotalUploadsSize != totalUploadSize {
		t.Fatalf("Expected total upload size of %d (%d MiB), got %d (%d MiB).",
			totalUploadSize, totalUploadSize/skynet.MiB, stats.TotalUploadsSize, stats.TotalUploadsSize/skynet.MiB)
	}
	if stats.BandwidthUploads != uploadBandwidth {
		t.Fatalf("Expected upload bandwidth used of %d (%d MiB), got %d (%d MiB).",
			uploadBandwidth, uploadBandwidth/skynet.MiB, stats.BandwidthUploads, stats.BandwidthUploads/skynet.MiB)
	}
	if stats.NumUploads != uploadsCount {
		t.Fatalf("Expected to have %d upload(s), got %d.", uploadsCount, stats.NumUploads)
	}
	// Delete the last upload. Expect the number of uploads to go down by 1 but
	// expect the storage used to stay the same because duplicate uploads don't
	// increase the user's used storage.
	unpinned, err := db.UnpinUpload(ctx, uid.Hex(), *u)
	if err != nil {
		t.Fatal("Failed to unpin.", err)
	}
	if unpinned != 1 {
		t.Fatalf("Expected to unpin 1 files, unpinned %d.", unpinned)
	}
	totalUploadSize -= 0 // total upload size stays the same
	rawStorageUsed -= 0  // storage stays the same
	uploadBandwidth -= 0 // upload bandwidth is already used, so it also stay the same
	uploadsCount -= 1
	// Fetch the user's uploads.
	ups, n, err = db.UploadsByUser(ctx, *u, 0, database.DefaultPageSize)
	if err != nil {
		t.Fatal("Failed to fetch uploads by user.", err)
	}
	if n != 1 {
		t.Fatalf("Expected to have exactly %d upload(s), got %d.", 1, n)
	}
	// Refresh the user's record and make sure we report storage used accurately.
	stats, err = db.UserStats(ctx, *u)
	if err != nil {
		t.Fatal("Failed to fetch user.", err)
	}
	if stats.RawStorageUsed != rawStorageUsed {
		t.Fatalf("Expected raw storage used of %d (%d MiB), got %d (%d MiB).",
			rawStorageUsed, rawStorageUsed, stats.RawStorageUsed, stats.RawStorageUsed/skynet.MiB)
	}
	if stats.TotalUploadsSize != totalUploadSize {
		t.Fatalf("Expected total upload size of %d (%d MiB), got %d (%d MiB).",
			totalUploadSize, totalUploadSize, stats.TotalUploadsSize, stats.TotalUploadsSize/skynet.MiB)
	}
	// Upload the same file again. Uploads go up, storage stays the same.
	_, _, err = createUpload(ctx, db, u, sl)
	if err != nil {
		t.Fatal("Failed to re-upload after unpinning.", err)
	}
	totalUploadSize += 0 // total upload size stays the same
	rawStorageUsed += 0  // storage stays the same
	uploadBandwidth += skynet.BandwidthUploadCost(testUploadSize)
	uploadsCount++
	// Refresh the user's record and make sure we report storage used accurately.
	stats, err = db.UserStats(ctx, *u)
	if err != nil {
		t.Fatal("Failed to fetch user.", err)
	}
	if stats.RawStorageUsed != rawStorageUsed {
		t.Fatalf("Expected raw storage used of %d (%d MiB), got %d (%d MiB).",
			rawStorageUsed, rawStorageUsed/skynet.MiB, stats.RawStorageUsed, stats.RawStorageUsed/skynet.MiB)
	}
	if stats.TotalUploadsSize != totalUploadSize {
		t.Fatalf("Expected total upload size of %d (%d MiB), got %d (%d MiB).",
			totalUploadSize, totalUploadSize/skynet.MiB, stats.TotalUploadsSize, stats.TotalUploadsSize/skynet.MiB)
	}
	if stats.BandwidthUploads != uploadBandwidth {
		t.Fatalf("Expected upload bandwidth used of %d (%d MiB), got %d (%d MiB).",
			uploadBandwidth, uploadBandwidth/skynet.MiB, stats.BandwidthUploads, stats.BandwidthUploads/skynet.MiB)
	}
	if stats.NumUploads != uploadsCount {
		t.Fatalf("Expected to have %d upload(s), got %d.", uploadsCount, stats.NumUploads)
	}
}

// TestUpload_UnpinUploads ensures UnpinUploads unpins all uploads of this
// skylink by this user without affecting uploads by other users.
func TestUpload_UnpinUploads(t *testing.T) {
	ctx := context.Background()
	db, err := database.New(ctx, DBTestCredentials(), nil)
	if err != nil {
		t.Fatal(err)
	}
	testUploadSize := int64(1 + fastrand.Intn(1e10))
	// Add two test users.
	sub1 := string(fastrand.Bytes(userSubLen))
	u1, err := db.UserCreate(ctx, sub1, database.TierPremium5)
	if err != nil {
		t.Fatal(err)
	}
	defer func(user *database.User) {
		_ = db.UserDelete(ctx, user)
	}(u1)
	sub2 := string(fastrand.Bytes(userSubLen))
	u2, err := db.UserCreate(ctx, sub2, database.TierPremium5)
	if err != nil {
		t.Fatal(err)
	}
	defer func(user *database.User) {
		_ = db.UserDelete(ctx, user)
	}(u2)
	// Create a skylink record and register an upload for it.
	sl, _, err := createTestUpload(ctx, db, u1, testUploadSize)
	if err != nil {
		t.Fatal(err)
	}
	// Upload it again for the same user.
	_, _, err = createUpload(ctx, db, u1, sl)
	if err != nil {
		t.Fatal("Failed to re-upload.", err)
	}
	// Upload it for the second user.
	_, _, err = createUpload(ctx, db, u2, sl)
	if err != nil {
		t.Fatal("Failed to re-upload.", err)
	}
	// Delete all uploads by the first user.
	unpinned, err := db.UnpinUploads(ctx, *sl, *u1)
	if err != nil {
		t.Fatal("Failed to unpin.", err)
	}
	if unpinned != 2 {
		t.Fatalf("Expected to unpin 2 files, unpinned %d.", unpinned)
	}
	// Fetch the first user's uploads.
	_, n, err := db.UploadsByUser(ctx, *u1, 0, database.DefaultPageSize)
	if err != nil {
		t.Fatal("Failed to fetch uploads by user1.", err)
	}
	if n != 0 {
		t.Fatalf("Expected to have exactly %d upload(s), got %d.", 0, n)
	}
	// Refresh the user's stats and make sure we report storage used accurately.
	stats, err := db.UserStats(ctx, *u1)
	if err != nil {
		t.Fatal("Failed to fetch user1.", err)
	}
	if stats.RawStorageUsed != 0 {
		t.Fatalf("Expected raw storage used of %d (%d MiB), got %d (%d MiB).",
			0, 0, stats.RawStorageUsed, stats.RawStorageUsed/skynet.MiB)
	}
	if stats.TotalUploadsSize != 0 {
		t.Fatalf("Expected total upload size of %d (%d MiB), got %d (%d MiB).",
			0, 0, stats.TotalUploadsSize, stats.TotalUploadsSize/skynet.MiB)
	}
	expectedUploadBandwidth := 2 * skynet.BandwidthUploadCost(testUploadSize)
	if stats.BandwidthUploads != expectedUploadBandwidth {
		t.Fatalf("Expected upload bandwidth used of %d (%d MiB), got %d (%d MiB).",
			expectedUploadBandwidth, expectedUploadBandwidth/skynet.MiB, stats.BandwidthUploads, stats.BandwidthUploads/skynet.MiB)
	}
	// Fetch the second user's uploads.
	_, n, err = db.UploadsByUser(ctx, *u2, 0, database.DefaultPageSize)
	if err != nil {
		t.Fatal("Failed to fetch uploads by user2.", err)
	}
	if n != 1 {
		t.Fatalf("Expected to have exactly %d upload(s), got %d.", 1, n)
	}
	// Refresh the user's stats and make sure we report storage used accurately.
	stats, err = db.UserStats(ctx, *u2)
	if err != nil {
		t.Fatal("Failed to fetch user2.", err)
	}
	expectedRawStorage := skynet.RawStorageUsed(testUploadSize)
	if stats.RawStorageUsed != expectedRawStorage {
		t.Fatalf("Expected raw storage used of %d (%d MiB), got %d (%d MiB).",
			expectedRawStorage, expectedRawStorage, stats.RawStorageUsed, stats.RawStorageUsed/skynet.MiB)
	}
	if stats.TotalUploadsSize != testUploadSize {
		t.Fatalf("Expected total upload size of %d (%d MiB), got %d (%d MiB).",
			testUploadSize, testUploadSize, stats.TotalUploadsSize, stats.TotalUploadsSize/skynet.MiB)
	}
	expectedUploadBandwidth = skynet.BandwidthUploadCost(testUploadSize)
	if stats.BandwidthUploads != expectedUploadBandwidth {
		t.Fatalf("Expected upload bandwidth used of %d (%d MiB), got %d (%d MiB).",
			expectedUploadBandwidth, expectedUploadBandwidth/skynet.MiB, stats.BandwidthUploads, stats.BandwidthUploads/skynet.MiB)
	}
}

// createTestUpload creates a new skyfile and uploads it under the given user's
// account. Returns the skylink, the upload's id and error.
func createTestUpload(ctx context.Context, db *database.DB, user *database.User, size int64) (*database.Skylink, primitive.ObjectID, error) {
	// Create a skylink record for which to register an upload
	sl := randomSkylink()
	skylink, err := db.Skylink(ctx, sl)
	if err != nil {
		return nil, primitive.ObjectID{}, errors.AddContext(err, "failed to create a test skylink")
	}
	err = db.SkylinkUpdate(ctx, skylink.ID, "test skylink "+sl, size)
	if err != nil {
		return nil, primitive.ObjectID{}, errors.AddContext(err, "failed to update skylink")
	}
	// Get the updated skylink.
	skylink, err = db.Skylink(ctx, sl)
	if err != nil {
		return nil, primitive.ObjectID{}, errors.AddContext(err, "failed to fetch skylink from DB")
	}
	if skylink.Size != size {
		return nil, primitive.ObjectID{}, errors.AddContext(err, fmt.Sprintf("expected skylink size to be %d, got %d.", size, skylink.Size))
	}
	// Register an upload.
	return createUpload(ctx, db, user, skylink)
}

// createUpload registers an upload of the given skylink by the given user.
// Returns the skylink, the upload's id and error.
func createUpload(ctx context.Context, db *database.DB, user *database.User, skylink *database.Skylink) (*database.Skylink, primitive.ObjectID, error) {
	up, err := db.UploadCreate(ctx, *user, *skylink)
	if err != nil {
		return nil, primitive.ObjectID{}, errors.AddContext(err, "failed to register an upload")
	}
	if up.UserID != user.ID {
		return nil, primitive.ObjectID{}, errors.AddContext(err, "expected upload's userId to match the uploader's id")
	}
	if up.SkylinkID != skylink.ID {
		return nil, primitive.ObjectID{}, errors.AddContext(err, "expected upload's skylinkId to match the given skylink's id")
	}
	return skylink, up.ID, nil
}

// randomSkylink generates a random skylink
func randomSkylink() string {
	sb := strings.Builder{}
	for i := 0; i < skylinkLen; i++ {
		_ = sb.WriteByte(skylinkCharset[fastrand.Intn(len(skylinkCharset))])
	}
	return sb.String()
}
