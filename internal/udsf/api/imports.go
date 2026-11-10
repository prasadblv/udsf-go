package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"udsf-go/internal/udsf/model"
	"udsf-go/internal/udsf/route"
	"udsf-go/internal/udsf/util"
)

type Route = route.Route
type Routes = route.Routes

type (
	ClientId                 = model.ClientId
	CountExpression          = model.CountExpression
	ExtendedProblemDetails   = model.ExtendedProblemDetails
	GetBlockList200Response  = model.GetBlockList200Response
	MetaSchema               = model.MetaSchema
	NotificationSubscription = model.NotificationSubscription
	NullValue                = model.NullValue
	PatchItem                = model.PatchItem
	PatchResult              = model.PatchResult
	ProblemDetails           = model.ProblemDetails
	Record                   = model.Record
	RecordIdList             = model.RecordIdList
	RecordMeta               = model.RecordMeta
	RecordSearchResult       = model.RecordSearchResult
	RetrieveRecords          = model.RetrieveRecords
	SearchExpression         = model.SearchExpression
	Timer                    = model.Timer
	TimerIdList              = model.TimerIdList
)

type (
	BlockContent  = util.BlockContent
	ErrorHandler  = util.ErrorHandler
	ImplResponse  = util.ImplResponse
	ParsingError  = util.ParsingError
	RequiredError = util.RequiredError
)

func Response(code int, body interface{}) ImplResponse {
	return util.Response(code, body)
}

func DefaultErrorHandler(w http.ResponseWriter, r *http.Request, err error, result *ImplResponse) {
	util.DefaultErrorHandler(w, r, err, result)
}

func EncodeJSONResponse(value interface{}, status *int, w http.ResponseWriter) error {
	return util.EncodeJSONResponse(value, status, w)
}

func parseQuery(rawQuery string) (url.Values, error) {
	return util.ParseQuery(rawQuery)
}

func parseBool(value string) (bool, error) {
	return util.ParseBool(value)
}

func parseBoolParameter(value string, operation util.Operation[bool]) (bool, error) {
	return util.ParseBoolParameter(value, operation)
}

func parseInt32(value string) (int32, error) {
	return util.ParseInt32(value)
}

func parseNumericParameter[T util.Number](value string, operation util.Operation[T], constraints ...util.Constraint[T]) (T, error) {
	return util.ParseNumericParameter(value, operation, constraints...)
}

func WithParse[T util.Number | string | bool](parse util.ParseString[T]) util.Operation[T] {
	return util.WithParse(parse)
}

func WithMinimum[T util.Number](expected T) util.Constraint[T] {
	return util.WithMinimum(expected)
}

func AssertMetaSchemaConstraints(value MetaSchema) error {
	return model.AssertMetaSchemaConstraints(value)
}

func AssertMetaSchemaRequired(value MetaSchema) error {
	return model.AssertMetaSchemaRequired(value)
}

func AssertNotificationSubscriptionConstraints(value NotificationSubscription) error {
	return model.AssertNotificationSubscriptionConstraints(value)
}

func AssertNotificationSubscriptionRequired(value NotificationSubscription) error {
	return model.AssertNotificationSubscriptionRequired(value)
}

func AssertPatchItemRequired(value PatchItem) error {
	return model.AssertPatchItemRequired(value)
}

func AssertTimerConstraints(value Timer) error {
	return model.AssertTimerConstraints(value)
}

func AssertTimerRequired(value Timer) error {
	return model.AssertTimerRequired(value)
}

// parseJSONParam decodes a JSON-encoded parameter value into dst.
func parseJSONParam(raw string, dst interface{}) error {
	return json.Unmarshal([]byte(raw), dst)
}

// parseBlocksParam reads the "blocks" parts of a multipart/form-data request. Each part becomes a
// util.BlockPart: the id is the part's Content-Id header (angle brackets stripped), falling back to
// the form filename, and the content type is the part's Content-Type (empty if absent).
// A part without an id is returned with an empty ID so the service can reject it.
// Only file-style parts (those with a filename) are visible to Go's multipart form parser.
func parseBlocksParam(r *http.Request) ([]interface{}, error) {
	blocks := []interface{}{}
	if r.MultipartForm == nil {
		return blocks, nil
	}
	for _, fh := range r.MultipartForm.File["blocks"] {
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		id := strings.Trim(strings.TrimSpace(fh.Header.Get("Content-Id")), "<>")
		if id == "" {
			id = fh.Filename
		}
		blocks = append(blocks, util.BlockPart{
			ID:      id,
			Content: util.BlockContent{ContentType: fh.Header.Get("Content-Type"), Data: data},
		})
	}
	return blocks, nil
}
