package appside

import (
	// 登记各 mode（init → core.RegisterMode）。
	_ "fluxguard/agents/appside/modes/files"
	_ "fluxguard/agents/appside/modes/jar"
	_ "fluxguard/agents/appside/modes/node"
	_ "fluxguard/agents/appside/modes/script"
	_ "fluxguard/agents/appside/modes/static"
	_ "fluxguard/agents/appside/modes/tomcat"
)
