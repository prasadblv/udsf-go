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
