package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWarpResponse_WithNoError(t *testing.T) {
	recorder := httptest.NewRecorder()
	data := "test data"
	warpResponse(recorder, http.StatusOK, data, nil)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"data":"test data"`)
	assert.NotContains(t, recorder.Body.String(), "error")
}

func TestWarpResponse_WithError(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := errors.New("test error")
	warpResponse(recorder, http.StatusInternalServerError, nil, err)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"error":"test error"`)
	assert.NotContains(t, recorder.Body.String(), "data")
}

func TestWarpResponse_WithDataAndError(t *testing.T) {
	recorder := httptest.NewRecorder()
	data := "test data"
	err := errors.New("test error")
	warpResponse(recorder, http.StatusOK, data, err)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"data":"test data"`)
	assert.Contains(t, recorder.Body.String(), `"error":"test error"`)
}
