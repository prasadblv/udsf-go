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
	"fmt"
	"net/http"
	"sort"
	"time"

	"udsf-go/internal/database"
	"udsf-go/internal/udsf/util"
)

// RecordCRUDAPIService implements the Record CRUD operations.
// A record's meta is stored in Redis as JSON under util.CreateRecordKey; its blocks live under
// separate keys (util.CreateBlockKey) managed by the Block CRUD API.
type RecordCRUDAPIService struct {
	db database.Service
}

// NewRecordCRUDAPIService creates an api service backed by the given database.
func NewRecordCRUDAPIService(db database.Service) *RecordCRUDAPIService {
	return &RecordCRUDAPIService{db: db}
}

// storedRecord is the JSON persisted for a record. Blocks are not part of it.
type storedRecord struct {
	Meta RecordMeta `json:"meta"`
}

func notImplemented(name string) (ImplResponse, error) {
	return Response(http.StatusNotImplemented, nil), errors.New(name + " method not implemented")
}

// blockIDs returns the sorted ids of a record's blocks from its block index. Reading the index
// is a single O(blocks) call; scanning the keyspace instead cost O(all keys) round trips per request.
func (s *RecordCRUDAPIService) blockIDs(ctx context.Context, realmId, storageId, recordId string) ([]string, error) {
	ids, err := s.db.SetMembers(ctx, util.CreateBlockIndexKey(realmId, storageId, recordId))
	if err != nil {
		return nil, err
	}
	sort.Strings(ids)
	return ids, nil
}

// toBlockParts converts the opaque block list from the controller into block parts,
// rejecting unknown types, empty ids and duplicate ids.
func toBlockParts(blocks []interface{}) ([]util.BlockPart, error) {
	parts := make([]util.BlockPart, 0, len(blocks))
	seen := make(map[string]struct{}, len(blocks))
	for i, b := range blocks {
		p, ok := b.(util.BlockPart)
		if !ok {
			return nil, fmt.Errorf("block %d has unsupported type %T", i, b)
		}
		if p.ID == "" {
			return nil, fmt.Errorf("block %d has no Content-Id", i)
		}
		if _, dup := seen[p.ID]; dup {
			return nil, fmt.Errorf("duplicate block id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
		parts = append(parts, p)
	}
	return parts, nil
}

// storeBlocks upserts the blocks of a record. It returns the keys it newly created so that
// the caller can roll them back.
func (s *RecordCRUDAPIService) storeBlocks(ctx context.Context, realmId, storageId, recordId string, parts []util.BlockPart) (created []string, err error) {
	if len(parts) == 0 {
		return nil, nil
	}
	ids := make([]string, len(parts))
	for i, p := range parts {
		ids[i] = p.ID
	}
	// Index first so a crash never leaves an unindexed block; a stale id is harmless.
	if err := s.db.SetAdd(ctx, util.CreateBlockIndexKey(realmId, storageId, recordId), ids...); err != nil {
		return nil, err
	}
	for _, p := range parts {
		bkey := util.CreateBlockKey(realmId, storageId, recordId, p.ID)
		data := p.Content.Marshal()
		err := s.db.Create(ctx, bkey, data, 0)
		switch {
		case err == nil:
			created = append(created, bkey)
		case errors.Is(err, database.ErrKeyExists):
			if err := s.db.Update(ctx, bkey, data); err != nil {
				return created, err
			}
		default:
			return created, err
		}
	}
	return created, nil
}

// loadRecord reads and decodes a record. found is false when it does not exist.
func (s *RecordCRUDAPIService) loadRecord(ctx context.Context, key string) (rec storedRecord, found bool, err error) {
	val, err := s.db.Get(ctx, key)
	if errors.Is(err, database.ErrKeyNotFound) {
		return rec, false, nil
	}
	if err != nil {
		return rec, false, err
	}
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return rec, true, err
	}
	return rec, true, nil
}

// SearchRecord - Records search with get
func (s *RecordCRUDAPIService) SearchRecord(ctx context.Context, realmId string, storageId string, limitRange int32, filter SearchExpression, countIndicator bool, supportedFeatures string, retrieveRecords RetrieveRecords, maxPayloadSize int32, tagCountFilter CountExpression) (ImplResponse, error) {
	return notImplemented("SearchRecord")
}

// BulkDeleteRecords - Bulk Deletion of Records
func (s *RecordCRUDAPIService) BulkDeleteRecords(ctx context.Context, realmId string, storageId string, filter SearchExpression, supportedFeatures string) (ImplResponse, error) {
	return notImplemented("BulkDeleteRecords")
}

// GetRecord - Record access. The returned blocks are the ids of the record's blocks.
func (s *RecordCRUDAPIService) GetRecord(ctx context.Context, realmId string, storageId string, recordId string, ifNoneMatch string, ifModifiedSince string, supportedFeatures string) (ImplResponse, error) {
	key := util.CreateRecordKey(realmId, storageId, recordId)
	util.LogDebug("GetRecord key=%s", key)

	rec, found, err := s.loadRecord(ctx, key)
	if err != nil {
		util.LogError("get record failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to read record", err), nil
	}
	if !found {
		util.LogInfo("record not found key=%s", key)
		return problem(http.StatusNotFound, "Record not found", nil), nil
	}

	ids, err := s.blockIDs(ctx, realmId, storageId, recordId)
	if err != nil {
		util.LogError("list record blocks failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to list blocks", err), nil
	}
	blocks := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		blocks = append(blocks, id)
	}
	return Response(http.StatusOK, Record{Meta: rec.Meta, Blocks: blocks}), nil
}

// CreateOrModifyRecord - Create/Modify Record.
// Creation returns 201. An update returns 200 with the previous record when get-previous is set,
// otherwise 204. If-None-Match: * fails with 412 when the record already exists.
// The record's TTL is applied on creation only. Blocks supplied with the request are upserted
// (existing blocks that are not part of the request are left untouched).
func (s *RecordCRUDAPIService) CreateOrModifyRecord(ctx context.Context, realmId string, storageId string, recordId string, meta RecordMeta, ifNoneMatch string, ifMatch string, getPrevious bool, supportedFeatures string, blocks []interface{}) (ImplResponse, error) {
	key := util.CreateRecordKey(realmId, storageId, recordId)
	util.LogDebug("CreateOrModifyRecord key=%s", key)

	var ttl time.Duration
	if !meta.Ttl.IsZero() {
		ttl = time.Until(meta.Ttl)
		if ttl <= 0 {
			return problem(http.StatusBadRequest, "Record ttl is in the past", nil), nil
		}
	}
	parts, err := toBlockParts(blocks)
	if err != nil {
		return problem(http.StatusBadRequest, "Invalid blocks", err), nil
	}
	data, err := json.Marshal(storedRecord{Meta: meta})
	if err != nil {
		return problem(http.StatusInternalServerError, "Failed to encode record", err), nil
	}

	err = s.db.Create(ctx, key, data, ttl)
	if err == nil {
		if created, err := s.storeBlocks(ctx, realmId, storageId, recordId, parts); err != nil {
			// Roll back so a failed create does not leave a half-written record behind.
			_ = s.db.DeleteMany(ctx, append(created, key, util.CreateBlockIndexKey(realmId, storageId, recordId))...)
			util.LogError("store blocks failed key=%s: %v", key, err)
			return problem(http.StatusInternalServerError, "Failed to store blocks", err), nil
		}
		util.LogInfo("record created key=%s blocks=%d", key, len(parts))
		return Response(http.StatusCreated, nil), nil
	}
	if !errors.Is(err, database.ErrKeyExists) {
		util.LogError("create record failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to create record", err), nil
	}

	// Already exists.
	if ifNoneMatch == "*" {
		return problem(http.StatusPreconditionFailed, "Record already exists", nil), nil
	}
	previous, found, err := s.loadRecord(ctx, key)
	if err != nil {
		util.LogError("read record failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to read record", err), nil
	}
	if !found {
		// Deleted between Create and Get; callers can retry.
		return problem(http.StatusConflict, "Record changed concurrently", nil), nil
	}
	if err := s.db.Update(ctx, key, data); err != nil {
		if errors.Is(err, database.ErrKeyNotFound) {
			return problem(http.StatusConflict, "Record changed concurrently", nil), nil
		}
		util.LogError("update record failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to update record", err), nil
	}
	if _, err := s.storeBlocks(ctx, realmId, storageId, recordId, parts); err != nil {
		util.LogError("store blocks failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to store blocks", err), nil
	}
	util.LogInfo("record updated key=%s blocks=%d", key, len(parts))

	if getPrevious {
		return Response(http.StatusOK, Record{Meta: previous.Meta}), nil
	}
	return Response(http.StatusNoContent, nil), nil
}

// DeleteRecord - Delete a Record with an user provided RecordId, together with its blocks.
func (s *RecordCRUDAPIService) DeleteRecord(ctx context.Context, realmId string, storageId string, recordId string, ifMatch string, getPrevious bool, supportedFeatures string) (ImplResponse, error) {
	key := util.CreateRecordKey(realmId, storageId, recordId)
	util.LogDebug("DeleteRecord key=%s", key)

	previous, found, err := s.loadRecord(ctx, key)
	if err != nil {
		util.LogError("read record failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to read record", err), nil
	}
	if !found {
		util.LogWarn("delete: record not found key=%s", key)
		return problem(http.StatusNotFound, "Record not found", nil), nil
	}

	if err := s.db.Delete(ctx, key); err != nil && !errors.Is(err, database.ErrKeyNotFound) {
		util.LogError("delete record failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to delete record", err), nil
	}

	ids, err := s.blockIDs(ctx, realmId, storageId, recordId)
	if err != nil {
		util.LogError("list record blocks failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to delete record blocks", err), nil
	}
	keys := make([]string, 0, len(ids)+1)
	for _, id := range ids {
		keys = append(keys, util.CreateBlockKey(realmId, storageId, recordId, id))
	}
	keys = append(keys, util.CreateBlockIndexKey(realmId, storageId, recordId))
	if err := s.db.DeleteMany(ctx, keys...); err != nil {
		util.LogError("delete record blocks failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to delete record blocks", err), nil
	}
	util.LogInfo("record deleted key=%s blocks=%d", key, len(ids))

	if getPrevious {
		return Response(http.StatusOK, Record{Meta: previous.Meta}), nil
	}
	return Response(http.StatusNoContent, nil), nil
}

// PatchRecord - Partial update Record.
// Not implemented: the generated PatchOperation model is an empty struct, so patch ops cannot be read.
func (s *RecordCRUDAPIService) PatchRecord(ctx context.Context, realmId string, storageId string, recordId string, patch []PatchItem, ifMatch string, supportedFeatures string, blocks []interface{}) (ImplResponse, error) {
	return notImplemented("PatchRecord")
}

// GetMeta - Record's meta access
func (s *RecordCRUDAPIService) GetMeta(ctx context.Context, realmId string, storageId string, recordId string, ifNoneMatch string, ifModifiedSince string, supportedFeatures string) (ImplResponse, error) {
	key := util.CreateRecordKey(realmId, storageId, recordId)
	util.LogDebug("GetMeta key=%s", key)

	rec, found, err := s.loadRecord(ctx, key)
	if err != nil {
		util.LogError("get meta failed key=%s: %v", key, err)
		return problem(http.StatusInternalServerError, "Failed to read record", err), nil
	}
	if !found {
		return problem(http.StatusNotFound, "Record not found", nil), nil
	}
	return Response(http.StatusOK, rec.Meta), nil
}

// UpdateMeta - Record's meta update. Not implemented, see PatchRecord.
func (s *RecordCRUDAPIService) UpdateMeta(ctx context.Context, realmId string, storageId string, recordId string, patchItem []PatchItem, ifMatch string, supportedFeatures string) (ImplResponse, error) {
	return notImplemented("UpdateMeta")
}
