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
	"net/http"
)

// BlockCRUDAPIRouter defines the required methods for binding the api requests to a responses for the BlockCRUDAPI
// The BlockCRUDAPIRouter implementation should parse necessary information from the http request,
// pass the data to a BlockCRUDAPIServicer to perform the required actions, then write the service results to the http response.
type BlockCRUDAPIRouter interface {
	GetBlockList(http.ResponseWriter, *http.Request)
	GetBlock(http.ResponseWriter, *http.Request)
	CreateOrModifyBlock(http.ResponseWriter, *http.Request)
	DeleteBlock(http.ResponseWriter, *http.Request)
}

// MetaSchemaCRUDAPIRouter defines the required methods for binding the api requests to a responses for the MetaSchemaCRUDAPI
// The MetaSchemaCRUDAPIRouter implementation should parse necessary information from the http request,
// pass the data to a MetaSchemaCRUDAPIServicer to perform the required actions, then write the service results to the http response.
type MetaSchemaCRUDAPIRouter interface {
	GetMetaSchema(http.ResponseWriter, *http.Request)
	CreateOrModifyMetaSchema(http.ResponseWriter, *http.Request)
	DeleteMetaSchema(http.ResponseWriter, *http.Request)
}

// NotificationSubscriptionCRUDAPIRouter defines the required methods for binding the api requests to a responses for the NotificationSubscriptionCRUDAPI
// The NotificationSubscriptionCRUDAPIRouter implementation should parse necessary information from the http request,
// pass the data to a NotificationSubscriptionCRUDAPIServicer to perform the required actions, then write the service results to the http response.
type NotificationSubscriptionCRUDAPIRouter interface {
	GetNotificationSubscription(http.ResponseWriter, *http.Request)
	CreateAndUpdateNotificationSubscription(http.ResponseWriter, *http.Request)
	DeleteNotificationSubscription(http.ResponseWriter, *http.Request)
	UpdateNotificationSubscription(http.ResponseWriter, *http.Request)
}

// NotificationSubscriptionsCRUDAPIRouter defines the required methods for binding the api requests to a responses for the NotificationSubscriptionsCRUDAPI
// The NotificationSubscriptionsCRUDAPIRouter implementation should parse necessary information from the http request,
// pass the data to a NotificationSubscriptionsCRUDAPIServicer to perform the required actions, then write the service results to the http response.
type NotificationSubscriptionsCRUDAPIRouter interface {
	GetNotificationSubscriptions(http.ResponseWriter, *http.Request)
}

// RecordCRUDAPIRouter defines the required methods for binding the api requests to a responses for the RecordCRUDAPI
// The RecordCRUDAPIRouter implementation should parse necessary information from the http request,
// pass the data to a RecordCRUDAPIServicer to perform the required actions, then write the service results to the http response.
type RecordCRUDAPIRouter interface {
	SearchRecord(http.ResponseWriter, *http.Request)
	BulkDeleteRecords(http.ResponseWriter, *http.Request)
	GetRecord(http.ResponseWriter, *http.Request)
	CreateOrModifyRecord(http.ResponseWriter, *http.Request)
	DeleteRecord(http.ResponseWriter, *http.Request)
	PatchRecord(http.ResponseWriter, *http.Request)
	GetMeta(http.ResponseWriter, *http.Request)
	UpdateMeta(http.ResponseWriter, *http.Request)
}

// BlockCRUDAPIServicer defines the api actions for the BlockCRUDAPI service
// This interface intended to stay up to date with the openapi yaml used to generate it,
// while the service implementation can be ignored with the .openapi-generator-ignore file
// and updated with the logic required for the API.
type BlockCRUDAPIServicer interface {
	GetBlockList(context.Context, string, string, string, string) (ImplResponse, error)
	GetBlock(context.Context, string, string, string, string, string, string, string) (ImplResponse, error)
	CreateOrModifyBlock(context.Context, string, string, string, string, *BlockContent, bool, string, string, string) (ImplResponse, error)
	DeleteBlock(context.Context, string, string, string, string, bool, string, string) (ImplResponse, error)
}

// MetaSchemaCRUDAPIServicer defines the api actions for the MetaSchemaCRUDAPI service
// This interface intended to stay up to date with the openapi yaml used to generate it,
// while the service implementation can be ignored with the .openapi-generator-ignore file
// and updated with the logic required for the API.
type MetaSchemaCRUDAPIServicer interface {
	GetMetaSchema(context.Context, string, string, string, string, string, string) (ImplResponse, error)
	CreateOrModifyMetaSchema(context.Context, string, string, string, MetaSchema, string, string, bool, string) (ImplResponse, error)
	DeleteMetaSchema(context.Context, string, string, string, string, bool, string) (ImplResponse, error)
}

// NotificationSubscriptionCRUDAPIServicer defines the api actions for the NotificationSubscriptionCRUDAPI service
// This interface intended to stay up to date with the openapi yaml used to generate it,
// while the service implementation can be ignored with the .openapi-generator-ignore file
// and updated with the logic required for the API.
type NotificationSubscriptionCRUDAPIServicer interface {
	GetNotificationSubscription(context.Context, string, string, string, string, string, string) (ImplResponse, error)
	CreateAndUpdateNotificationSubscription(context.Context, string, string, string, NotificationSubscription, string, string, string) (ImplResponse, error)
	DeleteNotificationSubscription(context.Context, string, string, string, ClientId, bool, string, string) (ImplResponse, error)
	UpdateNotificationSubscription(context.Context, string, string, string, []PatchItem, string, string) (ImplResponse, error)
}

// NotificationSubscriptionsCRUDAPIServicer defines the api actions for the NotificationSubscriptionsCRUDAPI service
// This interface intended to stay up to date with the openapi yaml used to generate it,
// while the service implementation can be ignored with the .openapi-generator-ignore file
// and updated with the logic required for the API.
type NotificationSubscriptionsCRUDAPIServicer interface {
	GetNotificationSubscriptions(context.Context, string, string, int32, string) (ImplResponse, error)
}

// RecordCRUDAPIServicer defines the api actions for the RecordCRUDAPI service
// This interface intended to stay up to date with the openapi yaml used to generate it,
// while the service implementation can be ignored with the .openapi-generator-ignore file
// and updated with the logic required for the API.
type RecordCRUDAPIServicer interface {
	SearchRecord(context.Context, string, string, int32, SearchExpression, bool, string, RetrieveRecords, int32, CountExpression) (ImplResponse, error)
	BulkDeleteRecords(context.Context, string, string, SearchExpression, string) (ImplResponse, error)
	GetRecord(context.Context, string, string, string, string, string, string) (ImplResponse, error)
	CreateOrModifyRecord(context.Context, string, string, string, RecordMeta, string, string, bool, string, []interface{}) (ImplResponse, error)
	DeleteRecord(context.Context, string, string, string, string, bool, string) (ImplResponse, error)
	PatchRecord(context.Context, string, string, string, []PatchItem, string, string, []interface{}) (ImplResponse, error)
	GetMeta(context.Context, string, string, string, string, string, string) (ImplResponse, error)
	UpdateMeta(context.Context, string, string, string, []PatchItem, string, string) (ImplResponse, error)
}

type TimerGetAPIServicer interface {
	GetTimer(context.Context, string, string, string, string) (ImplResponse, error)
}

type TimerSearchAPIServicer interface {
	SearchTimer(context.Context, string, string, SearchExpression, NullValue, string) (ImplResponse, error)
}

type TimerStartAPIServicer interface {
	CreateOrModifyTimer(context.Context, string, string, string, Timer, string) (ImplResponse, error)
}

type TimerStopAPIServicer interface {
	DeleteTimer(context.Context, string, string, string, string) (ImplResponse, error)
}

type TimerUpdateAPIServicer interface {
	UpdateTimer(context.Context, string, string, string, []PatchItem, string) (ImplResponse, error)
}

type TimersDeleteAPIServicer interface {
	DeleteTimers(context.Context, string, string, string, SearchExpression, NullValue) (ImplResponse, error)
}
