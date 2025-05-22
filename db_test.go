package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testDB(tb testing.TB) *convoDB {
	db, err := openDB(":memory:")
	require.NoError(tb, err)
	tb.Cleanup(func() {
		require.NoError(tb, db.Close())
	})
	return db
}

func TestConvoDB(t *testing.T) {
	const testid = "df31ae23ab8b75b5643c2f846c570997edc71333"

	t.Run("list-empty", func(t *testing.T) {
		db := testDB(t)
		list, err := db.List()
		require.NoError(t, err)
		require.Empty(t, list)
	})

	t.Run("save", func(t *testing.T) {
		db := testDB(t)

		require.NoError(t, db.Save(testid, "message 1", "openai", "gpt-4o"))

		convo, err := db.Find("df31")
		require.NoError(t, err)
		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 1", convo.Title)

		list, err := db.List()
		require.NoError(t, err)
		require.Len(t, list, 1)
	})

	t.Run("save no id", func(t *testing.T) {
		db := testDB(t)
		require.Error(t, db.Save("", "message 1", "openai", "gpt-4o"))
	})

	t.Run("save no message", func(t *testing.T) {
		db := testDB(t)
		require.Error(t, db.Save(newConversationID(), "", "openai", "gpt-4o"))
	})

	t.Run("update", func(t *testing.T) {
		db := testDB(t)

		require.NoError(t, db.Save(testid, "message 1", "openai", "gpt-4o"))
		time.Sleep(100 * time.Millisecond)
		require.NoError(t, db.Save(testid, "message 2", "openai", "gpt-4o"))

		convo, err := db.Find("df31")
		require.NoError(t, err)
		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 2", convo.Title)

		list, err := db.List()
		require.NoError(t, err)
		require.Len(t, list, 1)
	})

	t.Run("find head single", func(t *testing.T) {
		db := testDB(t)

		require.NoError(t, db.Save(testid, "message 2", "openai", "gpt-4o"))

		head, err := db.FindHEAD()
		require.NoError(t, err)
		require.Equal(t, testid, head.ID)
		require.Equal(t, "message 2", head.Title)
	})

	t.Run("find head multiple", func(t *testing.T) {
		db := testDB(t)

		require.NoError(t, db.Save(testid, "message 2", "openai", "gpt-4o"))
		time.Sleep(time.Millisecond * 100)
		nextConvo := newConversationID()
		require.NoError(t, db.Save(nextConvo, "another message", "openai", "gpt-4o"))

		head, err := db.FindHEAD()
		require.NoError(t, err)
		require.Equal(t, nextConvo, head.ID)
		require.Equal(t, "another message", head.Title)

		list, err := db.List()
		require.NoError(t, err)
		require.Len(t, list, 2)
	})

	t.Run("find by title", func(t *testing.T) {
		db := testDB(t)

		require.NoError(t, db.Save(newConversationID(), "message 1", "openai", "gpt-4o"))
		require.NoError(t, db.Save(testid, "message 2", "openai", "gpt-4o"))

		convo, err := db.Find("message 2")
		require.NoError(t, err)
		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 2", convo.Title)
	})

	t.Run("find match nothing", func(t *testing.T) {
		db := testDB(t)
		require.NoError(t, db.Save(testid, "message 1", "openai", "gpt-4o"))
		_, err := db.Find("message")
		require.ErrorIs(t, err, errNoMatches)
	})

	t.Run("find match many", func(t *testing.T) {
		db := testDB(t)
		const testid2 = "df31ae23ab9b75b5641c2f846c571000edc71315"
		require.NoError(t, db.Save(testid, "message 1", "openai", "gpt-4o"))
		require.NoError(t, db.Save(testid2, "message 2", "openai", "gpt-4o"))
		_, err := db.Find("df31ae")
		require.ErrorIs(t, err, errManyMatches)
	})

	t.Run("delete", func(t *testing.T) {
		db := testDB(t)

		require.NoError(t, db.Save(testid, "message 1", "openai", "gpt-4o"))
		require.NoError(t, db.Delete(newConversationID()))

		list, err := db.List()
		require.NoError(t, err)
		require.NotEmpty(t, list)

		for _, item := range list {
			require.NoError(t, db.Delete(item.ID))
		}

		list, err = db.List()
		require.NoError(t, err)
		require.Empty(t, list)
	})

	t.Run("completions", func(t *testing.T) {
		db := testDB(t)

		const testid1 = "fc5012d8c67073ea0a46a3c05488a0e1d87df74b"
		const title1 = "some title"
		const testid2 = "6c33f71694bf41a18c844a96d1f62f153e5f6f44"
		const title2 = "football teams"
		require.NoError(t, db.Save(testid1, title1, "openai", "gpt-4o"))
		require.NoError(t, db.Save(testid2, title2, "openai", "gpt-4o"))

		results, err := db.Completions("f")
		require.NoError(t, err)
		require.Equal(t, []string{
			fmt.Sprintf("%s\t%s", testid1[:sha1short], title1),
			fmt.Sprintf("%s\t%s", title2, testid2[:sha1short]),
		}, results)

		results, err = db.Completions(testid1[:8])
		require.NoError(t, err)
		require.Equal(t, []string{
			fmt.Sprintf("%s\t%s", testid1, title1),
		}, results)
	})
}
