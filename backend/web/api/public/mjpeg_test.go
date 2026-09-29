package public

import (
	"reflect"
	"testing"
)

func TestMJPEGLegacyLanguageUsesCompactEnglishLabels(t *testing.T) {
	if got, want := getLangPack("zh-CN"), getLangPack("en"); !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy language labels do not fit the fixed MJPEG table: got %+v, want %+v", got, want)
	}
}
