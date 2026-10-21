//go:build windows

package server

import (
	"encoding/hex"
	"testing"
)

func TestDecodeWindowsOutput(t *testing.T) {
	gbk, err := hex.DecodeString("b4edcef33a20c8cecef120584d4c20b8f1cabdb2bbd5fdc8b7a1a30d0a28312c3430293a3ab4edcef33a20cedeb7a8c7d0bbbbb1e0c2eba1a30d0a")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		data     []byte
		codePage uint32
		want     string
	}{
		{"中文 GBK", gbk, 936, "错误: 任务 XML 格式不正确。\r\n(1,40)::错误: 无法切换编码。\r\n"},
		{"OEM 字符", []byte{'c', 'a', 'f', 0x82}, 437, "café"},
		{"UTF-8", []byte("错误: 无法切换编码。"), 936, "错误: 无法切换编码。"},
		{"ASCII", []byte("ERROR: invalid task"), 437, "ERROR: invalid task"},
		{"空输出", nil, 936, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeWindowsOutput(tc.data, tc.codePage)
			if err != nil || got != tc.want {
				t.Fatalf("输出解码错误: got=%q want=%q err=%v", got, tc.want, err)
			}
		})
	}
	if _, err := decodeWindowsOutput([]byte{0xff}, 999999); err == nil {
		t.Fatal("不支持的代码页应返回解码错误")
	}
}
