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
	"errors"
	"net/http"
)

// BlockCRUDAPIService is a service that implements the logic for the BlockCRUDAPIServicer
// This service should implement the business logic for every endpoint for the BlockCRUDAPI API.
// Include any external packages or services that will be required by this service.
type BlockCRUDAPIService struct {
}

// NewBlockCRUDAPIService creates a default api service
func NewBlockCRUDAPIService() *BlockCRUDAPIService {
	return &BlockCRUDAPIService{}
}

// GetBlockList - Record&#39;s Blocks access
func (s *BlockCRUDAPIService) GetBlockList(ctx context.Context, realmId string, storageId string, recordId string, supportedFeatures string) (ImplResponse, error) {
	// TODO - update GetBlockList with the required logic for this service method.
	// Add api_block_crud_service.go to the .openapi-generator-ignore to avoid overwriting this service implementation when updating open api generation.

	// TODO: Uncomment the next line to return response Response(200, GetBlockList200Response{}) or use other options such as http.Ok ...
	// return Response(200, GetBlockList200Response{}), nil

	// TODO: Uncomment the next line to return response Response(204, {}) or use other options such as http.Ok ...
	// return Response(204, nil),nil

	// TODO: Uncomment the next line to return response Response(400, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(400, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(401, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(401, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(403, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(403, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(404, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(404, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(406, {}) or use other options such as http.Ok ...
	// return Response(406, nil),nil

	// TODO: Uncomment the next line to return response Response(412, ExtendedProblemDetails{}) or use other options such as http.Ok ...
	// return Response(412, ExtendedProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(429, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(429, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(500, ExtendedProblemDetails{}) or use other options such as http.Ok ...
	// return Response(500, ExtendedProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(502, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(502, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(503, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(503, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(0, {}) or use other options such as http.Ok ...
	// return Response(0, nil),nil

	return Response(http.StatusOK, nil), nil
}

// GetBlock - Retrieve a specific Block
func (s *BlockCRUDAPIService) GetBlock(ctx context.Context, realmId string, storageId string, recordId string, blockId string, ifNoneMatch string, ifModifiedSince string, supportedFeatures string) (ImplResponse, error) {
	// TODO - update GetBlock with the required logic for this service method.
	// Add api_block_crud_service.go to the .openapi-generator-ignore to avoid overwriting this service implementation when updating open api generation.

	// TODO: Uncomment the next line to return response Response(200, interface{}{}) or use other options such as http.Ok ...
	// return Response(200, interface{}{}), nil

	// TODO: Uncomment the next line to return response Response(304, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(304, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(400, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(400, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(401, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(401, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(403, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(403, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(404, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(404, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(406, {}) or use other options such as http.Ok ...
	// return Response(406, nil),nil

	// TODO: Uncomment the next line to return response Response(412, ExtendedProblemDetails{}) or use other options such as http.Ok ...
	// return Response(412, ExtendedProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(429, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(429, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(500, ExtendedProblemDetails{}) or use other options such as http.Ok ...
	// return Response(500, ExtendedProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(502, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(502, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(503, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(503, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(0, {}) or use other options such as http.Ok ...
	// return Response(0, nil),nil

	return Response(http.StatusNotImplemented, nil), errors.New("GetBlock method not implemented")
}

// CreateOrModifyBlock - Create or Update a specific Block in a Record.
func (s *BlockCRUDAPIService) CreateOrModifyBlock(ctx context.Context, realmId string, storageId string, recordId string, blockId string, body *interface{}, getPrevious bool, ifNoneMatch string, ifMatch string, supportedFeatures string) (ImplResponse, error) {
	// TODO - update CreateOrModifyBlock with the required logic for this service method.
	// Add api_block_crud_service.go to the .openapi-generator-ignore to avoid overwriting this service implementation when updating open api generation.

	// TODO: Uncomment the next line to return response Response(200, interface{}{}) or use other options such as http.Ok ...
	// return Response(200, interface{}{}), nil

	// TODO: Uncomment the next line to return response Response(201, {}) or use other options such as http.Ok ...
	// return Response(201, nil),nil

	// TODO: Uncomment the next line to return response Response(204, {}) or use other options such as http.Ok ...
	// return Response(204, nil),nil

	// TODO: Uncomment the next line to return response Response(400, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(400, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(401, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(401, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(403, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(403, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(404, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(404, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(408, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(408, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(409, ExtendedProblemDetails{}) or use other options such as http.Ok ...
	// return Response(409, ExtendedProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(411, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(411, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(412, interface{}{}) or use other options such as http.Ok ...
	// return Response(412, interface{}{}), nil

	// TODO: Uncomment the next line to return response Response(413, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(413, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(415, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(415, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(429, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(429, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(500, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(500, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(502, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(502, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(503, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(503, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(0, {}) or use other options such as http.Ok ...
	// return Response(0, nil),nil

	return Response(http.StatusNotImplemented, nil), errors.New("CreateOrModifyBlock method not implemented")
}

// DeleteBlock - Delete a specific Block. Then update the Record
func (s *BlockCRUDAPIService) DeleteBlock(ctx context.Context, realmId string, storageId string, recordId string, blockId string, getPrevious bool, ifMatch string, supportedFeatures string) (ImplResponse, error) {
	// TODO - update DeleteBlock with the required logic for this service method.
	// Add api_block_crud_service.go to the .openapi-generator-ignore to avoid overwriting this service implementation when updating open api generation.

	// TODO: Uncomment the next line to return response Response(200, interface{}{}) or use other options such as http.Ok ...
	// return Response(200, interface{}{}), nil

	// TODO: Uncomment the next line to return response Response(204, {}) or use other options such as http.Ok ...
	// return Response(204, nil),nil

	// TODO: Uncomment the next line to return response Response(400, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(400, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(401, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(401, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(403, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(403, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(404, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(404, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(408, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(408, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(412, interface{}{}) or use other options such as http.Ok ...
	// return Response(412, interface{}{}), nil

	// TODO: Uncomment the next line to return response Response(429, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(429, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(500, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(500, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(502, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(502, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(503, ProblemDetails{}) or use other options such as http.Ok ...
	// return Response(503, ProblemDetails{}), nil

	// TODO: Uncomment the next line to return response Response(0, {}) or use other options such as http.Ok ...
	// return Response(0, nil),nil

	return Response(http.StatusNotImplemented, nil), errors.New("DeleteBlock method not implemented")
}
