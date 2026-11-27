package d3d11

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/egoist/mygo/internal/gpu"
)

// Emit the Windows compiler's output for the exact class combinations of the
// outlined-text experiment. Only the development overlay builds this file.
func TestEmitDanmakuShaders(t *testing.T) {
	pair := uint32(1<<gpu.ClassGlyph | 1<<gpu.ClassSubpixel)
	for _, cl := range []uint32{pair, pair | 1<<gpu.ClassImage} {
		vs, ps, err := compileClass(cl)
		if err != nil {
			t.Fatal(err)
		}
		for _, stage := range []struct {
			name string
			data []byte
		}{{"vs", vs}, {"ps", ps}} {
			encoded := base64.StdEncoding.EncodeToString(stage.data)
			t.Logf("SHADERHASH %d %s %d %x", cl, stage.name, len(stage.data), sha256.Sum256(stage.data))
			for offset := 0; offset < len(encoded); offset += 768 {
				t.Logf("SHADERDATA %d %s %d %s", cl, stage.name, offset, encoded[offset:min(offset+768, len(encoded))])
			}
		}
	}
}
