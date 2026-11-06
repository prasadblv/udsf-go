/*
 * Nudsf_DataRepository
 *
 * Nudsf Data Repository Service.   © 2024, 3GPP Organizational Partners (ARIB, ATIS, CCSA, ETSI, TSDSI, TTA, TTC).   All rights reserved.
 *
 * API version: 1.2.0-alpha.6
 */

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"udsf-go/internal/database"
	"udsf-go/internal/udsf/util"
)

// BlockCRUDAPIService is a service that implements the logic for the BlockCRUDAPIServicer.
// Blocks are stored in Redis as JSON under the key built by util.CreateBlockKey.
type BlockCRUDAPIService struct {
	db database.Service
}

// NewBlockCRUDAPIService creates an api service backed by the given database.
func NewBlockCRUDAPIService(db database.Service) *BlockCRUDAPIService {
	return &BlockCRUDAPIService{db: db}
}

func problem(status int, title string, err error) ImplResponse {
	pd := ProblemDetails{Status: int32(status), Title: title}
	if err != nil {
		pd.Detail = err.Error()
	}
	return Response(status, pd)
}

// GetBlockList - Record's Blocks access
func (s *BlockCRUDAPIService) GetBlockList(ctx context.Context, realmId string, storageId string, recordId string, supportedFeatures string) (ImplResponse, error) {
	pattern := util.CreateRecordBlockPattern(realmId, storageId, recordId)
	util.LogDebug("GetBlockList pattern=%s", pattern)

	keys, err := s.db.Scan(ctx, pattern)
	if err != nil {
		util.LogError("scan blocks failed pattern=%s: %v", pattern, err)
		return problem(http.StatusInternalServerError, "Failed to list blocks", err), nil
	}
	sort.Strings(keys)

	vals, err := s.db.GetMany(ctx, keys)
	if err != nil {
		util.LogError("read blocks failed pattern=%s: %v", pattern, err)
		return problem(http.StatusInternalServerError, "Failed to read blocks", err), nil
	}

	blocks := make([]interface{}, 0, len(vals))
	for _, v := range vals {
		blocks = append(blocks, json.RawMessage(v))
	}
	util.LogInfo("listed %d blocks pattern=%s", len(blocks), pattern)
	return Response(http.StatusOK, GetBlockList200Response{Blocks: blocks}), nil
}

// GetBlock - Retrieve a specific Block
func (s *BlockCRUDAPIService) GetBlock(ctx context.Context, realmId string, storageId string, recordId string, blockId string, ifNoneMatch string, ifModifiedSince string, supportedFeatures string) (ImplResponse, error) {
	key := util.CreateBlockKey(realmId, storageId, recordId, blockId)
	util.LogDebug("GetBlock key=%s", key)

	val, err := s.db.Get(ctx, key)
	if errors.Is(err, database.ErrKeyNotFound) {
		util.LogInfo("block not found key=%s", key)
		return problem(http.StatusNotFound, "Block not found", nil), nil
	}
	if err != nil {
		util.LogError("get block failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to read block", err), nil
	}

	util.LogDebug("block retrieved key=%s", key)
	return Response(http.StatusOK, json.RawMessage(val)), nil
}

// CreateOrModifyBlock - Create or Update a specific Block in a Record.
func (s *BlockCRUDAPIService) CreateOrModifyBlock(ctx context.Context, realmId string, storageId string, recordId string, blockId string, body *interface{}, getPrevious bool, ifNoneMatch string, ifMatch string, supportedFeatures string) (ImplResponse, error) {
	key := util.CreateBlockKey(realmId, storageId, recordId, blockId)
	util.LogDebug("CreateOrModifyBlock key=%s", key)

	if body == nil {
		return problem(http.StatusBadRequest, "Missing block body", nil), nil
	}
	data, err := json.Marshal(*body)
	if err != nil {
		return problem(http.StatusBadRequest, "Invalid block body", err), nil
	}

	exists, err := s.db.Exists(ctx, key)
	if err != nil {
		util.LogError("exists check failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to check block", err), nil
	}

	if !exists {
		err = s.db.Create(ctx, key, data, 0)
		if err == nil {
			util.LogInfo("block created key=%s", key)
			return Response(http.StatusCreated, *body), nil
		}
		if !errors.Is(err, database.ErrKeyExists) {
			return problem(http.StatusInternalServerError, "Failed to create block", err), nil
		}
	}

	if err := s.db.Update(ctx, key, data); err != nil {
		if errors.Is(err, database.ErrKeyNotFound) {
			return problem(http.StatusNotFound, "Block not found", nil), nil
		}
		return problem(http.StatusInternalServerError, "Failed to update block", err), nil
	}
	util.LogInfo("block updated key=%s", key)
	return Response(http.StatusOK, *body), nil
}

// DeleteBlock - Delete a specific Block. Then update the Record
func (s *BlockCRUDAPIService) DeleteBlock(ctx context.Context, realmId string, storageId string, recordId string, blockId string, getPrevious bool, ifMatch string, supportedFeatures string) (ImplResponse, error) {
	key := util.CreateBlockKey(realmId, storageId, recordId, blockId)
	util.LogDebug("DeleteBlock key=%s", key)

	err := s.db.Delete(ctx, key)
	if errors.Is(err, database.ErrKeyNotFound) {
		util.LogWarn("delete: block not found key=%s", key)
		return problem(http.StatusNotFound, "Block not found", nil), nil
	}
	if err != nil {
		util.LogError("delete block failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to delete block", err), nil
	}
	util.LogInfo("block deleted key=%s", key)
	return Response(http.StatusNoContent, nil), nil
}
