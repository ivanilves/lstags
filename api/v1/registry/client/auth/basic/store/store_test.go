package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var examples = []string{"localhost:5000 foo:bar", "quay.io quser:qpass"}

func TestLoadAllValid(t *testing.T) {
	var store Store

	err := store.LoadAll(examples)

	assert.NoError(t, err)
}

func TestLoadAllInvalid(t *testing.T) {
	var store Store

	assert.Error(t, store.LoadAll([]string{""}))
	assert.Error(t, store.LoadAll([]string{"us.gcr.io"}))
	assert.Error(t, store.LoadAll([]string{"us.gcr.io forgotsomething"}))
	assert.Error(t, store.LoadAll([]string{"quay.io quser:"}))
	assert.Error(t, store.LoadAll([]string{" foo:bar"}))
}

func TestGet(t *testing.T) {
	var store Store

	store.LoadAll(examples)

	assert.NotNil(t, store.GetByHostname("localhost:5000"))
	assert.NotNil(t, store.GetByHostname("quay.io"))
	assert.Nil(t, store.GetByHostname("eu.gcr.io"))

	assert.NotNil(t, store.GetByURL("http://localhost:5000"))
	assert.NotNil(t, store.GetByURL("https://quay.io"))
	assert.Nil(t, store.GetByURL("https://eu.gcr.io"))
}

func TestGetValues(t *testing.T) {
	var store Store

	store.LoadAll(examples)

	login1 := store.GetByHostname("localhost:5000")
	login2 := store.GetByHostname("quay.io")

	assert.Equal(t, login1.Username, "foo")
	assert.Equal(t, login1.Password, "bar")
	assert.Equal(t, login2.Username, "quser")
	assert.Equal(t, login2.Password, "qpass")
}
