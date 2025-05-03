package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
//	"log"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestCreateEstate(t *testing.T) {
	e := echo.New()
	initDB()
	defer db.Close()

	requestBody := `{"width": 100, "length": 200}`
	req := httptest.NewRequest(http.MethodPost, "/estate", bytes.NewReader([]byte(requestBody)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	if assert.NoError(t, createEstate(c)) {
		assert.Equal(t, http.StatusOK, res.Code)
		var response map[string]string
		json.Unmarshal(res.Body.Bytes(), &response)
		assert.Contains(t, response, "id")
	}
}

func TestAddTree(t *testing.T) {
	e := echo.New()
	initDB()
	defer db.Close()

	estateID := "03a93030-b572-4c1d-a7f2-ce6baa87c962" // Pastikan estate ini ada di database untuk pengujian
	requestBody := `{"x": 10, "y": 10, "height": 30}`
	req := httptest.NewRequest(http.MethodPost, "/estate/"+estateID+"/tree", bytes.NewReader([]byte(requestBody)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)
	c.SetParamNames("id")
	c.SetParamValues(estateID)

	if assert.NoError(t, addTree(c)) {
		assert.Equal(t, http.StatusOK, res.Code)
	//	var response map[string]string
	//	json.Unmarshal(res.Body.Bytes(), &response)
	//	assert.Contains(t, response, "id")
	}
}

func TestGetEstateStats(t *testing.T) {
	e := echo.New()
	initDB()
	defer db.Close()

	estateID := "b67a3fc2-5f3d-41ed-bfe3-ecc7f8f03f9a" // Pastikan estate ini ada di database dengan beberapa pohon
	req := httptest.NewRequest(http.MethodGet, "/estate/"+estateID+"/stats", nil)
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)
	c.SetParamNames("id")
	c.SetParamValues(estateID)

	if assert.NoError(t, getEstateStats(c)) {
		assert.Equal(t, http.StatusOK, res.Code)
		var response map[string]interface{}
		json.Unmarshal(res.Body.Bytes(), &response)
		assert.Contains(t, response, "tree_count")
		assert.Contains(t, response, "max_height")
		assert.Contains(t, response, "min_height")
		assert.Contains(t, response, "median_height")
	}
}

func TestGetDronePlan(t *testing.T) {
	e := echo.New()
	initDB()
	defer db.Close()

	estateID := "b67a3fc2-5f3d-41ed-bfe3-ecc7f8f03f9a" // Pastikan estate ini ada di database
	req := httptest.NewRequest(http.MethodGet, "/estate/"+estateID+"/drone-plan", nil)
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)
	c.SetParamNames("id")
	c.SetParamValues(estateID)

	if assert.NoError(t, GetDronePlan(c)) {
		assert.Equal(t, http.StatusOK, res.Code)
		var response map[string]interface{}
		json.Unmarshal(res.Body.Bytes(), &response)
		assert.Contains(t, response, "distance")
	}
}
