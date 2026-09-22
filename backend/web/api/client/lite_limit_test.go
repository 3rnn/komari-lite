package client
import("testing";"net/http/httptest";"strings";"bytes";"compress/gzip")
func TestLiteReportLimit(t *testing.T){for _, zipped:=range []bool{false,true}{body:=strings.Repeat("x",(1<<20)+1);var buf bytes.Buffer;if zipped{z:=gzip.NewWriter(&buf);z.Write([]byte(body));z.Close()}else{buf.WriteString(body)};r:=httptest.NewRequest("POST","/",&buf);if zipped{r.Header.Set("Content-Encoding","gzip")};if _,err:=readMaybeCompressedBody(r);err==nil{t.Fatalf("oversize accepted gzip=%v",zipped)}}}
