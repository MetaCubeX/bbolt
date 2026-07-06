package bbolt

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/metacubex/bbolt/errors"
)

func TestOpenWithPreLoadFreelist(t *testing.T) {
	testCases := []struct {
		name                    string
		readonly                bool
		preLoadFreePage         bool
		expectedFreePagesLoaded bool
	}{
		{
			name:                    "write mode always load free pages",
			readonly:                false,
			preLoadFreePage:         false,
			expectedFreePagesLoaded: true,
		},
		{
			name:                    "readonly mode load free pages when flag set",
			readonly:                true,
			preLoadFreePage:         true,
			expectedFreePagesLoaded: true,
		},
		{
			name:                    "readonly mode doesn't load free pages when flag not set",
			readonly:                true,
			preLoadFreePage:         false,
			expectedFreePagesLoaded: false,
		},
	}

	fileName, err := prepareData(t)
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := Open(fileName, 0666, &Options{
				ReadOnly:        tc.readonly,
				PreLoadFreelist: tc.preLoadFreePage,
			})
			require.NoError(t, err)

			assert.Equal(t, tc.expectedFreePagesLoaded, db.freelist != nil)

			assert.NoError(t, db.Close())
		})
	}
}

func TestMmapFallbackReadWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	db, err := Open(path, 0600, nil)
	require.NoError(t, err)
	defer db.Close()

	sz := db.datasz
	require.NoError(t, db.munmap())
	require.NoError(t, mmapFallback(db, sz))
	db.meta0 = db.page(0).Meta()
	db.meta1 = db.page(1).Meta()

	require.True(t, db.mmapFallback)
	require.NoError(t, db.Update(func(tx *Tx) error {
		bucket, err := tx.CreateBucket([]byte("widgets"))
		if err != nil {
			return err
		}
		return bucket.Put([]byte("key"), []byte("value"))
	}))

	require.NoError(t, db.View(func(tx *Tx) error {
		bucket := tx.Bucket([]byte("widgets"))
		require.NotNil(t, bucket)
		require.Equal(t, []byte("value"), bucket.Get([]byte("key")))
		return nil
	}))
}

func TestMethodPage(t *testing.T) {
	testCases := []struct {
		name            string
		readonly        bool
		preLoadFreePage bool
		expectedError   error
	}{
		{
			name:            "write mode",
			readonly:        false,
			preLoadFreePage: false,
			expectedError:   nil,
		},
		{
			name:            "readonly mode with preloading free pages",
			readonly:        true,
			preLoadFreePage: true,
			expectedError:   nil,
		},
		{
			name:            "readonly mode without preloading free pages",
			readonly:        true,
			preLoadFreePage: false,
			expectedError:   errors.ErrFreePagesNotLoaded,
		},
	}

	fileName, err := prepareData(t)
	require.NoError(t, err)

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db, err := Open(fileName, 0666, &Options{
				ReadOnly:        tc.readonly,
				PreLoadFreelist: tc.preLoadFreePage,
			})
			require.NoError(t, err)
			defer db.Close()

			tx, err := db.Begin(!tc.readonly)
			require.NoError(t, err)

			_, err = tx.Page(0)
			require.Equal(t, tc.expectedError, err)

			if tc.readonly {
				require.NoError(t, tx.Rollback())
			} else {
				require.NoError(t, tx.Commit())
			}

			require.NoError(t, db.Close())
		})
	}
}

func prepareData(t *testing.T) (string, error) {
	fileName := filepath.Join(t.TempDir(), "db")
	db, err := Open(fileName, 0666, nil)
	if err != nil {
		return "", err
	}
	if err := db.Close(); err != nil {
		return "", err
	}

	return fileName, nil
}
