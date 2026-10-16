package core

import (
	"bytes"
	"testing"
)

func TestCmdlineHasExactArgUsesFullBase(t *testing.T) {
	vue := psLine(
		"/data/sdk/jdk-17.0.15/bin/java",
		"-classpath", "/data/vue_test/datahub01web/bin/bootstrap.jar:/data/vue_test/datahub01web/bin/tomcat-juli.jar",
		"-Dcatalina.base=/data/vue_test/datahub01web",
		"-Dcatalina.home=/data/vue_test/datahub01web",
		"org.apache.catalina.startup.Bootstrap", "start",
	)
	gray := psLine(
		"/data/sdk/jdk-17.0.15/bin/java",
		"-classpath", "/data/datahubv10-gray/datahub01web/bin/bootstrap.jar",
		"-Dcatalina.base=/data/datahubv10-gray/datahub01web",
		"org.apache.catalina.startup.Bootstrap", "start",
	)
	v9 := psLine(
		"/data/jdk-17.0.15/bin/java",
		"-Dcatalina.base=/home/data/datahubv9/tomcat/datahub01web",
		"org.apache.catalina.startup.Bootstrap", "start",
	)
	// 夹具是 Linux 上的命令行。匹配本身不改分隔符；filepath.Clean 由运行代理的系统处理。
	want := "-Dcatalina.base=/data/vue_test/datahub01web"
	if !cmdlineHasExactArg(vue, want) {
		t.Fatal("vue_test base should match")
	}
	if cmdlineHasExactArg(gray, want) || cmdlineHasExactArg(v9, want) {
		t.Fatal("other datahub01web installs must not match")
	}
	// 只出现在 classpath 里的 base 目录不算这套进程。
	classpathOnly := psLine(
		"java",
		"-classpath", "/data/vue_test/datahub01web/bin/bootstrap.jar",
		"org.apache.catalina.startup.Bootstrap", "start",
	)
	if cmdlineHasExactArg(classpathOnly, want) {
		t.Fatal("classpath prefix is not catalina.base")
	}
	longer := psLine("java", "-Dcatalina.base=/data/vue_test/datahub01web-extra", "start")
	if cmdlineHasExactArg(longer, want) {
		t.Fatal("longer base must not match")
	}
	if cmdlineHasExactArg(vue, "datahub01web") {
		t.Fatal("instance name alone must not match")
	}
}

func psLine(args ...string) []byte {
	return bytes.Join(toBytes(args), []byte{0})
}

func toBytes(args []string) [][]byte {
	out := make([][]byte, len(args))
	for i, arg := range args {
		out[i] = []byte(arg)
	}
	return out
}
