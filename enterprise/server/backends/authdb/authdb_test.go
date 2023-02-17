package authdb_test

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/buildbuddy-io/buildbuddy/enterprise/server/testutil/enterprise_testauth"
	"github.com/buildbuddy-io/buildbuddy/enterprise/server/testutil/enterprise_testenv"
	"github.com/buildbuddy-io/buildbuddy/server/environment"
	"github.com/buildbuddy-io/buildbuddy/server/tables"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testauth"
	"github.com/buildbuddy-io/buildbuddy/server/util/db"
	"github.com/buildbuddy-io/buildbuddy/server/util/role"
	"github.com/buildbuddy-io/buildbuddy/server/util/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionInsertUpdateDeleteRead(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()
	env := setupEnv(t)
	adb := env.GetAuthDB()

	// Insert many sessions; should all succeed
	const nSessions = 10
	for i := 0; i < nSessions; i++ {
		sid := strconv.Itoa(i)
		s := &tables.Session{
			SubID:        "SubID-" + sid,
			AccessToken:  "AccessToken-" + sid,
			RefreshToken: "RefreshToken-" + sid,
		}
		err := adb.InsertOrUpdateUserSession(ctx, sid, s)
		require.NoError(t, err)
	}

	// Try updating a random session; should succeed.
	sidToUpdate := strconv.Itoa(rand.Intn(nSessions))
	s := &tables.Session{AccessToken: "UPDATED-AccessToken-" + sidToUpdate}
	err := adb.InsertOrUpdateUserSession(ctx, sidToUpdate, s)
	require.NoError(t, err)

	// Try deleting a different random session; should succeed.
	sidToDelete := strconv.Itoa(rand.Intn(nSessions))
	for sidToDelete == sidToUpdate {
		sidToDelete = strconv.Itoa(rand.Intn(nSessions))
	}
	err = adb.ClearSession(ctx, sidToDelete)
	require.NoError(t, err)

	// Read back all the sessions, including the updated and deleted ones.
	for i := 0; i < nSessions; i++ {
		sid := strconv.Itoa(i)
		s, err := adb.ReadSession(ctx, sid)
		if sid == sidToDelete {
			require.Truef(
				t, db.IsRecordNotFound(err),
				"expected RecordNotFound, got: %v", err)
			continue
		}

		require.NoError(t, err)
		expected := &tables.Session{
			Model:        s.Model,
			SessionID:    sid,
			SubID:        "SubID-" + sid,
			AccessToken:  "AccessToken-" + sid,
			RefreshToken: "RefreshToken-" + sid,
		}
		if sid == sidToUpdate {
			expected.AccessToken = "UPDATED-AccessToken-" + sid
		}
		require.Equal(t, expected, s)
	}
}

func TestGetAPIKeyGroupFromAPIKey(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()
	env := setupEnv(t)
	adb := env.GetAuthDB()

	keys := createRandomAPIKeys(t, ctx, env)
	randKey := keys[rand.Intn(len(keys))]

	akg, err := adb.GetAPIKeyGroupFromAPIKey(ctx, randKey.Value)
	require.NoError(t, err)

	assert.Equal(t, randKey.GroupID, akg.GetGroupID())
	assert.Equal(t, randKey.Capabilities, akg.GetCapabilities())
	assert.Equal(t, false, akg.GetUseGroupOwnedExecutors())

	// Using an invalid or empty value should produce an error
	akg, err = adb.GetAPIKeyGroupFromAPIKey(ctx, "")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
	akg, err = adb.GetAPIKeyGroupFromAPIKey(ctx, "INVALID")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
}

func TestGetAPIKeyGroupFromAPIKeyID(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()
	env := setupEnv(t)
	adb := env.GetAuthDB()

	keys := createRandomAPIKeys(t, ctx, env)
	randKey := keys[rand.Intn(len(keys))]

	akg, err := adb.GetAPIKeyGroupFromAPIKeyID(ctx, randKey.APIKeyID)
	require.NoError(t, err)

	assert.Equal(t, randKey.GroupID, akg.GetGroupID())
	assert.Equal(t, randKey.Capabilities, akg.GetCapabilities())
	assert.Equal(t, false, akg.GetUseGroupOwnedExecutors())

	// Using an invalid or empty value should produce an error
	akg, err = adb.GetAPIKeyGroupFromAPIKeyID(ctx, "")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
	akg, err = adb.GetAPIKeyGroupFromAPIKeyID(ctx, "INVALID")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
}

func TestGetAPIKeyGroupFromBasicAuth(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()
	env := setupEnv(t)
	adb := env.GetAuthDB()

	keys := createRandomAPIKeys(t, ctx, env)
	randKey := keys[rand.Intn(len(keys))]

	// Look up the write token for the group
	g, err := env.GetUserDB().GetGroupByID(ctx, randKey.GroupID)
	require.NoError(t, err)
	require.Equal(
		t, randKey.GroupID, g.GroupID,
		"sanity check: group ID should match the API key ID")

	akg, err := adb.GetAPIKeyGroupFromBasicAuth(ctx, g.GroupID, g.WriteToken)
	require.NoError(t, err)

	assert.Equal(t, randKey.GroupID, akg.GetGroupID())
	assert.Equal(t, randKey.Capabilities, akg.GetCapabilities())
	assert.Equal(t, false, akg.GetUseGroupOwnedExecutors())

	// Using invalid or empty values should produce an error
	akg, err = adb.GetAPIKeyGroupFromBasicAuth(ctx, "", "")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
	akg, err = adb.GetAPIKeyGroupFromBasicAuth(ctx, "", g.WriteToken)
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
	akg, err = adb.GetAPIKeyGroupFromBasicAuth(ctx, g.GroupID, "")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
	akg, err = adb.GetAPIKeyGroupFromBasicAuth(ctx, "INVALID", g.WriteToken)
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
	akg, err = adb.GetAPIKeyGroupFromBasicAuth(ctx, g.GroupID, "INVALID")
	require.Nil(t, akg)
	require.Truef(
		t, status.IsUnauthenticatedError(err),
		"expected Unauthenticated error; got: %v", err)
}

func TestLookupUserFromSubID(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ctx := context.Background()
	env := setupEnv(t)
	adb := env.GetAuthDB()

	users := enterprise_testauth.CreateRandomGroups(t, env)
	randUser := users[rand.Intn(len(users))]

	u, err := adb.LookupUserFromSubID(ctx, randUser.SubID)
	require.NoError(t, err)
	require.Equal(t, randUser, u)

	// Using empty or invalid values should produce an error
	u, err = adb.LookupUserFromSubID(ctx, "")
	require.Nil(t, u)
	require.Truef(
		t, db.IsRecordNotFound(err),
		"expected RecordNotFound error; got: %v", err)
	u, err = adb.LookupUserFromSubID(ctx, "INVALID")
	require.Nil(t, u)
	require.Truef(
		t, db.IsRecordNotFound(err),
		"expected RecordNotFound error; got: %v", err)
}

func createRandomAPIKeys(t *testing.T, ctx context.Context, env environment.Env) []*tables.APIKey {
	users := enterprise_testauth.CreateRandomGroups(t, env)
	var allKeys []*tables.APIKey
	// List the org API keys accessible to any admins we created
	auth := env.GetAuthenticator().(*testauth.TestAuthenticator)
	for _, u := range users {
		authCtx, err := auth.WithAuthenticatedUser(ctx, u.UserID)
		require.NoError(t, err)
		if role.Role(u.Groups[0].Role) != role.Admin {
			continue
		}
		keys, err := env.GetUserDB().GetAPIKeys(authCtx, u.Groups[0].Group.GroupID)
		require.NoError(t, err)
		allKeys = append(allKeys, keys...)
	}
	require.NotEmpty(t, allKeys, "sanity check: should have created some random API keys")
	return allKeys
}

func setupEnv(t *testing.T) environment.Env {
	env := enterprise_testenv.New(t)
	enterprise_testauth.Configure(t, env) // provisions AuthDB and UserDB
	return env
}