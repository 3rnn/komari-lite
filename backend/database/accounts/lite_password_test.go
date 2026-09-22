package accounts
import("testing"; "strings")
func TestLitePasswordSalted(t *testing.T){a:=hashPasswd("test-only-password");b:=hashPasswd("test-only-password");if a==b || !strings.HasPrefix(a,"$2a$"){t.Fatal("password must use salted bcrypt")}}
