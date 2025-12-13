package minecraft

import (
	"archive/zip"
	"bytes"
	"crypto/aes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sandertv/gophertunnel/minecraft/resource"
)

func EncodePack(pack *resource.Pack) ([]byte, error) {
	buf := make([]byte, pack.Len())
	off := 0
	for {
		n, err := pack.ReadAt(buf[off:], int64(off))
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		off += n
	}
	return buf, nil
}

func decryptCBF(data []byte, key []byte) ([]byte, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	shiftRegister := append(key[:16], data...) // prefill with iv + cipherdata
	tmp := make([]byte, 16)

	off := 0
	for off < len(data) {
		b.Encrypt(tmp, shiftRegister)
		data[off] ^= tmp[0]
		shiftRegister = shiftRegister[1:]
		off++
	}
	return data, nil
}

type ContentEntry struct {
	Path string `json:"path"`
	Key  string `json:"key"`
}

type Content struct {
	Content []ContentEntry `json:"content"`
}

func DecryptPack(buf []byte, key string) ([]byte, error) {
	rb := bytes.NewReader(buf)
	r, err := zip.NewReader(rb, rb.Size())
	if err != nil {
		return nil, err
	}
	zb := bytes.NewBuffer([]byte{})
	z := zip.NewWriter(zb)

	var content Content
	path := contentPath(r)

	contentsPath := "contents.json"
	if len(path) > 0 {
		contentsPath = path + contentsPath
	}

	cf, err := r.Open(contentsPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("contents.json not found in %s\n", path)
			return nil, nil
		}
		fmt.Printf("error opening contents.json: %v\n", err)
		return nil, err
	}

	cbuf, _ := io.ReadAll(cf)
	decr, _ := decryptCBF(cbuf[0x100:], []byte(key))
	decr = bytes.Split(decr, []byte("\x00"))[0] // remove trailing \x00 (example: play.galaxite.net)

	cw, _ := z.Create("contents.json")
	_, _ = cw.Write(decr)
	if err = json.Unmarshal(decr, &content); err != nil {
		return nil, err
	}

	for _, entry := range content.Content {
		filePath := entry.Path
		if len(path) > 0 {
			filePath = path + "/" + entry.Path
		}
		f, err := r.Open(filePath)
		if err != nil {
			continue
		}
		fbuf, _ := io.ReadAll(f)
		if entry.Key != "" {
			fbuf, _ = decryptCBF(fbuf, []byte(entry.Key))
		}
		fw, _ := z.Create(entry.Path)
		_, _ = fw.Write(fbuf)
	}

	_ = z.Close()
	return zb.Bytes(), nil
}

func contentPath(r *zip.Reader) string {
	for _, f := range r.File {
		if strings.EqualFold(f.Name, "contents.json") {
			return ""
		}
		if strings.HasSuffix(f.Name, "contents.json") {
			return strings.Split(f.Name, "/")[0]
		}
	}
	return ""
}
