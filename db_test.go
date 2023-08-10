package main

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvoDB(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sqlite tests failing on windows for some reason")
	}

	const testid = "df31ae23ab8b75b5643c2f846c570997edc71333"
	db, err := openDB(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	t.Run("list-empty", func(t *testing.T) {
		list, err := db.List()
		require.NoError(t, err)
		require.Empty(t, list)
	})

	t.Run("save", func(t *testing.T) {
		require.NoError(t, db.Save(testid, "message 1"))

		convo, err := db.Find("df31")
		require.NoError(t, err)
		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 1", convo.Title)

		list, err := db.List()
		require.NoError(t, err)
		require.Len(t, list, 1)
	})

	t.Run("save no id", func(t *testing.T) {
		require.Error(t, db.Save("", "message 1"))
	})

	t.Run("save no message", func(t *testing.T) {
		require.Error(t, db.Save(newConversationID(), ""))
	})

	t.Run("update", func(t *testing.T) {
		require.NoError(t, db.Save(testid, "message 2"))

		convo, err := db.Find("df31")
		require.NoError(t, err)
		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 2", convo.Title)

		list, err := db.List()
		require.NoError(t, err)
		require.Len(t, list, 1)
	})

	t.Run("find head", func(t *testing.T) {
		convo, err := db.FindHEAD()
		require.NoError(t, err)

		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 2", convo.Title)

		nextConvo := newConversationID()
		require.NoError(t, db.Save(nextConvo, "another message"))

		head, err := db.FindHEAD()
		require.NoError(t, err)

		require.Equal(t, nextConvo, head.ID)
		require.Equal(t, "another message", head.Title)

		list, err := db.List()
		require.NoError(t, err)
		require.Len(t, list, 2)
	})

	t.Run("find by title", func(t *testing.T) {
		convo, err := db.Find("message 2")
		require.NoError(t, err)

		require.Equal(t, testid, convo.ID)
		require.Equal(t, "message 2", convo.Title)
	})

	t.Run("find match nothing", func(t *testing.T) {
		_, err := db.Find("message")
		require.ErrorIs(t, err, errNoMatches)
	})

	t.Run("find match many", func(t *testing.T) {
		const testid2 = "df31ae23ab9b75b5641c2f846c571000edc71315"
		require.NoError(t, db.Save(testid2, "message 4"))
		_, err := db.Find("df31ae")
		require.ErrorIs(t, err, errManyMatches)
	})

	t.Run("delete", func(t *testing.T) {
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
}
