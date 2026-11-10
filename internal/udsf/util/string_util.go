package util

import "strings"

const (
	UDSF     = "udsf"
	SPERATOR = ":"
)

// create Block Key
func CreateBlockKey(realmIdParam, storageIdParam, recordIdParam, blockIdParam string) string {
	var sb strings.Builder
	sb.WriteString(UDSF)
	sb.WriteString(SPERATOR)
	sb.WriteString(realmIdParam)
	sb.WriteString(SPERATOR)
	sb.WriteString(storageIdParam)
	sb.WriteString(SPERATOR)
	sb.WriteString(recordIdParam)
	sb.WriteString(SPERATOR)
	sb.WriteString(blockIdParam)
	return sb.String()
}

// create record key
func CreateRecordKey(realmIdParam, storageIdParam, recordIdParam string) string {
	var sb strings.Builder
	sb.WriteString(UDSF)
	sb.WriteString(SPERATOR)
	sb.WriteString(realmIdParam)
	sb.WriteString(SPERATOR)
	sb.WriteString(storageIdParam)
	sb.WriteString(SPERATOR)
	sb.WriteString(recordIdParam)
	return sb.String()
}

// BlockIndexPrefix prefixes the key of a record's block-id index. It differs from UDSF so the
// index key is never matched by CreateRecordBlockPattern.
const BlockIndexPrefix = "udsfidx"

// CreateBlockIndexKey returns the key of the Redis set holding the ids of a record's blocks.
// It lets a record's blocks be listed and deleted without scanning the keyspace.
func CreateBlockIndexKey(realmIdParam, storageIdParam, recordIdParam string) string {
	return BlockIndexPrefix + SPERATOR + realmIdParam + SPERATOR + storageIdParam + SPERATOR + recordIdParam
}

// CreateRecordBlockPattern returns a Redis glob pattern matching every block key of a record.
// Glob metacharacters in the ids are escaped so they match literally.
func CreateRecordBlockPattern(realmIdParam, storageIdParam, recordIdParam string) string {
	esc := strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`, "]", `\]`)
	var sb strings.Builder
	sb.WriteString(UDSF)
	for _, id := range []string{realmIdParam, storageIdParam, recordIdParam} {
		sb.WriteString(SPERATOR)
		sb.WriteString(esc.Replace(id))
	}
	sb.WriteString(SPERATOR)
	sb.WriteString("*")
	return sb.String()
}
