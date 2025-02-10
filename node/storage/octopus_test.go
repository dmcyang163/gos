package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddFile(t *testing.T) {
	octopus := NewOctopus()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "testfile")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("Hello, IPFS!")
	assert.NoError(t, err)
	tmpFile.Close()

	// Test AddFile
	cid, err := octopus.AddFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, cid)
}

func TestGetFile(t *testing.T) {
	octopus := NewOctopus()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "testfile")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("Hello, IPFS!")
	assert.NoError(t, err)
	tmpFile.Close()

	// Add the file to IPFS
	cid, err := octopus.AddFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, cid)

	// Test GetFile
	content, err := octopus.GetFile(cid)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, IPFS!\n", content)
}

func TestUpdateFile(t *testing.T) {
	octopus := NewOctopus()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "testfile")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("Hello, IPFS!")
	assert.NoError(t, err)
	tmpFile.Close()

	// Add the file to IPFS
	cid, err := octopus.AddFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, cid)

	// Create a new temporary file for updating
	newTmpFile, err := os.CreateTemp("", "newtestfile")
	assert.NoError(t, err)
	defer os.Remove(newTmpFile.Name())

	_, err = newTmpFile.WriteString("Updated content")
	assert.NoError(t, err)
	newTmpFile.Close()

	// Test UpdateFile
	newCid, err := octopus.UpdateFile(cid, newTmpFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, newCid)

	// Verify the updated content
	content, err := octopus.GetFile(newCid)
	assert.NoError(t, err)
	assert.Equal(t, "Updated content\n", content)
}

func TestDeleteFile(t *testing.T) {
	octopus := NewOctopus()

	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "testfile")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("Hello, IPFS!")
	assert.NoError(t, err)
	tmpFile.Close()

	// Add the file to IPFS
	cid, err := octopus.AddFile(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, cid)

	// Test DeleteFile
	err = octopus.DeleteFile(cid)
	assert.NoError(t, err)

	// Verify the file is deleted
	_, err = octopus.GetFile(cid)
	assert.Error(t, err)
}
