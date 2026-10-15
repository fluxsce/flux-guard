package dao

import "testing"

func TestTomcatHomeOf(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/data/vue_test/datahubServer/webapps/datahubServer.war", "/data/vue_test/datahubServer"},
		{"/opt/tomcat/logs/catalina.out", "/opt/tomcat"},
		{`E:\opt\tomcat\webapps\app.war`, `E:\opt\tomcat`},
		{"/opt/app/order.war", ""},
	}
	for _, c := range cases {
		got := tomcatHomeOf(c.in)
		if got != c.want {
			t.Fatalf("%q -> %q, want %q", c.in, got, c.want)
		}
	}
}
