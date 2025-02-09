package storage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewOctopus(t *testing.T) {
	octopus := NewOctopus()
	assert.NotNil(t, octopus)
	assert.IsType(t, &octopusImpl{}, octopus)
}

func TestAddFile(t *testing.T) {
	octopus := NewOctopus()
	cid, err := octopus.AddFile("testdata/testfile.txt")
	assert.NoError(t, err)
	assert.NotEmpty(t, cid)
}

func TestGetFile(t *testing.T) {
	octopus := NewOctopus()
	cid, _ := octopus.AddFile("testdata/testfile.txt")
	fmt.Printf("cid: %s\n", cid)
	fileContent, err := octopus.GetFile(cid)
	assert.NoError(t, err)
	assert.Contains(t, fileContent, "This is a test file.")
}

func TestUpdateFile(t *testing.T) {
	octopus := NewOctopus()
	cid, _ := octopus.AddFile("testdata/testfile.txt")
	newCid, err := octopus.UpdateFile(cid, "testdata/newtestfile.txt")
	fmt.Printf("newCid: %s\n", newCid)

	assert.NoError(t, err)
	assert.NotEqual(t, cid, newCid)
}

func TestDeleteFile(t *testing.T) {
	octopus := NewOctopus()
	cid, _ := octopus.AddFile("testdata/testfile.txt")
	err := octopus.DeleteFile(cid)
	assert.NoError(t, err)
	_, err = octopus.GetFile(cid)
	assert.Error(t, err)
}
