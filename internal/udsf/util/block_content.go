package util

import (
	"bytes"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

// DefaultBlockContentType is used when a client uploads a block without a Content-Type.
const DefaultBlockContentType = "application/octet-stream"

// BlockContent is the raw content of a Block together with its media type.
// Blocks can be of any type (openapi: "*/*"), so the bytes are never interpreted.
type BlockContent struct {
	ContentType string
	Data        []byte
}

// Marshal serializes the block for storage in Redis: "<content-type>\n<raw bytes>".
// Media types never contain a newline, so the first newline separates the header.
func (b BlockContent) Marshal() []byte {
	ct := b.ContentType
	if ct == "" {
		ct = DefaultBlockContentType
	}
	out := make([]byte, 0, len(ct)+1+len(b.Data))
	out = append(out, ct...)
	out = append(out, '\n')
	return append(out, b.Data...)
}

// UnmarshalBlockContent parses a value written by BlockContent.Marshal.
func UnmarshalBlockContent(stored string) (BlockContent, error) {
	i := bytes.IndexByte([]byte(stored), '\n')
	if i < 0 {
		return BlockContent{}, errors.New("stored block has no content-type header")
	}
	return BlockContent{ContentType: stored[:i], Data: []byte(stored[i+1:])}, nil
}

// BlockPart is one block of a multipart/parallel block list.
type BlockPart struct {
	ID      string // sent as the part's Content-Id
	Content BlockContent
}

// BlockListMultipart is the body of a GetBlockList response (multipart/parallel).
type BlockListMultipart struct {
	Parts []BlockPart
}

// writeBlockContent writes a single block with its own media type.
func writeBlockContent(w http.ResponseWriter, status int, b *BlockContent) error {
	ct := b.ContentType
	if ct == "" {
		ct = DefaultBlockContentType
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(status)
	_, err := w.Write(b.Data)
	return err
}

// writeBlockListMultipart writes the blocks as multipart/parallel, one part per block,
// each with the mandatory Content-Id and Content-Transfer-Encoding headers.
func writeBlockListMultipart(w http.ResponseWriter, status int, l *BlockListMultipart) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, p := range l.Parts {
		ct := p.Content.ContentType
		if ct == "" {
			ct = DefaultBlockContentType
		}
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", ct)
		h.Set("Content-Id", p.ID)
		h.Set("Content-Transfer-Encoding", "binary")
		pw, err := mw.CreatePart(h)
		if err != nil {
			return fmt.Errorf("create part %q: %w", p.ID, err)
		}
		if _, err := pw.Write(p.Content.Data); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "multipart/parallel; boundary="+mw.Boundary())
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}
